// Package whatsonchain adapts the official WhatsOnChain SDK
// (github.com/mrz1836/go-whatsonchain) to the wallet-toolbox service contracts.
//
// It maps the SDK's client methods and DTOs onto the wdk result types, preserves
// the semantics the rest of the toolbox relies on (broadcast result
// classification, in-memory Merkle-root cache, cached BSV exchange rate), and
// applies client-side rate limiting on top of the SDK.
package whatsonchain

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
	wocsdk "github.com/mrz1836/go-whatsonchain"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/time/rate"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// ServiceName is the registered name of the WhatsOnChain service.
const ServiceName = defs.WhatsOnChainServiceName

// userAgent identifies this client to the WhatsOnChain API.
const userAgent = "go-wallet-toolbox"

// SDKClient is the subset of the WhatsOnChain SDK client
// (github.com/mrz1836/go-whatsonchain) used by this adapter. The SDK's
// ClientInterface satisfies it, and tests can supply a mock.
type SDKClient interface {
	BroadcastTx(ctx context.Context, txHex string) (string, error)
	BulkTransactionStatus(ctx context.Context, hashes *wocsdk.TxHashes) (wocsdk.TxStatusList, error)
	GetRawTransactionData(ctx context.Context, hash string) (string, error)
	GetMerkleProofTSC(ctx context.Context, hash string) (wocsdk.MerkleTSCResults, error)
	GetHeaderByHash(ctx context.Context, hash string) (*wocsdk.BlockInfo, error)
	GetHeaders(ctx context.Context) ([]*wocsdk.BlockInfo, error)
	GetBlockByHeight(ctx context.Context, height int64) (*wocsdk.BlockInfo, error)
	GetChainInfo(ctx context.Context) (*wocsdk.ChainInfo, error)
	GetExchangeRate(ctx context.Context) (*wocsdk.ExchangeRate, error)
	GetScriptUnspentTransactions(ctx context.Context, scriptHash string) (wocsdk.ScriptList, error)
	GetScriptConfirmedHistory(ctx context.Context, scriptHash string) (wocsdk.ScriptList, error)
	GetScriptUnconfirmedHistory(ctx context.Context, scriptHash string) (wocsdk.ScriptList, error)
	SetRateLimit(rateLimit int)
}

// WhatsOnChain adapts the WhatsOnChain SDK to the toolbox service contracts.
type WhatsOnChain struct {
	client  SDKClient
	logger  *slog.Logger
	limiter *rate.Limiter

	bsvExchangeRate   defs.BSVExchangeRate
	bsvUpdateInterval time.Duration
	rootCache         map[uint32]*chainhash.Hash
	cacheMu           sync.RWMutex
}

// Option customizes construction of the WhatsOnChain service.
type Option func(*builderOptions)

type builderOptions struct {
	httpClient wocsdk.HTTPInterface
}

// WithHTTPClient injects a custom HTTP client into the underlying SDK client.
// It is primarily used by tests to route SDK requests through a mock transport.
func WithHTTPClient(httpClient wocsdk.HTTPInterface) Option {
	return func(o *builderOptions) {
		o.httpClient = httpClient
	}
}

// New creates a WhatsOnChain service backed by a real SDK client for the given network.
func New(logger *slog.Logger, network defs.BSVNetwork, config defs.WhatsOnChain, opts ...Option) *WhatsOnChain {
	logger = logging.Child(logger, "WoC").With(slog.String("network", string(network)))

	if err := network.Validate(); err != nil {
		panic(fmt.Sprintf("invalid BSV network configuration: %s", err.Error()))
	}

	builder := &builderOptions{}
	for _, opt := range opts {
		opt(builder)
	}

	clientOpts := []wocsdk.ClientOption{
		wocsdk.WithNetwork(mapNetwork(network)),
		wocsdk.WithAPIKey(config.APIKey),
		wocsdk.WithUserAgent(userAgent),
		wocsdk.WithRateLimit(rpsToInt(config.RequestsPerSecond)),
		// Disable the SDK's internal retry so every HTTP request maps to exactly
		// one client-side rate-limiter token (see wait). SDK retries would issue
		// additional attempts that bypass the limiter and could breach the WoC
		// rate cap; transient failures are instead handled by the services layer,
		// which fails over to the next provider.
		wocsdk.WithRequestRetryCount(0),
	}
	if builder.httpClient != nil {
		clientOpts = append(clientOpts, wocsdk.WithHTTPClient(builder.httpClient))
	}

	client, err := wocsdk.NewClient(context.Background(), clientOpts...)
	if err != nil {
		panic(fmt.Sprintf("failed to create WhatsOnChain client: %s", err.Error()))
	}

	return newWhatsOnChain(client, logger, config)
}

// NewWithClient creates a WhatsOnChain service backed by the supplied SDK client.
// It is used by tests to inject a mock client or an SDK client wired to a mock
// HTTP transport.
func NewWithClient(client SDKClient, logger *slog.Logger, config defs.WhatsOnChain) *WhatsOnChain {
	logger = logging.Child(logger, "WoC")
	return newWhatsOnChain(client, logger, config)
}

func newWhatsOnChain(client SDKClient, logger *slog.Logger, config defs.WhatsOnChain) *WhatsOnChain {
	return &WhatsOnChain{
		client:            client,
		logger:            logger,
		limiter:           newRequestLimiter(config.RequestsPerSecond),
		bsvExchangeRate:   config.BSVExchangeRate,
		bsvUpdateInterval: to.If(config.BSVUpdateInterval != nil, func() time.Duration { return *config.BSVUpdateInterval }).ElseThen(defs.DefaultBSVExchangeUpdateInterval),
		rootCache:         make(map[uint32]*chainhash.Hash),
	}
}

// mapNetwork maps a toolbox BSV network to the SDK network. WhatsOnChain only
// serves main/test/stn, so every non-mainnet network maps to the test network
// (tstn does not use WhatsOnChain in production).
func mapNetwork(network defs.BSVNetwork) wocsdk.NetworkType {
	if network == defs.NetworkMainnet {
		return wocsdk.NetworkMain
	}
	return wocsdk.NetworkTest
}

// rpsToInt converts a requests-per-second config value into the SDK's integer
// rate limit, defaulting to the WoC no-API-key limit when unset.
func rpsToInt(requestsPerSecond float64) int {
	if requestsPerSecond <= 0 {
		return defs.DefaultWhatsOnChainRequestsPerSecond
	}
	return int(requestsPerSecond)
}

// SetRequestsPerSecond reconfigures the client-side rate limiter.
// Not safe for concurrent use with in-flight requests - call right after New.
func (woc *WhatsOnChain) SetRequestsPerSecond(requestsPerSecond float64) {
	woc.limiter = newRequestLimiter(requestsPerSecond)
	woc.client.SetRateLimit(rpsToInt(requestsPerSecond))
}

// newRequestLimiter builds the client-side rate limiter for WhatsOnChain requests.
// Exceeding the WoC limit yields 429 responses, which under load turn into broadcast
// failures, so all requests are throttled to the configured requests-per-second
// (defaulting to the limit WoC applies to requests without an API key).
func newRequestLimiter(requestsPerSecond float64) *rate.Limiter {
	if requestsPerSecond <= 0 {
		requestsPerSecond = defs.DefaultWhatsOnChainRequestsPerSecond
	}

	burst := int(requestsPerSecond)
	if burst < 1 {
		burst = 1
	}

	return rate.NewLimiter(rate.Limit(requestsPerSecond), burst)
}

// wait blocks until the client-side rate limiter permits another request.
func (woc *WhatsOnChain) wait(ctx context.Context) error {
	if err := woc.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("waiting for WoC rate limiter: %w", err)
	}
	return nil
}

// RawTx fetches the raw transaction bytes for the given txID. A not-found
// transaction yields (nil, nil).
func (woc *WhatsOnChain) RawTx(ctx context.Context, txID string) (_ *wdk.RawTxResult, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-RawTx", attribute.String("service", "whatsonchain"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if err = woc.wait(ctx); err != nil {
		return nil, err
	}

	txHex, err := woc.client.GetRawTransactionData(ctx, txID)
	if err != nil {
		if errors.Is(err, wocsdk.ErrTransactionNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch raw tx hex: %w", err)
	}
	// WhatsOnChain returns HTTP 404 with an empty body for an unknown tx, which
	// the SDK surfaces as an empty string - treat that as "not found".
	if txHex == "" {
		return nil, nil
	}

	txHexDecoded, err := hex.DecodeString(txHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode raw transaction hex: %w", err)
	}

	txIDFromRawTx := txutils.TransactionIDFromRawTx(txHexDecoded)
	if txID != txIDFromRawTx {
		return nil, fmt.Errorf("computed txid %s doesn't match requested value %s", txIDFromRawTx, txID)
	}

	return &wdk.RawTxResult{
		Name:  ServiceName,
		TxID:  txID,
		RawTx: txHexDecoded,
	}, nil
}

// UpdateBsvExchangeRate returns the USD exchange rate, refreshing it from WoC when
// the cached value is stale.
func (woc *WhatsOnChain) UpdateBsvExchangeRate(ctx context.Context) (_ float64, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-UpdateBsvExchangeRate", attribute.String("service", "whatsonchain"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	nextUpdate := woc.bsvExchangeRate.Timestamp.Add(woc.bsvUpdateInterval)
	if nextUpdate.After(time.Now()) {
		return woc.bsvExchangeRate.Rate, nil
	}

	if err = woc.wait(ctx); err != nil {
		return 0, err
	}

	rate, err := woc.client.GetExchangeRate(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch exchange rate: %w", err)
	}
	if rate.Currency != string(defs.USD) {
		return 0, fmt.Errorf("unsupported currency returned from Whats On Chain")
	}

	return rate.Rate, nil
}

// MerklePath retrieves the merkle path for a transaction using WoC TSC proof.
func (woc *WhatsOnChain) MerklePath(ctx context.Context, txID string) (_ *wdk.MerklePathResult, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-MerklePath", attribute.String("service", "whatsonchain"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if err = woc.wait(ctx); err != nil {
		return nil, err
	}

	proofs, err := woc.client.GetMerkleProofTSC(ctx, txID)
	if err != nil {
		if errors.Is(err, wocsdk.ErrTransactionNotFound) {
			return nil, fmt.Errorf("tx %s has no merkle path yet: %w", txID, wdk.ErrNotFoundError)
		}
		return nil, fmt.Errorf("failed to get TSC proof: %w", err)
	}
	if len(proofs) == 0 || proofs[0] == nil {
		return nil, fmt.Errorf("tx %s has no merkle path yet: %w", txID, wdk.ErrNotFoundError)
	}
	proof := proofs[0]

	header, err := woc.fetchMerkleHeader(ctx, proof.Target)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block header: %w", err)
	}

	merklePath, err := txutils.ConvertTscProofToMerklePath(txID, proof.Index, proof.Nodes, header.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to convert proof for tx %s to merkle path: %w", txID, err)
	}

	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	if err != nil {
		return nil, fmt.Errorf("failed to compute merkle root: %w", err)
	}
	if merkleRoot != header.MerkleRoot {
		return nil, fmt.Errorf("computed merkle root %q does not match block header %q", merkleRoot, header.MerkleRoot)
	}

	return &wdk.MerklePathResult{
		Name:        ServiceName,
		MerklePath:  merklePath,
		BlockHeader: header,
		Notes:       history.NewBuilder().GetMerklePathSuccess(ServiceName).Note().AsList(),
	}, nil
}

// fetchMerkleHeader fetches the block header for a merkle proof target hash.
func (woc *WhatsOnChain) fetchMerkleHeader(ctx context.Context, blockHash string) (*wdk.MerklePathBlockHeader, error) {
	if err := woc.wait(ctx); err != nil {
		return nil, err
	}

	blockInfo, err := woc.client.GetHeaderByHash(ctx, blockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block header: %w", err)
	}

	height, err := to.UInt32(blockInfo.Height)
	if err != nil {
		return nil, fmt.Errorf("invalid block height %d: %w", blockInfo.Height, err)
	}

	return &wdk.MerklePathBlockHeader{
		Height:     height,
		Hash:       blockHash,
		MerkleRoot: blockInfo.MerkleRoot,
	}, nil
}

// FindChainTipHeader returns the current chain tip header.
func (woc *WhatsOnChain) FindChainTipHeader(ctx context.Context) (_ *wdk.ChainBlockHeader, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-FindChainTipHeader", attribute.String("service", "whatsonchain"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if err = woc.wait(ctx); err != nil {
		return nil, err
	}

	headers, err := woc.client.GetHeaders(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while fetching block headers from WhatsOnChain: %w", err)
	}
	if len(headers) == 0 || headers[0] == nil {
		return nil, fmt.Errorf("no block headers returned from WhatsOnChain; at least one expected")
	}

	header, err := blockInfoToChainBlockHeader(headers[0])
	if err != nil {
		return nil, fmt.Errorf("error while converting the response from WhatsOnChain to the *wdk.ChainBlockHeader: %w", err)
	}

	return header, nil
}

// CurrentHeight returns the current best-chain height.
func (woc *WhatsOnChain) CurrentHeight(ctx context.Context) (_ uint32, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-CurrentHeight", attribute.String("service", "whatsonchain"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if err = woc.wait(ctx); err != nil {
		return 0, err
	}

	info, err := woc.client.GetChainInfo(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch chain info: %w", err)
	}
	if info.Blocks == 0 {
		return 0, fmt.Errorf("WhatsOnChain returned height 0")
	}

	height, err := to.UInt32(info.Blocks)
	if err != nil {
		return 0, fmt.Errorf("invalid height %d in WhatsOnChain response: %w", info.Blocks, err)
	}
	return height, nil
}

// ChainHeaderByHeight returns the chain block header at the given height.
func (woc *WhatsOnChain) ChainHeaderByHeight(ctx context.Context, height uint32) (_ *wdk.ChainBlockHeader, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-ChainHeaderByHeight", attribute.String("service", "whatsonchain"), attribute.Int64("height", int64(height)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if err = woc.wait(ctx); err != nil {
		return nil, err
	}

	blockInfo, err := woc.client.GetBlockByHeight(ctx, int64(height))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block by height %d: %w", height, err)
	}

	blockHeader, err := blockInfoToChainBlockHeader(blockInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to convert block header by height from WoC to a chain block header: %w", err)
	}
	return blockHeader, nil
}

// IsValidRootForHeight checks if the provided Merkle root is valid for the given block height.
func (woc *WhatsOnChain) IsValidRootForHeight(ctx context.Context, root *chainhash.Hash, height uint32) (_ bool, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-IsValidRootForHeight", attribute.String("service", "whatsonchain"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if err = ctx.Err(); err != nil {
		return false, fmt.Errorf("context canceled while validating Merkle root for height %d: %w", height, err)
	}

	if cached, ok := woc.getRootFromCache(height); ok {
		return cached.IsEqual(root), nil
	}

	if err = woc.wait(ctx); err != nil {
		return false, err
	}

	blockInfo, err := woc.client.GetBlockByHeight(ctx, int64(height))
	if err != nil {
		if errors.Is(err, wocsdk.ErrBlockNotFound) {
			// Not found - do not cache.
			return false, nil
		}
		return false, fmt.Errorf("%s: %w", ServiceName, err)
	}

	remoteRoot, err := chainhash.NewHashFromHex(blockInfo.MerkleRoot)
	if err != nil {
		return false, fmt.Errorf("%s: failed to parse Merkle root %q for height %d: %w", ServiceName, blockInfo.MerkleRoot, height, err)
	}

	woc.storeRootInCache(height, remoteRoot)
	return remoteRoot.IsEqual(root), nil
}

// HashToHeader returns the chain block header for the given block hash.
func (woc *WhatsOnChain) HashToHeader(ctx context.Context, blockHash string) (_ *wdk.ChainBlockHeader, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-HashToHeader", attribute.String("service", "whatsonchain"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if err = woc.wait(ctx); err != nil {
		return nil, err
	}

	blockInfo, err := woc.client.GetHeaderByHash(ctx, blockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block header from WoC: %w", err)
	}

	chbh, err := blockInfoToChainBlockHeader(blockInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to convert WoC block header to ChainBlockHeader: %w", err)
	}
	return chbh, nil
}

// GetUtxoStatus retrieves the UTXO status for a given script hash and outpoint.
func (woc *WhatsOnChain) GetUtxoStatus(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (_ *wdk.UtxoStatusResult, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-GetUtxoStatus", attribute.String("service", "whatsonchain"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if err = validateScriptHash(scriptHash); err != nil {
		return nil, fmt.Errorf("invalid scripthash: %w", err)
	}

	if err = woc.wait(ctx); err != nil {
		return nil, err
	}

	records, err := woc.client.GetScriptUnspentTransactions(ctx, scriptHash)
	if err != nil && !errors.Is(err, wocsdk.ErrScriptNotFound) {
		return nil, fmt.Errorf("failed to query WoC for UTXO status: %w", err)
	}

	result := &wdk.UtxoStatusResult{
		Name:    ServiceName,
		Details: scriptRecordsToUtxoDetails(records),
	}

	if outpoint != nil {
		result.IsUtxo = txutils.ContainsUtxo(result.Details, outpoint)
	} else {
		result.IsUtxo = len(result.Details) > 0
	}

	return result, nil
}

// IsUtxo checks if the given outpoint is a UTXO for the specified script hash.
func (woc *WhatsOnChain) IsUtxo(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (_ bool, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-IsUtxo", attribute.String("service", "whatsonchain"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if scriptHash == "" {
		return false, fmt.Errorf("scriptHash is required")
	}
	if outpoint == nil {
		return false, fmt.Errorf("outpoint is required")
	}

	status, err := woc.GetUtxoStatus(ctx, scriptHash, outpoint)
	if err != nil {
		return false, fmt.Errorf("failed to determine UTXO status: %w", err)
	}

	return status.IsUtxo, nil
}

// GetStatusForTxIDs returns depth/status information for a list of txIDs.
func (woc *WhatsOnChain) GetStatusForTxIDs(ctx context.Context, txIDs []string) (_ *wdk.GetStatusForTxIDsResult, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-GetStatusForTxIDs", attribute.String("service", "whatsonchain"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txIDs) == 0 {
		return nil, fmt.Errorf("no txIDs provided")
	}

	results := make([]wdk.TxStatusDetail, 0, len(txIDs))
	for _, chunk := range chunkTxIDs(txIDs, maxTxsPerStatusRequest) {
		if err = woc.wait(ctx); err != nil {
			return nil, err
		}

		statuses, statusErr := woc.client.BulkTransactionStatus(ctx, &wocsdk.TxHashes{TxIDs: chunk})
		if statusErr != nil {
			return nil, fmt.Errorf("failed to get status for txIDs: %w", statusErr)
		}

		for _, status := range statuses {
			if status == nil {
				continue
			}
			results = append(results, woc.mapSingleTxStatus(status))
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results found for provided txIDs")
	}

	return &wdk.GetStatusForTxIDsResult{
		Name:    ServiceName,
		Status:  wdk.GetStatusSuccess,
		Results: results,
	}, nil
}

// mapSingleTxStatus converts an SDK TxStatus into the toolbox status detail.
func (woc *WhatsOnChain) mapSingleTxStatus(tx *wocsdk.TxStatus) wdk.TxStatusDetail {
	if tx.Error != "" {
		if tx.Error != "unknown" {
			woc.logger.WarnContext(context.Background(), "unexpected error for tx", slog.String("txid", tx.TxID), slog.String("error", tx.Error))
		}
		return wdk.TxStatusDetail{TxID: tx.TxID, Depth: nil, Status: wdk.ResultStatusForTxIDNotFound.String()}
	}

	if tx.Confirmations <= 0 {
		if tx.BlockHash != "" {
			woc.logger.WarnContext(context.Background(), "blockhash present but non-positive confirmations", slog.String("txid", tx.TxID), slog.String("blockhash", tx.BlockHash), slog.Int64("confirmations", tx.Confirmations))
		}
		return wdk.TxStatusDetail{TxID: tx.TxID, Depth: to.Ptr(0), Status: wdk.ResultStatusForTxIDKnown.String()}
	}

	return wdk.TxStatusDetail{
		TxID:   tx.TxID,
		Depth:  to.Ptr(int(tx.Confirmations)),
		Status: wdk.ResultStatusForTxIDMined.String(),
	}
}

// GetScriptHashHistory retrieves both confirmed and unconfirmed script history.
func (woc *WhatsOnChain) GetScriptHashHistory(ctx context.Context, scriptHash string) (_ *wdk.ScriptHistoryResult, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-GetScriptHashHistory", attribute.String("service", "whatsonchain"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if err = validateScriptHash(scriptHash); err != nil {
		return nil, err
	}

	if err = woc.wait(ctx); err != nil {
		return nil, err
	}
	confirmed, err := woc.client.GetScriptConfirmedHistory(ctx, scriptHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get confirmed script history: %w", err)
	}

	if err = woc.wait(ctx); err != nil {
		return nil, err
	}
	unconfirmed, err := woc.client.GetScriptUnconfirmedHistory(ctx, scriptHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get unconfirmed script history: %w", err)
	}

	history := make([]wdk.ScriptHistoryItem, 0, len(confirmed)+len(unconfirmed))
	for _, record := range confirmed {
		if record == nil {
			continue
		}
		history = append(history, scriptRecordToHistoryItem(record, true))
	}
	for _, record := range unconfirmed {
		if record == nil {
			continue
		}
		history = append(history, scriptRecordToHistoryItem(record, false))
	}

	return &wdk.ScriptHistoryResult{
		Name:       ServiceName,
		ScriptHash: scriptHash,
		History:    history,
	}, nil
}

func (woc *WhatsOnChain) getRootFromCache(height uint32) (*chainhash.Hash, bool) {
	woc.cacheMu.RLock()
	defer woc.cacheMu.RUnlock()
	val, ok := woc.rootCache[height]
	return val, ok
}

func (woc *WhatsOnChain) storeRootInCache(height uint32, root *chainhash.Hash) {
	woc.cacheMu.Lock()
	defer woc.cacheMu.Unlock()
	woc.rootCache[height] = root
}
