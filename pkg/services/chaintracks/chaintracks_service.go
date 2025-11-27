package chaintracks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/gormstorage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/ingest"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/internal"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

const (
	liveHeadersChanSize   = 1000
	lastPresentHeightTTL  = 60 * time.Second
	cdnSyncRepeatDuration = 24 * time.Hour
	syncCheckInterval     = 1 * time.Second
)

// Service provides core functionality for the Chaintracks service with logging and configuration support.
type Service struct {
	logger *slog.Logger
	config defs.ChaintracksServiceConfig

	storage Storage

	liveIngestors   []NamedLiveIngestor
	liveHeadersChan chan wdk.ChainBlockHeader

	bulkMgr *bulkManager

	cancelCtx          context.CancelFunc
	makeAvailableOnce  sync.Once
	shiftLiveHeadersWG sync.WaitGroup

	available   bool
	availableMu sync.RWMutex

	cachedPresentHeight *internal.CacheableWithTTL[uint]

	lastSyncCheck time.Time
	lastBulkSync  time.Time

	headerCallbacks *internal.PubSubEvents[wdk.ChainBlockHeader]
	reorgCallbacks  *internal.PubSubEvents[models.ReorgEvent]
}

// NewService creates and returns a new Service instance initialized with the provided logger and configuration.
// Returns an error if the given config is invalid according to its validation rules.
func NewService(logger *slog.Logger, config defs.ChaintracksServiceConfig, overrides ...Initializers) (*Service, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid chaintracks service config: %w", err)
	}

	// NOTE: This creates an in-memory SQLite DB which is not persistent.
	// NOTE: This is acceptable for this case, when it's no big deal to re-sync data on restart.
	// TODO: Add config options to allow persistent storage backends.
	dbConfig := gormstorage.InMemorySQLiteDBConfig()
	storage, err := gormstorage.NewProvider(logger, gormstorage.WithDBConfig(dbConfig))
	if err != nil {
		return nil, fmt.Errorf("failed to create chaintracks storage provider: %w", err)
	}

	initializers := createInitializers(overrides...)
	liveIngestors := createLiveIngestors(logger, config, initializers)
	bulkIngestors := createBulkIngestors(logger, config, initializers)

	srv := &Service{
		logger:          logging.Child(logger, "chaintracks_service"),
		config:          config,
		storage:         storage,
		liveIngestors:   liveIngestors,
		liveHeadersChan: make(chan wdk.ChainBlockHeader, liveHeadersChanSize),
		bulkMgr:         newBulkManager(logger, bulkIngestors, config.Chain),
		headerCallbacks: internal.NewPubSubEvents[wdk.ChainBlockHeader](logger),
		reorgCallbacks:  internal.NewPubSubEvents[models.ReorgEvent](logger),
	}

	srv.cachedPresentHeight = internal.NewCachableWithTTL[uint](lastPresentHeightTTL, srv.fetchLatestPresentHeight)

	return srv, nil
}

// GetChain returns the configured BSV network for the service.
func (s *Service) GetChain() defs.BSVNetwork {
	return s.config.Chain
}

// MakeAvailable initializes and marks the service as available, starting background workers as needed.
func (s *Service) MakeAvailable(parentCtx context.Context) (err error) {
	s.makeAvailableOnce.Do(func() {
		s.logger.Info("Chaintracks service - making available")

		ctx, cancel := context.WithCancel(parentCtx)
		s.cancelCtx = cancel

		if err := s.storage.Migrate(ctx); err != nil {
			err = fmt.Errorf("failed to migrate chaintracks storage: %w", err)
			s.logger.Error("Chaintracks service", slog.String("error", err.Error()))
			return
		}

		for _, ingestor := range s.liveIngestors {
			s.logger.Info("Chaintracks service - starting live ingestor", slog.String("ingestor_name", ingestor.Name))
			ingestor.Ingestor.StartListening(parentCtx, s.liveHeadersChan)
		}

		err = s.shiftLiveHeaders(ctx)
		if err != nil {
			err = fmt.Errorf("error during initial live headers shift: %w", err)
			s.logger.Error("Chaintracks service", slog.String("error", err.Error()))
			return
		}

		s.shiftLiveHeadersWG.Add(1)
		go s.shiftLiveHeadersWorker(ctx)
		s.setAvailable(true)

		s.logger.Info("Chaintracks service - now available")
	})

	return
}

// Available returns true if the service is currently marked as available, false otherwise.
func (s *Service) Available() bool {
	s.availableMu.RLock()
	defer s.availableMu.RUnlock()
	return s.available
}

func (s *Service) setAvailable(value bool) {
	s.availableMu.Lock()
	defer s.availableMu.Unlock()
	s.available = value
}

// Destroy gracefully shuts down the service, cancels background tasks, and waits for all workers to complete.
func (s *Service) Destroy() {
	s.logger.Info("Chaintracks service - destroying")

	if s.cancelCtx != nil {
		s.cancelCtx()
	}
	s.shiftLiveHeadersWG.Wait()

	for _, ingestor := range s.liveIngestors {
		s.logger.Info("Chaintracks service - stopping live ingestor", slog.String("ingestor_name", ingestor.Name))
		ingestor.Ingestor.StopListening()
	}

	s.setAvailable(false)
	s.logger.Info("Chaintracks service - destroyed")
}

// GetPresentHeight returns the present blockchain height using a cached value with automatic TTL refresh if expired.
// It queries the cache and, if invalid, fetches the latest value using the configured setter function for the cache.
// Returns the blockchain height as uint32 and an error if the retrieval fails.
// Context is used for cancellation and timeout during cache population or data fetching.
func (s *Service) GetPresentHeight(ctx context.Context) (uint, error) {
	if presentHeight, err := s.cachedPresentHeight.Get(ctx); err != nil {
		return 0, fmt.Errorf("failed to get cached present height: %w", err)
	} else {
		return presentHeight, nil
	}
}

// GetAvailableHeightRanges retrieves the available height ranges from both live and bulk storage managers.
// Returns an error if querying or validation of ranges fails.
func (s *Service) GetAvailableHeightRanges(ctx context.Context) (ranges models.HeightRanges, err error) {
	ranges.Live, err = s.storage.Query(ctx).FindLiveHeightRange()
	if err != nil {
		err = fmt.Errorf("failed to query live height range from storage: %w", err)
		return
	}

	ranges.Bulk = s.bulkMgr.GetHeightRange()
	return
}

// GetInfo returns information about the service, including chain, heights, storage type, and ingestor names.
// It ensures the service is available before gathering details and may return an error if prerequisites fail.
func (s *Service) GetInfo(ctx context.Context) (*models.InfoResponse, error) {
	if err := s.MakeAvailable(ctx); err != nil {
		return nil, fmt.Errorf("failed to make service available: %w", err)
	}

	availableRanges, err := s.GetAvailableHeightRanges(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get available height ranges: %w", err)
	}

	return &models.InfoResponse{
		Chain:         s.GetChain(),
		HeightBulk:    availableRanges.Bulk.MaxHeight,
		HeightLive:    availableRanges.Live.MaxHeight,
		Storage:       "gorm-sqlite-inmemory", // TODO: make dynamic when other storage backends are supported
		BulkIngestors: slices.Map(s.bulkMgr.bulkIngestors, func(ingestor NamedBulkIngestor) string { return ingestor.Name }),
		LiveIngestors: slices.Map(s.liveIngestors, func(ingestor NamedLiveIngestor) string { return ingestor.Name }),
	}, nil
}

// FindChainTipHeader retrieves the current chain tip block header from storage or returns an error if not found.
func (s *Service) FindChainTipHeader(ctx context.Context) (*models.LiveBlockHeader, error) {
	tipHeader, err := s.storage.Query(ctx).GetActiveTipLiveHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to get active tip live header: %w", err)
	}
	if tipHeader == nil {
		return nil, fmt.Errorf("no active tip live header found: %w", wdk.ErrNotFoundError)
	}
	return tipHeader, nil
}

// FindChainTipHash retrieves the hash of the current chain tip.
// Returns the hash as a string or an error if the chain tip header cannot be found.
func (s *Service) FindChainTipHash(ctx context.Context) (string, error) {
	if tipHeader, err := s.FindChainTipHeader(ctx); err != nil {
		return "", fmt.Errorf("failed to find chain tip header: %w", err)
	} else {
		return tipHeader.Hash, nil
	}
}

// FindHeaderForHeight returns the chain block header for the given height from live storage or bulk storage if available.
// Returns an error if the header is not found or if storage access fails.
// In case of "not-found" the error wraps wdk.ErrNotFoundError for easier identification.
func (s *Service) FindHeaderForHeight(ctx context.Context, height uint) (*models.LiveOrBulkBlockHeader, error) {
	liveHeader, err := s.storage.Query(ctx).GetLiveHeaderByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("failed to get live header by height: %w", err)
	}
	if liveHeader != nil {
		return liveHeader, nil
	}

	header, err := s.bulkMgr.FindHeaderForHeight(height)
	if err != nil {
		return nil, fmt.Errorf("failed to find header for height in bulk storage: %w", err)
	}

	if header == nil {
		return nil, fmt.Errorf("header not found for height %d: %w", height, wdk.ErrNotFoundError)
	}

	return &models.LiveOrBulkBlockHeader{ChainBlockHeader: *header}, nil
}

// BulkFilesInfo returns metadata about all bulk block header files currently managed by the service.
func (s *Service) BulkFilesInfo() *ingest.BulkHeaderFilesInfo {
	return s.bulkMgr.FilesInfo()
}

// BulkFileDataByIndex retrieves the BulkFileData for the file at the specified index.
// Returns an error if the index is out of bounds or the file cannot be accessed.
func (s *Service) BulkFileDataByIndex(index int) (*ingest.BulkFileData, error) {
	return s.bulkMgr.GetFileDataByIndex(index)
}

// SubscribeHeaders subscribes to notifications of new ChainBlockHeader events as they are published by the service.
// It returns a receive-only channel for ChainBlockHeader messages and an unsubscribe function to stop receiving updates.
// Unsubscribing will remove the subscriber and close the associated channel to clean up resources.
func (s *Service) SubscribeHeaders() (readOnlyChan <-chan wdk.ChainBlockHeader, unsubscribe func()) {
	return s.headerCallbacks.Subscribe()
}

// SubscribeReorgs returns a read-only channel for receiving chain reorganization events and an unsubscribe function.
// The channel delivers ReorgEvent values whenever a chain reorg is detected, providing details of the old and new tip.
// Call the returned unsubscribe function to stop receiving events and release resources associated with the subscription.
func (s *Service) SubscribeReorgs() (readOnlyChan <-chan models.ReorgEvent, unsubscribe func()) {
	return s.reorgCallbacks.Subscribe()
}

func (s *Service) getMissingBlockHeader(ctx context.Context, hash string) *wdk.ChainBlockHeader {
	for _, liveIngestor := range s.liveIngestors {
		header, err := liveIngestor.Ingestor.GetHeaderByHash(ctx, hash)
		if err != nil {
			s.logger.Warn("Chaintracks service - error fetching header by hash from ingestor", slog.String("ingestor_name", liveIngestor.Name), slog.String("error", err.Error()))
			continue
		}

		if header != nil {
			s.logger.Debug("Chaintracks service - fetched missing header from ingestor", slog.String("ingestor_name", liveIngestor.Name), slog.String("header_hash", hash))
			return header
		}
	}

	return nil
}

func (s *Service) fetchLatestPresentHeight(ctx context.Context) (uint, error) {
	var maxHeight uint

	for _, ingestor := range s.liveIngestors {
		height, err := ingestor.Ingestor.GetPresentHeight(ctx)
		if err != nil {
			s.logger.Error("Chaintracks service - error fetching present height from ingestor", slog.String("ingestor_name", ingestor.Name), slog.String("error", err.Error()))
			continue
		}

		s.logger.Debug("Chaintracks service - fetched present height from ingestor", slog.String("ingestor_name", ingestor.Name), slog.Any("present_height", height))

		if height > maxHeight {
			maxHeight = height
		}
	}

	if maxHeight > 0 {
		return maxHeight, nil
	}
	return 0, fmt.Errorf("no live ingestors available to fetch present height")
}

func (s *Service) syncBulkStorage(ctx context.Context, presentHeight uint, initialRanges models.HeightRanges) (err error) {
	if s.skipBulkSync(presentHeight, initialRanges) {
		s.logger.Debug("Chaintracks service - skipping bulk synchronization as recent sync already performed", slog.Any("present_height", presentHeight))
		return nil
	}

	defer func() {
		if err == nil {
			s.lastBulkSync = time.Now()
			s.logger.Info("Chaintracks service - bulk synchronization completed successfully", slog.Any("present_height", presentHeight))
		} else {
			s.logger.Error("Chaintracks service - bulk synchronization failed", slog.String("error", err.Error()), slog.Any("present_height", presentHeight))
		}
	}()

	if err := s.bulkMgr.SyncBulkStorage(ctx, presentHeight, initialRanges, s.config.LiveHeightThreshold); err != nil {
		return fmt.Errorf("bulk synchronization failed: %w", err)
	}

	return nil
}

func (s *Service) shiftLiveHeadersWorker(ctx context.Context) {
	defer s.shiftLiveHeadersWG.Done()
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Chaintracks service - shift live headers worker stopping due to context cancellation")
			return
		default:
			err := s.shiftLiveHeaders(ctx)
			if err != nil {
				s.logger.Error("Chaintracks service - error shifting live headers", slog.String("error", err.Error()))
			}

			if err := cancellableSleep(ctx, syncCheckInterval); err != nil {
				s.logger.Info("Chaintracks service - shift live headers worker stopping during sleep due to context cancellation")
				return
			}
		}
	}
}

func (s *Service) shiftLiveHeaders(ctx context.Context) error {
	s.lastSyncCheck = time.Now()

	presentHeight, err := s.GetPresentHeight(ctx)
	if err != nil {
		return fmt.Errorf("failed to get present height during live headers shift: %w", err)
	}

	before, err := s.GetAvailableHeightRanges(ctx)
	if err != nil {
		return fmt.Errorf("failed to get available height ranges during live headers shift: %w", err)
	}

	if err = before.Validate(); err != nil {
		return fmt.Errorf("invalid available 'before' height ranges: %w", err)
	}

	if err := s.syncBulkStorage(ctx, presentHeight, before); err != nil {
		return fmt.Errorf("bulk synchronization failed during live headers shift: %w", err)
	}

	if err := s.fillGapLiveHeaders(ctx, presentHeight, before.Live); err != nil {
		return fmt.Errorf("failed to fill gap live headers during live headers shift: %w", err)
	}

	if err := s.processHeaders(ctx); err != nil {
		return fmt.Errorf("failed to process live headers during live headers shift: %w", err)
	}

	if err := s.migrateLiveToBulk(ctx); err != nil {
		return fmt.Errorf("failed to migrate live headers to bulk during live headers shift: %w", err)
	}
	return nil
}

func (s *Service) skipBulkSync(presentHeight uint, ranges models.HeightRanges) bool {
	if time.Since(s.lastBulkSync) > cdnSyncRepeatDuration {
		return false
	}

	return ranges.Live.NotEmpty() && ranges.Live.MaxHeight >= presentHeight-(s.config.AddLiveRecursionLimit/2)
}

func (s *Service) fillGapLiveHeaders(ctx context.Context, presentHeight uint, liveInitialRange models.HeightRange) error {
	gapHeaders, err := s.bulkMgr.GetGapHeadersAsLive(ctx, presentHeight, liveInitialRange, s.config.AddLiveRecursionLimit)
	if err != nil {
		return fmt.Errorf("failed to get gap headers as live during live headers shift: %w", err)
	}

	if len(gapHeaders) > 0 {
		s.logger.Info("Filling gap between bulk and live headers using bulk ingestors", slog.Any("num_gap_headers", len(gapHeaders)))
	}

	for _, header := range gapHeaders {
		if _, err := s.addLiveHeader(ctx, header); err != nil {
			s.logger.Warn("Chaintracks service - failed to add gap header as live", slog.String("header_hash", header.Hash), slog.String("error", err.Error()))
		}
	}

	return nil
}

func (s *Service) processHeaders(ctx context.Context) error {
	addedHeadersHashes := make(map[string]struct{})
	for {
		select {
		case <-ctx.Done():
			return nil
		case header := <-s.liveHeadersChan:
			if added, err := s.addLiveHeader(ctx, header); err != nil {
				s.logger.Warn("Chaintracks service - failed to add live header", slog.String("header_hash", header.Hash), slog.String("error", err.Error()))
			} else if added {
				addedHeadersHashes[header.Hash] = struct{}{}
			}
		default:
			// No more headers to process

			// Publish chain tip if it was added during this processing cycle
			if activeTip, err := s.storage.Query(ctx).GetActiveTipLiveHeader(); err != nil {
				return fmt.Errorf("failed to get active tip live header after processing headers: %w", err)
			} else if activeTip != nil {
				if _, ok := addedHeadersHashes[activeTip.Hash]; ok {
					s.headerCallbacks.Publish(activeTip.ChainBlockHeader)
				}
			}

			// invalidate cached present height to force refresh on next request
			s.cachedPresentHeight.Invalidate()

			return nil
		}
	}
}

func (s *Service) addLiveHeader(ctx context.Context, header wdk.ChainBlockHeader) (added bool, err error) {
	added, err = s.storeLiveHeader(ctx, header)
	if errors.Is(err, errNoPrev) {
		added, err = s.addLiveHeaderRecursive(ctx, header, must.ConvertToIntFromUnsigned(s.config.AddLiveRecursionLimit))
	}
	if err != nil {
		return added, fmt.Errorf("failed to store header: %w", err)
	}

	return added, nil
}

func (s *Service) addLiveHeaderRecursive(ctx context.Context, header wdk.ChainBlockHeader, depth int) (bool, error) {
	if depth <= 0 {
		return false, fmt.Errorf("recursion limit reached while adding header recursively for header: %s", header.Hash)
	}

	prevHeader := s.getMissingBlockHeader(ctx, header.PreviousHash)
	if prevHeader == nil {
		return false, fmt.Errorf("previous header not found for hash: %s", header.PreviousHash)
	}

	_, err := s.storeLiveHeader(ctx, *prevHeader)
	if errors.Is(err, errNoPrev) {
		_, err = s.addLiveHeaderRecursive(ctx, *prevHeader, depth-1)
	}
	if err != nil {
		return false, fmt.Errorf("failed to store previous header: %w", err)
	}

	added, err := s.storeLiveHeader(ctx, header)
	if err != nil {
		return false, fmt.Errorf("failed to store header after adding previous: %w", err)
	}

	return added, nil
}

var errNoPrev = fmt.Errorf("no previous header found")

func (s *Service) storeLiveHeader(ctx context.Context, header wdk.ChainBlockHeader) (added bool, err error) {
	if header.Height == 0 {
		return false, fmt.Errorf("handling genesis block header is not supported here")
	}

	if err := header.Validate(); err != nil {
		return false, fmt.Errorf("invalid block header: %w", err)
	}

	if IsDirtyHash(header.Hash) {
		return false, fmt.Errorf("cannot add block header with dirty hash: %s", header.Hash)
	}

	lastBulk, lastBulkChainWork, err := s.bulkMgr.LastHeader()
	if err != nil {
		return false, fmt.Errorf("failed to get last bulk header: %w", err)
	}

	if lastBulk == nil {
		return false, fmt.Errorf("no bulk headers available to validate against")
	}

	if header.Height <= lastBulk.Height {
		s.logger.Info("Chaintracks service - skipping storage of live header already present in bulk storage", slog.String("header_hash", header.Hash), slog.Any("header_height", header.Height))
		return false, nil
	}

	q := s.storage.Query(ctx)
	q.Begin()
	defer func() {
		if err != nil {
			rollbackErr := q.Rollback()
			if rollbackErr != nil {
				err = fmt.Errorf("failed to rollback transaction after error: %s; original error: %w", rollbackErr.Error(), err)
			}
		} else {
			err = q.Commit()
			if err != nil {
				err = fmt.Errorf("failed to commit transaction: %w", err)
			}
		}
	}()

	if exists, err := q.LiveHeaderExists(header.Hash); err != nil {
		return false, fmt.Errorf("failed to check existing header: %w", err)
	} else if exists {
		s.logger.Debug("Chaintracks service - header already exists, skipping", slog.String("header_hash", header.Hash))
		return false, nil
	}

	oneBack, err := q.GetLiveHeaderByHash(header.PreviousHash)
	if err != nil {
		return false, fmt.Errorf("failed to get previous header: %w", err)
	}

	if oneBack == nil {
		if count, err := q.CountLiveHeaders(); err != nil {
			return false, fmt.Errorf("failed to count live headers: %w", err)
		} else if count == 0 {
			// No live headers yet, check if this header connects directly to last bulk
			if header.PreviousHash == lastBulk.Hash && header.Height == lastBulk.Height+1 {
				headerChainWork := internal.ChainWorkFromBits(header.Bits)
				chainWork := headerChainWork.AddChainWork(*lastBulkChainWork)

				if err := q.InsertNewLiveHeader(&models.LiveBlockHeader{
					ChainBlockHeader: header,
					PreviousHeaderID: nil,
					ChainWork:        chainWork.To64PadHex(),
					IsChainTip:       true,
					IsActive:         true,
				}); err != nil {
					return false, fmt.Errorf("failed to insert new live header: %w", err)
				}

				return true, nil
			}
		}

		return false, errNoPrev
	}

	if oneBack.Height+1 != header.Height {
		return false, fmt.Errorf("header height mismatch: expected %d, got %d", oneBack.Height+1, header.Height)
	}

	var priorTip *models.LiveBlockHeader
	if oneBack.IsActive && oneBack.IsChainTip {
		priorTip = oneBack
	} else {
		priorTip, err = q.GetActiveTipLiveHeader()
		if err != nil {
			return false, fmt.Errorf("failed to get active tip header: %w", err)
		}

		if priorTip == nil {
			return false, fmt.Errorf("active tip header not found for hash: %s", oneBack.Hash)
		}
	}

	oneBackChainWork, err := internal.ChainWorkFromHex(oneBack.ChainWork)
	if err != nil {
		return false, fmt.Errorf("failed to parse chain work from previous header: %w", err)
	}

	headerChainWork := internal.ChainWorkFromBits(header.Bits)

	chainWork := headerChainWork.AddChainWork(oneBackChainWork)

	priorTipChainWork, err := internal.ChainWorkFromHex(priorTip.ChainWork)
	if err != nil {
		return false, fmt.Errorf("failed to parse chain work from prior tip header: %w", err)
	}

	isActiveTip := chainWork.CmpChainWork(priorTipChainWork) > 0
	if isActiveTip {
		s.logger.Info("Chaintracks service - new active chain tip detected", slog.String("header_hash", header.Hash), slog.Any("header_height", header.Height))

		// find newHeader's first active ancestor
		activeAncestor := oneBack
		for !activeAncestor.IsActive {
			previousHash := activeAncestor.PreviousHash
			activeAncestor, err = q.GetLiveHeaderByHash(previousHash)
			if err != nil {
				return false, fmt.Errorf("failed to get active ancestor header: %w", err)
			}

			if activeAncestor == nil {
				return false, fmt.Errorf("active ancestor header not found for hash: %s", previousHash)
			}
		}

		if activeAncestor.HeaderID != oneBack.HeaderID {
			s.logger.Info("Chaintracks service - chain reorganization detected", slog.String("new_tip_hash", header.Hash), slog.Any("new_tip_height", header.Height), slog.String("active_ancestor_hash", activeAncestor.Hash), slog.Any("active_ancestor_height", activeAncestor.Height))

			// deactivate headers from the current active chain tip up to but excluding our activeAncestor:
			if err := s.setActiveRecursivelyUntilReachAncestor(q, false, priorTip, activeAncestor); err != nil {
				return false, fmt.Errorf("failed to deactivate headers during reorg: %w", err)
			}

			// the first header to activate is one before the one we are about to insert
			// headers are activated until we reach the active ancestor
			if err := s.setActiveRecursivelyUntilReachAncestor(q, true, oneBack, activeAncestor); err != nil {
				return false, fmt.Errorf("failed to activate headers during reorg: %w", err)
			}

			s.reorgCallbacks.Publish(models.ReorgEvent{
				NewTip: header,
				OldTip: priorTip.ChainBlockHeader,
			})
		}
	}

	if oneBack.IsChainTip {
		if err := q.SetChainTipByID(oneBack.HeaderID, false); err != nil {
			return false, fmt.Errorf("failed to unset prior chain tip: %w", err)
		}
	}

	if err := q.InsertNewLiveHeader(&models.LiveBlockHeader{
		ChainBlockHeader: header,
		PreviousHeaderID: to.Ptr(oneBack.HeaderID),
		ChainWork:        chainWork.To64PadHex(),
		IsChainTip:       isActiveTip,
		IsActive:         isActiveTip,
	}); err != nil {
		return false, fmt.Errorf("failed to insert new live header: %w", err)
	}

	return true, nil
}

func (s *Service) migrateLiveToBulk(ctx context.Context) (err error) {
	q := s.storage.Query(ctx)
	q.Begin()
	defer func() {
		if err != nil {
			rollbackErr := q.Rollback()
			if rollbackErr != nil {
				err = fmt.Errorf("failed to rollback transaction after error: %s; original error: %w", rollbackErr.Error(), err)
			}
		} else {
			err = q.Commit()
			if err != nil {
				err = fmt.Errorf("failed to commit transaction: %w", err)
			}
		}
	}()

	liveRange, err := s.storage.Query(ctx).FindLiveHeightRange()
	if err != nil {
		return fmt.Errorf("failed to find live height range during migration: %w", err)
	}

	if liveRange.IsEmpty() {
		s.logger.Debug("No live headers to migrate to bulk storage")
		return nil
	}

	count := liveRange.MaxHeight - liveRange.MinHeight + 1
	if count <= s.config.LiveHeightThreshold {
		s.logger.Debug("Not enough live headers to migrate to bulk storage", slog.Any("live_header_count", count))
		return nil
	}
	thresholdHeight := liveRange.MaxHeight - s.config.LiveHeightThreshold

	headers, err := q.FindHeadersForHeightLessThanOrEqualSorted(thresholdHeight, must.ConvertToIntFromUnsigned(s.config.LiveHeightThreshold))
	if err != nil {
		return fmt.Errorf("failed to find live headers for migration: %w", err)
	}

	if len(headers) == 0 {
		s.logger.Debug("No live headers found for migration to bulk storage")
		return nil
	}

	err = s.bulkMgr.MigrateFromLiveHeaders(ctx, headers)
	if err != nil {
		return fmt.Errorf("failed to migrate live headers to bulk storage: %w", err)
	}

	err = q.DeleteLiveHeadersByIDs(slices.Map(headers, func(h *models.LiveBlockHeader) uint { return h.HeaderID }))
	if err != nil {
		return fmt.Errorf("failed to delete migrated live headers: %w", err)
	}

	s.logger.Info("Migrated live headers to bulk storage", slog.Any("migrated_header_count", len(headers)), slog.Any("up_to_height", thresholdHeight))

	return nil
}

func (s *Service) setActiveRecursivelyUntilReachAncestor(q models.StorageQueries, isActive bool, first, ancestor *models.LiveBlockHeader) error {
	var err error
	current := first
	for current.HeaderID != ancestor.HeaderID {
		if err := q.SetActiveByID(current.HeaderID, isActive); err != nil {
			return fmt.Errorf("failed to set active=%t during reorg: %w", isActive, err)
		}

		previousHash := current.PreviousHash
		current, err = q.GetLiveHeaderByHash(previousHash)
		if err != nil {
			return fmt.Errorf("failed to get previous header during reorg: %w", err)
		}

		if current == nil {
			return fmt.Errorf("previous header not found during reorg for hash: %s", previousHash)
		}
	}

	return nil
}

func cancellableSleep(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return fmt.Errorf("sleep canceled: %w", ctx.Err())
	}
}
