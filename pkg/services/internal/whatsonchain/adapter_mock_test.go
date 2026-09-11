package whatsonchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	wocsdk "github.com/mrz1836/go-whatsonchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// mockSDKClient is a configurable stub of whatsonchain.SDKClient for unit-testing
// the adapter's mapping and classification logic without any HTTP transport.
type mockSDKClient struct {
	broadcastTx       func(ctx context.Context, txHex string) (string, error)
	bulkStatus        func(ctx context.Context, hashes *wocsdk.TxHashes) (wocsdk.TxStatusList, error)
	rawTx             func(ctx context.Context, hash string) (string, error)
	merkleTSC         func(ctx context.Context, hash string) (wocsdk.MerkleTSCResults, error)
	headerByHash      func(ctx context.Context, hash string) (*wocsdk.BlockInfo, error)
	headers           func(ctx context.Context) ([]*wocsdk.BlockInfo, error)
	blockByHeight     func(ctx context.Context, height int64) (*wocsdk.BlockInfo, error)
	chainInfo         func(ctx context.Context) (*wocsdk.ChainInfo, error)
	exchangeRate      func(ctx context.Context) (*wocsdk.ExchangeRate, error)
	scriptUnspent     func(ctx context.Context, scriptHash string) (wocsdk.ScriptList, error)
	scriptConfirmed   func(ctx context.Context, scriptHash string) (wocsdk.ScriptList, error)
	scriptUnconfirmed func(ctx context.Context, scriptHash string) (wocsdk.ScriptList, error)
}

func (m *mockSDKClient) BroadcastTx(ctx context.Context, txHex string) (string, error) {
	return m.broadcastTx(ctx, txHex)
}

func (m *mockSDKClient) BulkTransactionStatus(ctx context.Context, hashes *wocsdk.TxHashes) (wocsdk.TxStatusList, error) {
	return m.bulkStatus(ctx, hashes)
}

func (m *mockSDKClient) GetRawTransactionData(ctx context.Context, hash string) (string, error) {
	return m.rawTx(ctx, hash)
}

func (m *mockSDKClient) GetMerkleProofTSC(ctx context.Context, hash string) (wocsdk.MerkleTSCResults, error) {
	return m.merkleTSC(ctx, hash)
}

func (m *mockSDKClient) GetHeaderByHash(ctx context.Context, hash string) (*wocsdk.BlockInfo, error) {
	return m.headerByHash(ctx, hash)
}

func (m *mockSDKClient) GetHeaders(ctx context.Context) ([]*wocsdk.BlockInfo, error) {
	return m.headers(ctx)
}

func (m *mockSDKClient) GetBlockByHeight(ctx context.Context, height int64) (*wocsdk.BlockInfo, error) {
	return m.blockByHeight(ctx, height)
}

func (m *mockSDKClient) GetChainInfo(ctx context.Context) (*wocsdk.ChainInfo, error) {
	return m.chainInfo(ctx)
}

func (m *mockSDKClient) GetExchangeRate(ctx context.Context) (*wocsdk.ExchangeRate, error) {
	return m.exchangeRate(ctx)
}

func (m *mockSDKClient) GetScriptUnspentTransactions(ctx context.Context, scriptHash string) (wocsdk.ScriptList, error) {
	return m.scriptUnspent(ctx, scriptHash)
}

func (m *mockSDKClient) GetScriptConfirmedHistory(ctx context.Context, scriptHash string) (wocsdk.ScriptList, error) {
	return m.scriptConfirmed(ctx, scriptHash)
}

func (m *mockSDKClient) GetScriptUnconfirmedHistory(ctx context.Context, scriptHash string) (wocsdk.ScriptList, error) {
	return m.scriptUnconfirmed(ctx, scriptHash)
}

func (m *mockSDKClient) SetRateLimit(int) {}

func newMockedService(t *testing.T, client *mockSDKClient) *whatsonchain.WhatsOnChain {
	t.Helper()
	return whatsonchain.NewWithClient(client, logging.NewTestLogger(t), defs.WhatsOnChain{
		RequestsPerSecond: 10000,
	})
}

const mockTestTxID = "294cd1ebd5689fdee03509f92c32184c0f52f037d4046af250229b97e0c8f1aa"

func TestAdapter_PostTX_Classification(t *testing.T) {
	t.Parallel()

	// A minimal but valid raw tx so TransactionIDFromRawTx works.
	rawTx := []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	tests := []struct {
		name           string
		broadcastErr   error
		expectedResult wdk.PostedTxIDResultStatus
		expectError    bool // whether result.Error (a Go error) is populated
		expectDouble   bool
		expectKnown    bool
	}{
		// Double-spend / missing-inputs are terminal result-level verdicts: the
		// router keys off the DoubleSpend flag, so result.Error stays nil.
		{"success", nil, wdk.PostedTxIDResultSuccess, false, false, false},
		{"already in mempool", wocsdk.ErrTxAlreadyInMempool, wdk.PostedTxIDResultAlreadyKnown, false, false, true},
		{"mempool conflict", wocsdk.ErrTxMempoolConflict, wdk.PostedTxIDResultDoubleSpend, false, true, false},
		{"missing inputs", wocsdk.ErrTxMissingInputs, wdk.PostedTxIDResultMissingInputs, false, true, false},
		{"generic error", errors.New("boom"), wdk.PostedTxIDResultError, true, false, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			client := &mockSDKClient{
				broadcastTx: func(context.Context, string) (string, error) {
					if test.broadcastErr != nil {
						return "", test.broadcastErr
					}
					return mockTestTxID, nil
				},
				// enrichment is best-effort; return empty so it is a no-op.
				bulkStatus: func(context.Context, *wocsdk.TxHashes) (wocsdk.TxStatusList, error) {
					return wocsdk.TxStatusList{}, nil
				},
			}

			svc := newMockedService(t, client)
			result, err := svc.PostTX(context.Background(), rawTx)

			require.NoError(t, err, "PostTX must never return a Go error")
			require.NotNil(t, result)
			assert.Equal(t, test.expectedResult, result.Result)
			assert.Equal(t, test.expectDouble, result.DoubleSpend)
			assert.Equal(t, test.expectKnown, result.AlreadyKnown)
			if test.expectError {
				assert.Error(t, result.Error)
			} else {
				assert.NoError(t, result.Error)
			}
		})
	}
}

func TestAdapter_GetStatusForTxIDs_Mapping(t *testing.T) {
	t.Parallel()

	client := &mockSDKClient{
		bulkStatus: func(_ context.Context, hashes *wocsdk.TxHashes) (wocsdk.TxStatusList, error) {
			require.Len(t, hashes.TxIDs, 3)
			return wocsdk.TxStatusList{
				{TxID: "mined", Confirmations: 7, BlockHash: "abc", BlockHeight: 100},
				{TxID: "known", Confirmations: 0},
				{TxID: "unknown", Error: "unknown"},
			}, nil
		},
	}

	svc := newMockedService(t, client)
	result, err := svc.GetStatusForTxIDs(context.Background(), []string{"mined", "known", "unknown"})
	require.NoError(t, err)
	require.Len(t, result.Results, 3)

	mined := result.Results[0]
	assert.Equal(t, wdk.ResultStatusForTxIDMined.String(), mined.Status)
	require.NotNil(t, mined.Depth)
	assert.Equal(t, 7, *mined.Depth)

	known := result.Results[1]
	assert.Equal(t, wdk.ResultStatusForTxIDKnown.String(), known.Status)
	require.NotNil(t, known.Depth)
	assert.Equal(t, 0, *known.Depth)

	unknown := result.Results[2]
	assert.Equal(t, wdk.ResultStatusForTxIDNotFound.String(), unknown.Status)
	assert.Nil(t, unknown.Depth)
}

func TestAdapter_RawTx_NotFound(t *testing.T) {
	t.Parallel()

	client := &mockSDKClient{
		rawTx: func(context.Context, string) (string, error) {
			return "", wocsdk.ErrTransactionNotFound
		},
	}

	svc := newMockedService(t, client)
	result, err := svc.RawTx(context.Background(), mockTestTxID)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestAdapter_MerklePath_NotFound(t *testing.T) {
	t.Parallel()

	client := &mockSDKClient{
		merkleTSC: func(context.Context, string) (wocsdk.MerkleTSCResults, error) {
			return nil, wocsdk.ErrTransactionNotFound
		},
	}

	svc := newMockedService(t, client)
	_, err := svc.MerklePath(context.Background(), mockTestTxID)
	require.Error(t, err)
	assert.ErrorIs(t, err, wdk.ErrNotFoundError)
}

func TestAdapter_IsValidRootForHeight(t *testing.T) {
	t.Parallel()

	const merkleRootHex = "c7a78f2edd611b0fe7aad6829a243e4a9e351e5ab203b7beb875ba1e6a802461"
	root, err := chainhash.NewHashFromHex(merkleRootHex)
	require.NoError(t, err)

	t.Run("matching root", func(t *testing.T) {
		t.Parallel()
		client := &mockSDKClient{
			blockByHeight: func(context.Context, int64) (*wocsdk.BlockInfo, error) {
				return &wocsdk.BlockInfo{MerkleRoot: merkleRootHex}, nil
			},
		}
		ok, err := newMockedService(t, client).IsValidRootForHeight(context.Background(), root, 100)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("not found returns false without error", func(t *testing.T) {
		t.Parallel()
		client := &mockSDKClient{
			blockByHeight: func(context.Context, int64) (*wocsdk.BlockInfo, error) {
				return nil, wocsdk.ErrBlockNotFound
			},
		}
		ok, err := newMockedService(t, client).IsValidRootForHeight(context.Background(), root, 100)
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestAdapter_GetScriptHashHistory_Mapping(t *testing.T) {
	t.Parallel()

	const scriptHash = "995ea8d0f752f41cdd99bb9d54cb004709e04c7dc4088bcbbbb9ea5c390a43c3"

	client := &mockSDKClient{
		scriptConfirmed: func(context.Context, string) (wocsdk.ScriptList, error) {
			return wocsdk.ScriptList{{TxHash: "confirmed", Height: 620539}}, nil
		},
		scriptUnconfirmed: func(context.Context, string) (wocsdk.ScriptList, error) {
			return wocsdk.ScriptList{{TxHash: "unconfirmed"}}, nil
		},
	}

	result, err := newMockedService(t, client).GetScriptHashHistory(context.Background(), scriptHash)
	require.NoError(t, err)
	require.Len(t, result.History, 2)

	assert.Equal(t, "confirmed", result.History[0].TxHash)
	require.NotNil(t, result.History[0].Height)
	assert.Equal(t, 620539, *result.History[0].Height)

	assert.Equal(t, "unconfirmed", result.History[1].TxHash)
	assert.Nil(t, result.History[1].Height, "unconfirmed history items have no height")
}

func TestAdapter_GetUtxoStatus_NotFoundIsEmpty(t *testing.T) {
	t.Parallel()

	const scriptHash = "995ea8d0f752f41cdd99bb9d54cb004709e04c7dc4088bcbbbb9ea5c390a43c3"

	client := &mockSDKClient{
		scriptUnspent: func(context.Context, string) (wocsdk.ScriptList, error) {
			return nil, wocsdk.ErrScriptNotFound
		},
	}

	result, err := newMockedService(t, client).GetUtxoStatus(context.Background(), scriptHash, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsUtxo)
	assert.Empty(t, result.Details)
}
