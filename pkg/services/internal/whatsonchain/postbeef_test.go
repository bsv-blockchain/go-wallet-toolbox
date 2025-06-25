package whatsonchain_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
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
	woc := fixture.NewWoCService(whatsonchain.WithBroadcastDelay(0))

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
		expectBlockHeight  int64
		expectBlockHash    string
	}{
		{
			name:              "success - matching txid",
			httpStatus:        http.StatusOK,
			httpResponse:      `{"txid":"` + calculatedTxID + `"}`,
			expectTxID:        calculatedTxID,
			expectErrorResult: false,
			expectBlockHeight: 123456,
			expectBlockHash:   "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		{
			name:               "already in mempool",
			httpStatus:         http.StatusInternalServerError,
			httpResponse:       "already in mempool",
			expectTxID:         calculatedTxID,
			expectErrorResult:  false,
			expectAlreadyKnown: true,
			expectNotes:        []string{"Transaction already in mempool"},
			expectBlockHeight:  0,
			expectBlockHash:    "",
		},
		{
			name:              "double spend",
			httpStatus:        http.StatusInternalServerError,
			httpResponse:      "txn-mempool-conflict",
			expectTxID:        calculatedTxID,
			expectErrorResult: false,
			expectDoubleSpend: true,
			expectNotes:       []string{"Double spend detected"},
			expectBlockHeight: 0,
			expectBlockHash:   "",
		},
		{
			name:              "missing inputs",
			httpStatus:        http.StatusInternalServerError,
			httpResponse:      "missing inputs",
			expectTxID:        calculatedTxID,
			expectErrorResult: false,
			expectDoubleSpend: true,
			expectNotes:       []string{"Missing inputs detected"},
			expectBlockHeight: 0,
			expectBlockHash:   "",
		},
		{
			name:              "success - mismatched txid",
			httpStatus:        http.StatusOK,
			httpResponse:      `{"txid":"othertxid987"}`,
			expectErrorResult: true,
			expectTxID:        calculatedTxID,
			expectBlockHeight: 0,
			expectBlockHash:   "",
		},
		{
			name:              "unknown error",
			httpStatus:        http.StatusInternalServerError,
			httpResponse:      "unexpected response code 500: unknown failure",
			expectErrorResult: true,
			expectTxID:        calculatedTxID,
			expectBlockHeight: 0,
			expectBlockHash:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Given:
			fixture.WhatsOnChain().WillRespondWithBroadcast(tc.httpStatus, tc.httpResponse)

			fixture.WhatsOnChain().Transport().RegisterResponder("POST",
				fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/txs/status", fixture.Network()),
				func(req *http.Request) (*http.Response, error) {
					var body struct {
						Txids []string `json:"txids"`
					}
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						return httpmock.NewStringResponse(http.StatusBadRequest, "bad request"), nil
					}

					respItems := []map[string]interface{}{}
					for _, txid := range body.Txids {
						respItems = append(respItems, map[string]interface{}{
							"txid":          txid,
							"blockhash":     tc.expectBlockHash,
							"blockheight":   tc.expectBlockHeight,
							"confirmations": 10,
							"time":          1599999999,
							"blocktime":     1599999999,
						})
					}

					respBytes, _ := json.Marshal(respItems)
					resp := httpmock.NewStringResponse(http.StatusOK, string(respBytes))
					resp.Header.Set("Content-Type", "application/json")
					return resp, nil
				})

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
	expectBlockHeight  int64
	expectBlockHash    string
}) {
	require.Equal(t, tc.expectTxID, got.TxID)

	if tc.expectErrorResult {
		require.Equal(t, wdk.PostedTxIDResultError, got.Result)
	} else {
		require.NotEqual(t, wdk.PostedTxIDResultError, got.Result)
		require.Nil(t, got.Error)
	}

	require.Equal(t, tc.expectDoubleSpend, got.DoubleSpend)
	require.Equal(t, tc.expectAlreadyKnown, got.AlreadyKnown)

	require.Equal(t, tc.expectBlockHeight, got.BlockHeight)
	require.Equal(t, tc.expectBlockHash, got.BlockHash)

	strNotes := slices.Map(got.Notes, func(note wdk.ReqHistoryNote) string {
		return note.What
	})

	for _, expectedNote := range tc.expectNotes {
		assert.Contains(t, strNotes, expectedNote)
	}
}
