package arcade_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// statusServer serves GET /tx/{txid} with a canned TXInfo per txid; unknown
// txids get 404 like a real Arcade.
func statusServer(t testing.TB, byTxID map[string]map[string]any) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		txID := strings.TrimPrefix(r.URL.Path, "/tx/")
		payload, ok := byTxID[txID]
		if !ok {
			writeJSON(t, w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		writeJSON(t, w, http.StatusOK, payload)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestGetStatusForTxIDs_MapsArcadeLifecycleToStatusContract(t *testing.T) {
	server := statusServer(t, map[string]map[string]any{
		"mined-tx": {"txid": "mined-tx", "txStatus": "MINED", "blockHeight": 95},
		"deep-tx":  {"txid": "deep-tx", "txStatus": "IMMUTABLE", "blockHeight": 50},
		"seen-tx":  {"txid": "seen-tx", "txStatus": "SEEN_MULTIPLE_NODES"},
		"bad-tx":   {"txid": "bad-tx", "txStatus": "REJECTED"},
	})

	service := newService(t, defaultConfig(server.URL))
	service.SetChainTipHeight(func(context.Context) (uint32, error) { return 100, nil })

	result, err := service.GetStatusForTxIDs(t.Context(),
		[]string{"mined-tx", "deep-tx", "seen-tx", "bad-tx", "missing-tx"})
	require.NoError(t, err)
	require.Equal(t, wdk.GetStatusSuccess, result.Status)
	require.Len(t, result.Results, 5)

	byID := map[string]wdk.TxStatusDetail{}
	for _, d := range result.Results {
		byID[d.TxID] = d
	}

	// MINED at height 95, tip 100 → 6 confirmations.
	require.Equal(t, wdk.ResultStatusForTxIDMined.String(), byID["mined-tx"].Status)
	require.NotNil(t, byID["mined-tx"].Depth)
	require.Equal(t, 6, *byID["mined-tx"].Depth)

	require.Equal(t, wdk.ResultStatusForTxIDMined.String(), byID["deep-tx"].Status)
	require.Equal(t, 51, *byID["deep-tx"].Depth)

	// Seen but unconfirmed → known, depth 0 (below any blocks_delay).
	require.Equal(t, wdk.ResultStatusForTxIDKnown.String(), byID["seen-tx"].Status)
	require.Equal(t, 0, *byID["seen-tx"].Depth)

	// Rejected and 404 are both off-chain → unknown, no depth.
	require.Equal(t, wdk.ResultStatusForTxIDNotFound.String(), byID["bad-tx"].Status)
	require.Nil(t, byID["bad-tx"].Depth)
	require.Equal(t, wdk.ResultStatusForTxIDNotFound.String(), byID["missing-tx"].Status)
	require.Nil(t, byID["missing-tx"].Depth)
}

func TestGetStatusForTxIDs_MinedWithoutTipUsesMinimumDepth(t *testing.T) {
	server := statusServer(t, map[string]map[string]any{
		"mined-tx": {"txid": "mined-tx", "txStatus": "MINED", "blockHeight": 95},
	})
	// No SetChainTipHeight wired at all.
	service := newService(t, defaultConfig(server.URL))

	result, err := service.GetStatusForTxIDs(t.Context(), []string{"mined-tx"})
	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.Equal(t, wdk.ResultStatusForTxIDMined.String(), result.Results[0].Status)
	require.NotNil(t, result.Results[0].Depth)
	require.Equal(t, 1, *result.Results[0].Depth, "no tip source → conservative minimum depth")
}

func TestGetStatusForTxIDs_EmptyInputErrors(t *testing.T) {
	server := statusServer(t, nil)
	service := newService(t, defaultConfig(server.URL))
	_, err := service.GetStatusForTxIDs(t.Context(), nil)
	require.Error(t, err)
}

func TestGetStatusForTxIDs_AllLookupsFailingReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	service := newService(t, defaultConfig(server.URL))
	_, err := service.GetStatusForTxIDs(t.Context(), []string{"a", "b"})
	require.Error(t, err)
}
