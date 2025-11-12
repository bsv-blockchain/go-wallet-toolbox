package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/golang/groupcache/lru"
)

const cachedEntries = 500

// LiveIngestorWocPoll provides functionality for polling block header data from an external source, such as WhatsOnChain.
type LiveIngestorWocPoll struct {
	logger *slog.Logger
	config defs.WOCPollIngestorConfig
	resty  *resty.Client

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
func NewLiveIngestorWocPoll(logger *slog.Logger, config defs.WOCPollIngestorConfig, opts ...func(options *ClientOptions)) *LiveIngestorWocPoll {
	logger = logging.Child(logger, "live_ingestor_woc_poll")

	options := to.OptionsWithDefault(DefaultClientOptions(), opts...)

	url, err := whatsonchain.MakeBaseURL(config.Chain)
	if err != nil {
		panic(fmt.Sprintf("failed to build base URL for WhatsOnChain: %s", err.Error()))
	}

	restyClient := options.RestyClientFactory.New()
	headers := httpx.NewHeaders().
		AcceptJSON().
		ContentTypeJSON().
		UserAgent().Value("go-wallet-toolbox").
		Authorization().IfNotEmpty(config.APIKey)

	restyClient = restyClient.
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(logger)).
		SetDebug(logging.IsDebug(logger)).
		SetBaseURL(url)

	return &LiveIngestorWocPoll{
		logger:     logger,
		config:     config,
		resty:      restyClient,
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
	path := fmt.Sprintf("/block/%s/header", hash)

	var hdrResp WOCBlockHeaderDTO
	res, err := ing.resty.R().
		SetContext(ctx).
		SetResult(&hdrResp).
		Get(path)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch block header: %w", err)
	}
	if res.StatusCode() != http.StatusOK {
		if res.StatusCode() == http.StatusNotFound {
			return nil, fmt.Errorf("block header not found for hash %s: %w", hash, wdk.ErrNotFoundError)
		}
		return nil, fmt.Errorf("unexpected status code %d fetching block header", res.StatusCode())
	}

	if hdrResp.PrevBlock == "" {
		hdrResp.PrevBlock = genesisAsPrevBlockHash
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
	path := "/chain/info"

	var infoResp blockOnlyChainInfoDTO
	res, err := ing.resty.R().
		SetContext(ctx).
		SetResult(&infoResp).
		Get(path)

	if err != nil {
		return 0, fmt.Errorf("failed to fetch chain info: %w", err)
	}
	if res.StatusCode() != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code %d fetching chain info", res.StatusCode())
	}

	return infoResp.Blocks, nil
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
		return must.ConvertToIntFromUnsigned(a.Height) - must.ConvertToIntFromUnsigned(b.Height)
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
	path := "/block/headers"

	var headersResponse WOCBlockHeadersDTO
	res, err := ing.resty.R().
		SetContext(ctx).
		SetResult(&headersResponse).
		Get(path)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch block headers: %w", err)
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d fetching block headers", res.StatusCode())
	}

	wdkHeaders, err := headersResponse.ToWDK()
	if err != nil {
		return nil, fmt.Errorf("failed to convert block headers DTO to WDK format: %w", err)
	}

	return wdkHeaders, nil
}
