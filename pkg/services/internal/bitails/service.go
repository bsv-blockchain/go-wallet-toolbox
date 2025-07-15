package bitails

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/utils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
)

type Bitails struct {
	httpClient *resty.Client
	url        string
	apiKey     string
	logger     *slog.Logger
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

	for i, txID := range txIDs {
		raw := rawTxs[i]
		broadcastResult, err := b.broadcast(ctx, raw)
		if err != nil {
			results = append(results, wdk.PostedTxID{
				TxID:   txID,
				Result: wdk.PostedTxIDResultError,
				Error:  fmt.Errorf("failed to broadcast tx %s: %w", txID, err),
				Notes:  utils.ConvertNotes([]string{err.Error()}),
			})
			continue
		}

		results = append(results, *broadcastResult)
	}

	return &wdk.PostedBEEF{TxIDResults: results}, nil
}

// MerklePath fetches a Merkle-path proof for the given txID using Bitails
func (b *Bitails) MerklePath(ctx context.Context, txID string) (*wdk.MerklePathResult, error) {
	proof, err := b.getTscProof(ctx, txID)
	if err != nil {
		return nil, err
	}
	if proof == nil {
		return &wdk.MerklePathResult{Name: ServiceName}, nil
	}

	header, err := b.hashToHeader(ctx, proof.Target)
	if err != nil {
		return nil, fmt.Errorf("failed for service %s to resolve block header for txID %s: %w", ServiceName, txID, err)
	}

	txInfo, err := b.fetchTxInfo(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("failed for service %s to fetch tx info for %s: %w", ServiceName, txID, err)
	}
	header.Height, err = to.UInt32(txInfo.BlockHeight)
	if err != nil {
		return nil, fmt.Errorf("failed for service %s: invalid block height %d for %s: %w", ServiceName, txInfo.BlockHeight, txID, err)
	}

	merklePath, err := txutils.ConvertTscProofToMerklePath(
		txID,
		proof.Index,
		proof.Nodes,
		header.Height,
	)
	if err != nil {
		return nil, fmt.Errorf("failed for service %s to convert TSC proof: %w", ServiceName, err)
	}

	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	if err != nil {
		return nil, fmt.Errorf("failed for service %s to compute merkle root: %w", ServiceName, err)
	}
	if merkleRoot != header.MerkleRoot {
		return nil, fmt.Errorf("failed for service %s: merkle root mismatch (got %s, want %s) for txID %s in block %s", ServiceName, merkleRoot, header.MerkleRoot, txID, header.Hash)
	}

	return &wdk.MerklePathResult{
		Name:        ServiceName,
		MerklePath:  merklePath,
		BlockHeader: header,
		Notes:       wdk.Notes{{When: to.Ptr(time.Now()), What: "getMerklePathTSC"}},
	}, nil
}

// FindChainTipHeader fetches the header of the current chain-tip block and converts it to *wdk.ChainBlockHeader.
func (b *Bitails) FindChainTipHeader(ctx context.Context) (*wdk.ChainBlockHeader, error) {
	hash, height, err := b.latestBlock(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed for service %s: to fetch latest block: %w", ServiceName, err)
	}
	raw, err := b.rawHeader(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed for service %s: to fetch raw header for block %s: %w", ServiceName, hash, err)
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
		return 0, fmt.Errorf("failed to build height URL: %w", err)
	}

	var payload networkInfoResponse
	res, err := b.httpClient.
		R().
		SetContext(ctx).
		SetResult(&payload).
		Get(url)
	if err != nil {
		return 0, fmt.Errorf("error from service %s: %w", ServiceName, err)
	}
	if res.StatusCode() != HTTPStatusOK {
		return 0, fmt.Errorf("unexpected HTTP %d for %s", res.StatusCode(), url)
	}
	if payload.Blocks == 0 {
		return 0, fmt.Errorf("API returned height 0")
	}

	height, err := to.UInt32(payload.Blocks)
	if err != nil {
		return 0, fmt.Errorf("invalid height %d in service %s response: %w", payload.Blocks, ServiceName, err)
	}
	return height, nil
}
