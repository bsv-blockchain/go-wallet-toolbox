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
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/internal"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

const (
	liveHeadersChanSize    = 1000
	lastPresentHeightTTL   = 60 * time.Second
	cdnSyncRepeatDuration  = 24 * time.Hour
	syncCheckInterval      = 1 * time.Second
	addLiveRecursionLimit  = 11
	halfLiveRecursionLimit = addLiveRecursionLimit / 2
)

// Service provides core functionality for the Chaintracks service with logging and configuration support.
type Service struct {
	logger *slog.Logger
	config defs.ChaintracksServiceConfig

	storage Storage

	liveIngestors   []NamedLiveIngestor
	liveHeadersChan chan wdk.ChainBlockHeader

	bulkIngestors []NamedBulkIngestor

	cancelCtx          context.CancelFunc
	makeAvailableOnce  sync.Once
	shiftLiveHeadersWG sync.WaitGroup

	available   bool
	availableMu sync.RWMutex

	cachedPresentHeight *internal.CacheableWithTTL[uint]

	lastSyncCheck time.Time
	lastBulkSync  time.Time
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
		bulkIngestors:   bulkIngestors,
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
	// TODO: change initialRanges to proper type when the PR is merged

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

	for _, ingestor := range s.bulkIngestors {
		_, err := ingestor.Ingestor.Synchronize(ctx, presentHeight, initialRanges)
		if err != nil {
			s.logger.Error("Chaintracks service - error during bulk synchronization", slog.String("ingestor_name", ingestor.Name), slog.String("error", err.Error()))
			return fmt.Errorf("bulk synchronization failed for ingestor %s: %w", ingestor.Name, err)
		}

		// TODO: Implement DONE check and break if done
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

	// TODO: get "before" variable with bulk and live height ranges
	// before := s.storage.GetAvailableHeightRanges()
	before := models.HeightRanges{}

	if err := s.syncBulkStorage(ctx, presentHeight, before); err != nil {
		return fmt.Errorf("bulk synchronization failed during live headers shift: %w", err)
	}

	if err := s.processHeaders(ctx); err != nil {
		return fmt.Errorf("failed to process live headers during live headers shift: %w", err)
	}
	return nil
}

func (s *Service) skipBulkSync(presentHeight uint, ranges models.HeightRanges) bool {
	if time.Since(s.lastBulkSync) > cdnSyncRepeatDuration {
		return false
	}

	return ranges.Live.NotEmpty() && ranges.Live.MaxHeight >= presentHeight-halfLiveRecursionLimit
}

func (s *Service) processHeaders(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case header := <-s.liveHeadersChan:
			if err := s.addLiveHeader(ctx, header); err != nil {
				s.logger.Warn("Chaintracks service - failed to add live header", slog.String("header_hash", header.Hash), slog.String("error", err.Error()))
			}
		default:
			// No more headers to process
			return nil
		}
	}
}

func (s *Service) addLiveHeader(ctx context.Context, header wdk.ChainBlockHeader) error {
	err := s.storeLiveHeader(ctx, header)
	if errors.Is(err, errNoPrev) {
		if err := s.addLiveHeaderRecursive(ctx, header, addLiveRecursionLimit); err != nil {
			return fmt.Errorf("failed to add header recursively: %w", err)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to store header: %w", err)
	}

	return nil
}

func (s *Service) addLiveHeaderRecursive(ctx context.Context, header wdk.ChainBlockHeader, depth int) error {
	if depth <= 0 {
		return fmt.Errorf("recursion limit reached while adding header recursively for header: %s", header.Hash)
	}

	prevHeader := s.getMissingBlockHeader(ctx, header.PreviousHash)
	if prevHeader == nil {
		return fmt.Errorf("previous header not found for hash: %s", header.PreviousHash)
	}

	err := s.storeLiveHeader(ctx, *prevHeader)
	if errors.Is(err, errNoPrev) {
		if err := s.addLiveHeaderRecursive(ctx, *prevHeader, depth-1); err != nil {
			return fmt.Errorf("failed to add previous header recursively: %w", err)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to store previous header: %w", err)
	}

	if err := s.storeLiveHeader(ctx, header); err != nil {
		return fmt.Errorf("failed to store header after adding previous: %w", err)
	}

	return nil
}

var errNoPrev = fmt.Errorf("no previous header found")

func (s *Service) storeLiveHeader(ctx context.Context, header wdk.ChainBlockHeader) (err error) {
	// TODO: implement header.Validate() method and uncomment validation check
	//if err := header.Validate(); err != nil {
	//	return fmt.Errorf("invalid block header: %w", err)
	//}

	if IsDirtyHash(header.Hash) {
		return fmt.Errorf("cannot add block header with dirty hash: %s", header.Hash)
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
		return fmt.Errorf("failed to check existing header: %w", err)
	} else if exists {
		s.logger.Debug("Chaintracks service - header already exists, skipping", slog.String("header_hash", header.Hash))
		return nil
	}

	oneBack, err := q.GetLiveHeaderByHash(header.PreviousHash)
	if err != nil {
		return fmt.Errorf("failed to get previous header: %w", err)
	}

	if oneBack == nil {
		s.logger.Debug("Chaintracks service - previous header not found, cannot add header yet", slog.String("header_hash", header.Hash), slog.String("previous_hash", header.PreviousHash))

		if count, err := q.CountLiveHeaders(); err != nil {
			return fmt.Errorf("failed to count live headers: %w", err)
		} else if count == 0 {
			s.logger.Info("Chaintracks service - no live headers present, inserting genesis header", slog.String("header_hash", header.Hash))

			// TODO check if this first-live-block-header matches the last bulk header
			// TODO: Important: Chainwork from bits should be added to the last ChainWork from the last bulk file
			headerChainWork := internal.ChainWorkFromBits(header.Bits)

			if err := q.InsertNewLiveHeader(&models.LiveBlockHeader{
				ChainBlockHeader: header,
				PreviousHeaderID: nil,
				ChainWork:        headerChainWork.To64PadHex(),
				IsActive:         true,
				IsChainTip:       true,
			}); err != nil {
				return fmt.Errorf("failed to insert genesis live header: %w", err)
			}

			return nil
		}

		return errNoPrev
	}

	if oneBack.Height+1 != header.Height {
		return fmt.Errorf("header height mismatch: expected %d, got %d", oneBack.Height+1, header.Height)
	}

	var priorTip *models.LiveBlockHeader
	if oneBack.IsActive && oneBack.IsChainTip {
		priorTip = oneBack
	} else {
		priorTip, err = q.GetActiveTipLiveHeader()
		if err != nil {
			return fmt.Errorf("failed to get active tip header: %w", err)
		}

		if priorTip == nil {
			return fmt.Errorf("active tip header not found for hash: %s", oneBack.Hash)
		}
	}

	oneBackChainWork, err := internal.ChainWorkFromHex(oneBack.ChainWork)
	if err != nil {
		return fmt.Errorf("failed to parse chain work from previous header: %w", err)
	}

	headerChainWork := internal.ChainWorkFromBits(header.Bits)

	chainWork := headerChainWork.AddChainWork(oneBackChainWork)
	_ = chainWork

	priorTipChainWork, err := internal.ChainWorkFromHex(priorTip.ChainWork)
	if err != nil {
		return fmt.Errorf("failed to parse chain work from prior tip header: %w", err)
	}

	isActiveTip := chainWork.CmpChainWork(priorTipChainWork) > 0
	//if isActiveTip {
	//	// TODO: handle reorgs if needed
	//}

	if oneBack.IsChainTip {
		if err := q.SetChainTipByID(oneBack.HeaderID, false); err != nil {
			return fmt.Errorf("failed to unset prior chain tip: %w", err)
		}
	}

	if err := q.InsertNewLiveHeader(&models.LiveBlockHeader{
		ChainBlockHeader: header,
		PreviousHeaderID: to.Ptr(oneBack.HeaderID),
		ChainWork:        chainWork.To64PadHex(),
		IsChainTip:       isActiveTip,
		IsActive:         isActiveTip,
	}); err != nil {
		return fmt.Errorf("failed to insert new live header: %w", err)
	}

	// TODO: Prune live block headers

	// TODO: trigger callbacks when implemented

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
