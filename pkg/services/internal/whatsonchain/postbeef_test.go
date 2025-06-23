package whatsonchain_test

import (
	"net/http"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestWhatsOnChain_PostBEEF(t *testing.T) {
	// Given:
	txSpec := testvectors.GivenTX().
		WithInput(100).
		WithP2PKHOutput(90)
	tx := txSpec.TX()
	calculatedTxID := tx.TxID().String()

	beef, err := transaction.NewBeefFromTransaction(tx)
	require.NoError(t, err)

	fixture := testabilities.Given(t)
	woc := fixture.NewWoCService()

	client := fixture.WhatsOnChain().HttpClient()
	httpmock.ActivateNonDefault(client.GetClient())
	defer httpmock.DeactivateAndReset()

	testCases := []struct {
		name               string
		httpStatus         int
		httpResponse       string
		expectTxID         string
		expectErrorResult  bool
		expectDoubleSpend  bool
		expectAlreadyKnown bool
		expectNotes        []string
	}{
		{
			name:              "success - matching txid",
			httpStatus:        http.StatusOK,
			httpResponse:      `{"txid":"` + calculatedTxID + `"}`,
			expectTxID:        calculatedTxID,
			expectErrorResult: false,
		},
		{
			name:               "already in mempool",
			httpStatus:         http.StatusInternalServerError,
			httpResponse:       "already in mempool",
			expectTxID:         calculatedTxID,
			expectErrorResult:  false,
			expectAlreadyKnown: true,
			expectNotes:        []string{"Transaction already in mempool"},
		},
		{
			name:              "double spend",
			httpStatus:        http.StatusInternalServerError,
			httpResponse:      "txn-mempool-conflict",
			expectTxID:        calculatedTxID,
			expectErrorResult: false,
			expectDoubleSpend: true,
			expectNotes:       []string{"Possible double spend (txn-mempool-conflict)"},
		},
		{
			name:              "missing inputs",
			httpStatus:        http.StatusInternalServerError,
			httpResponse:      "missing inputs",
			expectTxID:        calculatedTxID,
			expectErrorResult: false,
			expectDoubleSpend: true,
			expectNotes:       []string{"Missing inputs (possible double spend)"},
		},
		{
			name:              "success - mismatched txid",
			httpStatus:        http.StatusOK,
			httpResponse:      `{"txid":"othertxid987"}`,
			expectErrorResult: true,
			expectTxID:        "f036a074ace427c5ebc3d0de89a63d4a5e7aabeff9fbc77435eb58f8dbfd59a9",
		},
		{
			name:              "unknown error",
			httpStatus:        http.StatusInternalServerError,
			httpResponse:      "unexpected response code 500: unknown failure",
			expectErrorResult: true,
			expectTxID:        "f036a074ace427c5ebc3d0de89a63d4a5e7aabeff9fbc77435eb58f8dbfd59a9",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Given:
			fixture.WhatsOnChain().WillRespondWithBroadcast(tc.httpStatus, tc.httpResponse, nil)

			// When:
			result, err := woc.PostBEEF(t.Context(), beef, []string{calculatedTxID})

			// Then:
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, result.TxIDResults, 1)

			assertTxIDResult(t, result.TxIDResults[0], tc.expectTxID, tc.expectErrorResult, tc.expectDoubleSpend, tc.expectAlreadyKnown, tc.expectNotes)
		})
	}
}

func assertTxIDResult(t *testing.T, got wdk.PostedTxID, expectTxID string, expectErrorResult, expectDoubleSpend, expectAlreadyKnown bool, expectNotes []string) {
	require.Equal(t, expectTxID, got.TxID)

	if expectErrorResult {
		require.Equal(t, wdk.PostedTxIDResultError, got.Result)
	} else {
		require.NotEqual(t, wdk.PostedTxIDResultError, got.Result)
	}

	require.Equal(t, expectDoubleSpend, got.DoubleSpend)
	require.Equal(t, expectAlreadyKnown, got.AlreadyKnown)

	for i, note := range expectNotes {
		require.Contains(t, got.Notes[i].What, note)
	}
}
