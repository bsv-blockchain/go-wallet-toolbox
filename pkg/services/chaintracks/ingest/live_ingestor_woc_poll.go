package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/golang/groupcache/lru"
)

const cachedEntries = 500

// LiveIngestorWocPoll provides functionality for polling block header data from an external source, such as WhatsOnChain.
type LiveIngestorWocPoll struct {
	logger    *slog.Logger
	wocClient *wocClient

	ctx       context.Context
	cancelCtx context.CancelFunc

	syncPeriod     time.Duration
	waitForStop    sync.WaitGroup
	lifecycleMutex sync.Mutex
	stopped        bool

	cached *lru.Cache
}

// NewLiveIngestorWocPoll creates a new LiveIngestorWocPoll using the provided logger, config, and optional client options.
// It initializes a Resty HTTP client configured with default headers, user agent, and API key authorization if set.
// Panics if the WhatsOnChain base URL cannot be built for the specified chain network in the config.
// Returns a pointer to the initialized LiveIngestorWocPoll struct, ready for external data polling operations.
func NewLiveIngestorWocPoll(logger *slog.Logger, chain defs.BSVNetwork, opts ...func(*IngestorWocPollOptions)) *LiveIngestorWocPoll {
	logger = logging.Child(logger, "live_ingestor_woc_poll")

	options := to.OptionsWithDefault(DefaultIngestorWocPollOptions(), opts...)

	return &LiveIngestorWocPoll{
		logger:     logger,
		wocClient:  newWocClient(logger, chain, options.APIKey, options.RestyClientFactory.New()),
		syncPeriod: options.SyncPeriod,
		cached:     lru.New(cachedEntries),
	}
}

// GetHeaderByHash retrieves a Bitcoin block header from an external data source using its hash.
// Returns a ChainBlockHeader and error if the header could not be fetched or parsed.
// If the block is not found, returns wdk.ErrNotFoundError as wrapped error.
// The hash parameter must be a valid block hash as a hex string.
// PreviousHash is set to a predefined value if the block is the genesis block.
func (ing *LiveIngestorWocPoll) GetHeaderByHash(ctx context.Context, hash string) (*wdk.ChainBlockHeader, error) {
	hdrResp, err := ing.wocClient.GetHeaderByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block header by hash %s: %w", hash, err)
	}

	wdkBlockHeader, err := hdrResp.ToWDK()
	if err != nil {
		return nil, fmt.Errorf("failed to convert block header DTO to WDK format: %w", err)
	}

	return wdkBlockHeader, nil
}

// GetPresentHeight retrieves the current blockchain height from the external data source.
// Returns the number of blocks in the chain or an error if the info cannot be fetched or parsed.
func (ing *LiveIngestorWocPoll) GetPresentHeight(ctx context.Context) (uint, error) {
	blocks, err := ing.wocClient.GetPresentHeight(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch present height: %w", err)
	}

	return blocks, nil
}

// StartListening begins polling for new block headers and sends them to respChan until the parent context is canceled.
// This method runs polling in a separate goroutine, checking for context cancellation or periodic sync timeout.
// Each cycle fetches the latest block headers and processes them, forwarding results through respChan.
func (ing *LiveIngestorWocPoll) StartListening(parentCtx context.Context, respChan chan wdk.ChainBlockHeader) {
	ing.lifecycleMutex.Lock()
	defer ing.lifecycleMutex.Unlock()

	if ing.stopped {
		ing.logger.Warn("LiveIngestorWocPoll cannot start listening because it has been stopped")
		return
	}

	if ing.cancelCtx != nil {
		ing.logger.Warn("LiveIngestorWocPoll is already listening")
		return
	}

	ing.logger.Info("LiveIngestorWocPoll started listening")
	ing.ctx, ing.cancelCtx = context.WithCancel(parentCtx)
	ticker := time.NewTicker(ing.syncPeriod)

	ing.waitForStop.Add(1)
	go func() {
		defer ing.waitForStop.Done()
		defer ticker.Stop()

		ing.processNewHeaders(respChan)

		for {
			select {
			case <-ing.ctx.Done():
				ing.logger.Info("LiveIngestorWocPoll stopping listening due to context cancellation")
				return
			case <-ticker.C:
				ing.processNewHeaders(respChan)
			}
		}
	}()
}

func (ing *LiveIngestorWocPoll) processNewHeaders(respChan chan wdk.ChainBlockHeader) {
	headers, err := ing.getLastHeaders(ing.ctx)
	if err != nil {
		ing.logger.Error("failed to get last 10 headers", slog.String("error", err.Error()))
		return
	}

	// oldest first order
	slices.SortFunc(headers, func(a, b *wdk.ChainBlockHeader) int {
		if a.Height < b.Height {
			return -1
		} else if a.Height > b.Height {
			return 1
		} else {
			return 0
		}
	})

	for _, hdr := range headers {
		if _, found := ing.cached.Get(hdr.Hash); found {
			continue
		}

		select {
		case respChan <- *hdr:
			ing.cached.Add(hdr.Hash, struct{}{})
		case <-ing.ctx.Done():
			ing.logger.Info("LiveIngestorWocPoll stopping processing new headers due to context cancellation")
			return
		}
	}
}

// StopListening signals the polling goroutine to stop and waits for it to exit before returning.
func (ing *LiveIngestorWocPoll) StopListening() {
	ing.lifecycleMutex.Lock()
	if ing.cancelCtx != nil {
		ing.cancelCtx()
		ing.logger.Info("LiveIngestorWocPoll stopped listening")
	}
	ing.stopped = true
	ing.lifecycleMutex.Unlock()

	ing.waitForStop.Wait()
}

// getLastHeaders normally fetches the last 10 block headers from the external data source.
func (ing *LiveIngestorWocPoll) getLastHeaders(ctx context.Context) ([]*wdk.ChainBlockHeader, error) {
	headersResponse, err := ing.wocClient.GetLastHeaders(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch last block headers: %w", err)
	}

	wdkHeaders, err := headersResponse.ToWDK()
	if err != nil {
		return nil, fmt.Errorf("failed to convert block headers DTO to WDK format: %w", err)
	}

	return wdkHeaders, nil
}
