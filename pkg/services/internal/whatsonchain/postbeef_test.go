package whatsonchain_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestWhatsOnChain_PostBEEF(t *testing.T) {
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
			expectNotes:       []string{"Double spend detected"},
		},
		{
			name:              "missing inputs",
			httpStatus:        http.StatusInternalServerError,
			httpResponse:      "missing inputs",
			expectTxID:        calculatedTxID,
			expectErrorResult: false,
			expectDoubleSpend: true,
			expectNotes:       []string{"Missing inputs detected"},
		},
		{
			name:              "success - mismatched txid",
			httpStatus:        http.StatusOK,
			httpResponse:      `{"txid":"othertxid987"}`,
			expectErrorResult: true,
			expectTxID:        calculatedTxID,
		},
		{
			name:              "unknown error",
			httpStatus:        http.StatusInternalServerError,
			httpResponse:      "unexpected response code 500: unknown failure",
			expectErrorResult: true,
			expectTxID:        calculatedTxID,
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

			assertTxIDResult(t, result.TxIDResults[0], tc)
		})
	}
}

func assertTxIDResult(t *testing.T, got wdk.PostedTxID, tc struct {
	name               string
	httpStatus         int
	httpResponse       string
	expectTxID         string
	expectErrorResult  bool
	expectDoubleSpend  bool
	expectAlreadyKnown bool
	expectNotes        []string
}) {
	require.Equal(t, tc.expectTxID, got.TxID)

	if tc.expectErrorResult {
		require.Equal(t, wdk.PostedTxIDResultError, got.Result)
	} else {
		require.NotEqual(t, wdk.PostedTxIDResultError, got.Result)
	}

	require.Equal(t, tc.expectDoubleSpend, got.DoubleSpend)
	require.Equal(t, tc.expectAlreadyKnown, got.AlreadyKnown)

	for _, expectedNote := range tc.expectNotes {
		found := false
		for _, actualNote := range got.Notes {
			if strings.Contains(actualNote.What, expectedNote) {
				found = true
				break
			}
		}
		require.True(t, found, "expected note to contain: %q", expectedNote)
	}
}
