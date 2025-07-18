package bitails

import (
	"context"
	"errors"
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
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
)

type Bitails struct {
	httpClient *resty.Client
	url        string
	apiKey     string
	logger     *slog.Logger
	rootCache  map[uint32]*chainhash.Hash // TODO: possibly handle by some caching structure/redis
	cacheMu    sync.RWMutex
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
		return nil, fmt.Errorf("no txids provided")
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

	header, err := b.hashToHeader(ctx, proof.Target)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("error from service %s: resolve block header for txID %s", ServiceName, txID))
	}

	txInfo, err := b.fetchTxInfo(ctx, txID)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("error from service %s: fetch tx info for txID %s", ServiceName, txID))
	}
	header.Height, err = to.UInt32(txInfo.BlockHeight)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("error from service %s: convert block height for txID %s", ServiceName, txID))
	}

	merklePath, err := txutils.ConvertTscProofToMerklePath(
		txID,
		proof.Index,
		proof.Nodes,
		header.Height,
	)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("error from service %s: convert TSC proof for txID %s", ServiceName, txID))
	}

	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("error from service %s: compute merkle root for txID %s", ServiceName, txID))
	}
	if merkleRoot != header.MerkleRoot {
		return nil, fmt.Errorf("error from %s: merkle root mismatch (got %s, want %s) for txID %s in block %s", ServiceName, merkleRoot, header.MerkleRoot, txID, header.Hash)
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
		return nil, errors.Join(err, fmt.Errorf("error from service %s: find chain tip header", ServiceName))
	}
	raw, err := b.rawHeader(ctx, hash)
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("error from service %s: get raw header for chain tip", ServiceName))
	}

	return ConvertHeader(raw, height)
}

type networkInfoResponse struct {
	Blocks uint64 `json:"blocks"`
}

// CurrentHeight contacts the Bitails API and returns the current best-chain height.
func (b *Bitails) CurrentHeight(ctx context.Context) (uint32, error) {
	url, err := buildURL(b.url, "network", "info")
	if err != nil {
		return 0, errors.Join(err, fmt.Errorf("error from service %s: build URL for network info", ServiceName))
	}

	var payload networkInfoResponse
	found, err := b.handleJSON(ctx, url, &payload, http.StatusOK, false)
	if err != nil {
		return 0, errors.Join(err, fmt.Errorf("error from service %s: get current height", ServiceName))
	}
	if !found {
		return 0, fmt.Errorf("unexpected 404 for service %s at %s", ServiceName, url)
	}

	if payload.Blocks == 0 {
		return 0, fmt.Errorf("API returned height 0 for service %s", ServiceName)
	}

	height, err := to.UInt32(payload.Blocks)
	if err != nil {
		return 0, fmt.Errorf("invalid height %d in service %s response: %w", payload.Blocks, ServiceName, err)
	}

	return height, nil
}
