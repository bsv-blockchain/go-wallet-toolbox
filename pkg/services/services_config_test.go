package services_test

import (
	"context"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/require"
)

var mockTxID = testvectors.GivenTX().WithInput(10).WithP2PKHOutput(9).ID().String()

func TestServicesConfig_CustomService(t *testing.T) {
	given := testservices.GivenServices(t)
	counter := 0

	// and:
	customImplementation := services.Implementation{
		RawTx: func(ctx context.Context, txID string) (*wdk.RawTxResult, error) {
			counter++
			return &wdk.RawTxResult{}, nil
		},
		PostBEEF: func(ctx context.Context, beef *transaction.Beef, txIDs []string) (*wdk.PostedBEEF, error) {
			counter++
			return &wdk.PostedBEEF{}, nil
		},
		MerklePath: func(ctx context.Context, txID string) (*wdk.MerklePathResult, error) {
			counter++
			return &wdk.MerklePathResult{}, nil
		},
		FindChainTipHeader: func(ctx context.Context) (*wdk.ChainBlockHeader, error) {
			counter++
			return &wdk.ChainBlockHeader{}, nil
		},
		IsValidRootForHeight: func(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error) {
			counter++
			return true, nil
		},
		CurrentHeight: func(ctx context.Context) (uint32, error) {
			counter++
			return 0, nil
		},
		GetScriptHashHistory: func(ctx context.Context, scriptHash string) (*wdk.ScriptHistoryResult, error) {
			counter++
			return &wdk.ScriptHistoryResult{}, nil
		},
		HashToHeader: func(ctx context.Context, hash string) (*wdk.ChainBlockHeader, error) {
			counter++
			return &wdk.ChainBlockHeader{}, nil
		},
		ChainHeaderByHeight: func(ctx context.Context, height uint32) (*wdk.ChainBaseBlockHeader, error) {
			counter++
			return &wdk.ChainBaseBlockHeader{}, nil
		},
		GetStatusForTxIDs: func(ctx context.Context, txIDs []string) (*wdk.GetStatusForTxIDsResult, error) {
			counter++
			return &wdk.GetStatusForTxIDsResult{}, nil
		},
		GetUtxoStatus: func(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (*wdk.UtxoStatusResult, error) {
			counter++
			return &wdk.UtxoStatusResult{}, nil
		},
		IsUtxo: func(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (bool, error) {
			counter++
			return true, nil
		},
		BsvExchangeRate: func(ctx context.Context) (float64, error) {
			counter++
			return 0, nil
		},
	}

	// and:
	service := given.Services().
		Opts(services.WithCustomImplementation("custom", customImplementation)).
		New()

	// when:
	_, _ = service.RawTx(t.Context(), mockTxID)
	_, _ = service.PostBEEF(t.Context(), &transaction.Beef{}, []string{mockTxID})
	_, _ = service.MerklePath(t.Context(), mockTxID)
	_, _ = service.FindChainTipHeader(t.Context())
	_, _ = service.IsValidRootForHeight(t.Context(), &chainhash.Hash{}, 0)
	_, _ = service.CurrentHeight(t.Context())
	_, _ = service.GetScriptHashHistory(t.Context(), "scriptHash")
	_, _ = service.HashToHeader(t.Context(), "hash")
	_, _ = service.ChainHeaderByHeight(t.Context(), 0)
	_, _ = service.GetStatusForTxIDs(t.Context(), []string{mockTxID})
	_, _ = service.GetUtxoStatus(t.Context(), "scriptHash", &transaction.Outpoint{})
	_, _ = service.IsUtxo(t.Context(), "scriptHash", &transaction.Outpoint{})
	_, _ = service.BsvExchangeRate(t.Context())

	// then:
	require.Equal(t, 13, counter)
}
