package bitails

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/internal/dto"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/servicequeue"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

type Bitails struct {
	httpClient                 *resty.Client
	url                        string
	apiKey                     string
	hashScriptHistoryPageLimit int
	logger                     *slog.Logger
	rootCache                  map[uint32]*chainhash.Hash // TODO: possibly handle by some caching structure/redis
	cacheMu                    sync.RWMutex
}

func New(httpClient *resty.Client, logger *slog.Logger, network defs.BSVNetwork, config defs.Bitails) *Bitails {
	logger = logging.Child(logger, "Bitails").With(slog.String("network", string(network)))

	headers := httpx.NewHeaders().
		AcceptJSON().
		UserAgent().Value("go-wallet-toolbox").
		Authorization().IfNotEmpty(config.APIKey)

	client := httpClient.
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(logger)).
		SetDebug(logging.IsDebug(logger))

	baseURL := ProductionURL
	if strings.ToLower(string(network)) == "test" {
		baseURL = TestnetURL
	}

	return &Bitails{
		httpClient: client,
		apiKey:     config.APIKey,
		url:        baseURL,
		logger:     logger,
		rootCache:  make(map[uint32]*chainhash.Hash),
	}
}

// PostBEEF sends the given beef with selected txIDs to Bitails for broadcasting.
func (b *Bitails) PostBEEF(ctx context.Context, beef *transaction.Beef, txIDs []string) (*wdk.PostedBEEF, error) {
	if beef == nil {
		return nil, fmt.Errorf("beef is required to post transactions")
	}
	if len(txIDs) == 0 {
		return nil, fmt.Errorf("no txIDs provided")
	}

	rawTxs, err := txutils.ExtractRawTransactions(beef, txIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to extract raw transactions: %w", err)
	}

	results := make([]wdk.PostedTxID, 0, len(txIDs))

	for i := range txIDs {
		raw := rawTxs[i]
		broadcastResult := b.broadcast(ctx, raw)
		results = append(results, broadcastResult)
	}

	return &wdk.PostedBEEF{TxIDResults: results}, nil
}

// IsValidRootForHeight checks if the supplied merkle-root belongs to the block at `height`.
func (b *Bitails) IsValidRootForHeight(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error) {
	if cached, ok := b.getRootFromCache(height); ok {
		return cached.IsEqual(root), nil
	}

	remoteRoot, err := b.fetchRemoteRoot(ctx, height)
	if err != nil {
		return false, fmt.Errorf("%s: %w", ServiceName, err)
	}
	if remoteRoot == nil {
		return false, nil
	}

	b.storeRootInCache(height, remoteRoot)
	return remoteRoot.IsEqual(root), nil
}

// MerklePath fetches a Merkle-path proof for the given txID using Bitails
func (b *Bitails) MerklePath(ctx context.Context, txID string) (*wdk.MerklePathResult, error) {
	proof, err := b.getTscProof(ctx, txID)
	if err != nil {
		return nil, err
	}
	if proof == nil {
		return &wdk.MerklePathResult{
			Name:  ServiceName,
			Notes: history.NewBuilder().GetMerklePathNotFound(ServiceName).Note().AsList(),
		}, nil
	}

	header, err := b.fetchMerkleHeader(ctx, proof.Target)
	if err != nil {
		return nil, fmt.Errorf("error converting hash to header: %w", err)
	}

	txInfo, err := b.fetchTxInfo(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("error fetching transaction info for txID %s: %w", txID, err)
	}
	header.Height, err = to.UInt32(txInfo.BlockHeight)
	if err != nil {
		return nil, fmt.Errorf("invalid block height %d for txID %s: %w", txInfo.BlockHeight, txID, err)
	}

	merklePath, err := txutils.ConvertTscProofToMerklePath(
		txID,
		proof.Index,
		proof.Nodes,
		header.Height,
	)
	if err != nil {
		return nil, fmt.Errorf("error converting TSC proof to Merkle path: %w", err)
	}

	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	if err != nil {
		return nil, fmt.Errorf("error computing Merkle root from path: %w", err)
	}
	if merkleRoot != header.MerkleRoot {
		return nil, fmt.Errorf("merkle root mismatch (got %s, want %s) for txID %s in block %s", merkleRoot, header.MerkleRoot, txID, header.Hash)
	}

	return &wdk.MerklePathResult{
		Name:        ServiceName,
		MerklePath:  merklePath,
		BlockHeader: header,
		Notes:       history.NewBuilder().GetMerklePathSuccess(ServiceName).Note().AsList(),
	}, nil
}

// FindChainTipHeader fetches the header of the current chain-tip block and converts it to *wdk.ChainBlockHeader.
func (b *Bitails) FindChainTipHeader(ctx context.Context) (*wdk.ChainBlockHeader, error) {
	hash, height, err := b.latestBlock(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching latest block: %w", err)
	}
	raw, err := b.rawHeader(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("error fetching raw block header: %w", err)
	}

	return ConvertHeader(raw, height)
}

// CurrentHeight contacts the Bitails API and returns the current best-chain height.
func (b *Bitails) CurrentHeight(ctx context.Context) (uint32, error) {
	url, err := buildURL(b.url, "network", "info")
	if err != nil {
		return 0, fmt.Errorf("error building URL: %w", err)
	}

	var payload dto.NetworkInfoResponse
	found, err := b.handleJSON(ctx, url, &payload, http.StatusOK, false)
	if err != nil {
		return 0, fmt.Errorf("error fetching current height: %w", err)
	}
	if !found {
		return 0, fmt.Errorf("unexpected 404 for %s", url)
	}

	if payload.Blocks == 0 {
		return 0, fmt.Errorf("API returned height %v", payload.Blocks)
	}

	height, err := to.UInt32(payload.Blocks)
	if err != nil {
		return 0, fmt.Errorf("invalid height %d in response: %w", payload.Blocks, err)
	}

	return height, nil
}

// RawTx fetches and validates the raw transaction for a given txID.
func (b *Bitails) RawTx(ctx context.Context, txID string) (*wdk.RawTxResult, error) {
	url, err := rawTxURL(b.url, txID)
	if err != nil {
		return nil, fmt.Errorf("error building raw tx URL: %w", err)
	}

	res, err := b.httpClient.R().
		SetContext(ctx).
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("%s: HTTP request failed for raw tx: %w", ServiceName, err)
	}

	switch res.StatusCode() {
	case http.StatusOK:
		// proceed
	case http.StatusNotFound:
		return nil, nil
	default:
		return nil, fmt.Errorf("%s: unexpected HTTP %d: %s", ServiceName, res.StatusCode(), res.String())
	}

	rawHex := string(res.Body())
	rawHex = strings.TrimSpace(rawHex)

	raw, err := hex.DecodeString(rawHex)
	if err != nil {
		return nil, fmt.Errorf("%s: decode hex failed: %w", ServiceName, err)
	}

	computedTxID := txutils.TransactionIDFromRawTx(raw)
	if txID != computedTxID {
		return nil, fmt.Errorf("%s: txID mismatch: expected %s, got %s", ServiceName, txID, computedTxID)
	}

	return &wdk.RawTxResult{
		Name:  ServiceName,
		TxID:  txID,
		RawTx: raw,
	}, nil
}

// HashToHeader fetches and decodes a block header by its hash.
func (b *Bitails) HashToHeader(ctx context.Context, blockHash string) (*wdk.ChainBlockHeader, error) {
	raw, err := b.rawHeader(ctx, blockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch raw header for hash %s: %w", blockHash, err)
	}

	return ConvertHeader(raw, 0)
}

// GetScriptHashHistory fetches the script hash history for a given script hash.
func (b *Bitails) GetScriptHashHistory(ctx context.Context, scriptHash string) (*wdk.ScriptHistoryResult, error) {
	if err := validateScriptHash(scriptHash); err != nil {
		return nil, fmt.Errorf("invalid script hash %s: %w", scriptHash, err)
	}

	scriptHistory, err := b.fetchScriptHistory(ctx, scriptHash)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch script hash history: %w", err)
	}

	items := slices.Map(scriptHistory, func(item dto.ScriptHistoryItem) wdk.ScriptHistoryItem {
		return wdk.ScriptHistoryItem{
			TxHash: item.TxID,
			Height: item.Height,
		}
	})

	return &wdk.ScriptHistoryResult{
		Name:       ServiceName,
		ScriptHash: scriptHash,
		History:    items,
	}, nil
}

// GetStatusForTxIDs returns depth/status info for a list of txIDs using Bitails.
func (b *Bitails) GetStatusForTxIDs(ctx context.Context, txIDs []string) (*wdk.GetStatusForTxIDsResult, error) {
	if len(txIDs) == 0 {
		return nil, fmt.Errorf("no txIDs provided")
	}

	tip, err := b.CurrentHeight(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get current height: %w", ServiceName, err)
	}

	res := &wdk.GetStatusForTxIDsResult{
		Name:    ServiceName,
		Status:  wdk.GetStatusSuccess,
		Results: make([]wdk.TxStatusDetail, 0, len(txIDs)),
	}

	var anyFound bool

	for _, txID := range txIDs {
		found, mined, height, err := b.getTxStatus(ctx, txID)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to get status for %s: %w", ServiceName, txID, err)
		}

		item := wdk.TxStatusDetail{TxID: txID}

		switch {
		case !found:
			item.Status = wdk.ResultStatusForTxIDNotFound.String()

		case mined:
			anyFound = true
			item.Status = wdk.ResultStatusForTxIDMined.String()
			depth, err := calcDepth(tip, height)
			if err != nil {
				return nil, fmt.Errorf("failed to calculate depth for %s: %w", txID, err)
			}
			item.Depth = depth

		default:
			anyFound = true
			item.Status = wdk.ResultStatusForTxIDKnown.String()
			item.Depth = to.Ptr(0)
		}

		res.Results = append(res.Results, item)
	}

	if !anyFound {
		return nil, servicequeue.ErrEmptyResult
	}
	return res, nil
}
