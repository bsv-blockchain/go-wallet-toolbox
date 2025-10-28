package whatsonchain

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/internal/dto"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

const ServiceName = defs.WhatsOnChainServiceName

type WhatsOnChain struct {
	httpClient *resty.Client
	url        string
	apiKey     string
	logger     *slog.Logger

	bsvExchangeRate            defs.BSVExchangeRate // TODO: possibly handle by some caching structure/redis
	bsvUpdateInterval          time.Duration
	rootForHeightRetryInterval time.Duration
	rootForHeightRetries       int
	rootCache                  map[uint32]*chainhash.Hash // TODO: possibly handle by some caching structure/redis
	cacheMu                    sync.RWMutex
}

func New(httpClient *resty.Client, logger *slog.Logger, network defs.BSVNetwork, config defs.WhatsOnChain) *WhatsOnChain {
	logger = logging.Child(logger, "WoC").With(slog.String("network", string(network)))

	err := network.Validate()
	if err != nil {
		panic(fmt.Sprintf("invalid BSV network configuration: %s", err.Error()))
	}

	url, err := MakeBaseURL(network)
	if err != nil {
		panic(fmt.Sprintf("failed to build base URL for WhatsOnChain: %s", err.Error()))
	}

	headers := httpx.NewHeaders().
		AcceptJSON().
		UserAgent().Value("go-wallet-toolbox").
		Authorization().IfNotEmpty(config.APIKey)

	client := httpClient.
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(logger)).
		SetDebug(logging.IsDebug(logger))

	return &WhatsOnChain{
		httpClient:                 client,
		apiKey:                     config.APIKey,
		url:                        url,
		logger:                     logger,
		bsvExchangeRate:            config.BSVExchangeRate,
		bsvUpdateInterval:          to.If(config.BSVUpdateInterval != nil, func() time.Duration { return *config.BSVUpdateInterval }).ElseThen(defs.DefaultBSVExchangeUpdateInterval),
		rootForHeightRetryInterval: config.RootForHeightRetryInterval,
		rootForHeightRetries:       config.RootForHeightRetries,
		rootCache:                  make(map[uint32]*chainhash.Hash),
	}
}

func (woc *WhatsOnChain) RawTx(ctx context.Context, txID string) (*wdk.RawTxResult, error) {
	req := woc.httpClient.
		R().
		SetContext(ctx).
		SetHeader("Cache-Control", "no-cache")

	res, err := req.Get(fmt.Sprintf("%s/tx/%s/hex", woc.url, txID))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch raw tx hex: %w", err)
	}
	if res.StatusCode() == http.StatusNotFound {
		return nil, nil
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to retrieve successful response from WOC. Actual status: %d", res.StatusCode())
	}

	txHexDecoded, err := hex.DecodeString(res.String())
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

func (woc *WhatsOnChain) UpdateBsvExchangeRate(ctx context.Context) (float64, error) {
	nextUpdate := woc.bsvExchangeRate.Timestamp.Add(woc.bsvUpdateInterval)

	// Check if the rate timestamp is newer than the threshold time
	if nextUpdate.After(time.Now()) {
		return woc.bsvExchangeRate.Rate, nil
	}

	var exchangeRateResponse dto.BSVExchangeRateResponse
	req := woc.httpClient.R()

	res, err := req.
		SetContext(ctx).
		SetResult(&exchangeRateResponse).
		Get(fmt.Sprintf("%s/exchangerate", woc.url))
	if err != nil {
		return 0, fmt.Errorf("failed to fetch exchange rate: %w", err)
	}

	if res.StatusCode() != http.StatusOK {
		return 0, fmt.Errorf("failed to retrieve successful response from WOC. Actual status: %d", res.StatusCode())
	}

	if exchangeRateResponse.Currency != string(defs.USD) {
		return 0, fmt.Errorf("unsupported currency returned from Whats On Chain")
	}

	return exchangeRateResponse.Rate, nil
}

// MerklePath retrieves the merkle path for a transaction using WoC TSC proof.
func (woc *WhatsOnChain) MerklePath(ctx context.Context, txID string) (*wdk.MerklePathResult, error) {
	proof, err := woc.getTscProof(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get TSC proof: %w", err)
	}
	if proof == nil {
		// Proof not found
		return &wdk.MerklePathResult{
			Name:  ServiceName,
			Notes: history.NewBuilder().GetMerklePathNotFound(ServiceName).Note().AsList(),
		}, nil
	}

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

func (woc *WhatsOnChain) FindChainTipHeader(ctx context.Context) (*wdk.ChainBlockHeader, error) {
	var blocks []dto.BlockHeader
	url := fmt.Sprintf("%s/block/headers?limit=1", woc.url)
	res, err := woc.
		httpClient.
		R().
		SetContext(ctx).
		SetResult(&blocks).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error while fetching block headers from WhatsOnChain (URL: %s): %w", url, err)
	}

	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected response from WhatsOnChain (URL: %s): status code %d", url, res.StatusCode())
	}

	if len(blocks) == 0 {
		return nil, fmt.Errorf("no block headers returned from WhatsOnChain (URL: %s); at least one expected", url)
	}

	first := blocks[0]
	header, err := first.ConvertToChainBlockHeader()
	if err != nil {
		return nil, fmt.Errorf("error while converting the response from WhatsOnChain (URL: %s) to the *wdk.ChainBlockHeader: %w", url, err)
	}

	return header, nil
}

// PostBEEF attempts to post beef with given txIDs
func (woc *WhatsOnChain) PostBEEF(ctx context.Context, beef *transaction.Beef, txIDs []string) (*wdk.PostedBEEF, error) {
	if len(txIDs) == 0 {
		return nil, fmt.Errorf("no txIDs provided")
	}
	if beef == nil {
		return nil, fmt.Errorf("beef is required to post transactions")
	}

	rawTxs, err := txutils.ExtractRawTransactions(beef, txIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to extract raw transactions: %w", err)
	}

	txResults := make([]wdk.PostedTxID, 0, len(txIDs))

	for i, txid := range txIDs {
		result := woc.processSingleTx(ctx, txid, rawTxs[i])
		txResults = append(txResults, result)
	}

	return &wdk.PostedBEEF{TxIDResults: txResults}, nil
}

// IsValidRootForHeight checks if the provided Merkle root is valid for the given block height.
func (woc *WhatsOnChain) IsValidRootForHeight(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf("context canceled while validating Merkle root for height %d: %w", height, err)
	}

	if cached, ok := woc.getRootFromCache(height); ok {
		return cached.IsEqual(root), nil
	}

	remoteRoot, err := woc.fetchRemoteRoot(ctx, height)
	if err != nil {
		return false, fmt.Errorf("%s: %w", ServiceName, err)
	}
	if remoteRoot == nil {
		return false, nil
	}

	woc.storeRootInCache(height, remoteRoot)
	return remoteRoot.IsEqual(root), nil
}

func (woc *WhatsOnChain) HashToHeader(ctx context.Context, blockHash string) (*wdk.ChainBlockHeader, error) {
	url, err := blockHeaderByHashURL(woc.url, blockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to construct URL for block hash %s: %w", blockHash, err)
	}

	var dto dto.BlockHeader
	resp, err := woc.httpClient.
		R().
		SetContext(ctx).
		SetResult(&dto).
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block header from WoC: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status %d", resp.StatusCode())
	}

	chbh, err := dto.ConvertToChainBlockHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to convert WoC block header to ChainBlockHeader: %w", err)
	}
	return chbh, nil
}

// GetUtxoStatus retrieves the UTXO status for a given script hash and outpoint.
func (woc *WhatsOnChain) GetUtxoStatus(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (*wdk.UtxoStatusResult, error) {
	if err := validateScriptHash(scriptHash); err != nil {
		return nil, fmt.Errorf("invalid scripthash: %w", err)
	}

	url, err := scriptUnspentAllURL(woc.url, scriptHash)
	if err != nil {
		return nil, fmt.Errorf("failed to build WoC URL: %w", err)
	}

	var response dto.ScriptHashUnspentResponse
	res, err := woc.httpClient.
		R().
		SetContext(ctx).
		SetResult(&response).
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to query WoC for UTXO status: %w", err)
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d from WoC", res.StatusCode())
	}
	if response.Error != "" {
		return nil, fmt.Errorf("WoC API error: %s", response.Error)
	}

	result := &wdk.UtxoStatusResult{
		Name: ServiceName,
		Details: slices.Map(response.Result, func(item dto.ScriptHashUnspentItem) wdk.UtxoDetail {
			return wdk.UtxoDetail{
				TxID:     item.TxHash,
				Index:    item.TxPos,
				Height:   item.Height,
				Satoshis: item.Value,
			}
		}),
	}

	if outpoint != nil {
		result.IsUtxo = txutils.ContainsUtxo(result.Details, outpoint)
	} else {
		result.IsUtxo = len(result.Details) > 0
	}

	return result, nil
}

// IsUtxo checks if the given outpoint is a UTXO for the specified script hash.
func (woc *WhatsOnChain) IsUtxo(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (bool, error) {
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

func (woc *WhatsOnChain) GetStatusForTxIDs(ctx context.Context, txIDs []string) (*wdk.GetStatusForTxIDsResult, error) {
	if len(txIDs) == 0 {
		return nil, fmt.Errorf("no txIDs provided")
	}

	url, err := txsStatusURL(woc.url)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL: %w", err)
	}

	results, err := woc.getStatusForTxIDs(ctx, url, txIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get status for txIDs: %w", err)
	}

	if results == nil {
		return nil, fmt.Errorf("no status found for provided txIDs")
	}

	if len(results.Results) == 0 {
		return nil, fmt.Errorf("no results found for provided txIDs")
	}

	if results.Status != wdk.GetStatusSuccess {
		return nil, fmt.Errorf("failed to get status for txIDs: %s", results.Status)
	}

	return results, nil
}
