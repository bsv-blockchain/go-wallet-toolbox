package bitails_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/bitails"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

func TestBitails_PostBEEF(t *testing.T) {
	txSpec := testvectors.GivenTX().
		WithInput(100).
		WithP2PKHOutput(90)
	tx := txSpec.TX()
	calculatedTxID := tx.TxID().String()

	beef, err := transaction.NewBeefFromTransaction(tx)
	require.NoError(t, err)

	fixture := testabilities.Given(t)
	bitailsService := fixture.NewBitailsService()

	client := fixture.Bitails().HttpClient()
	httpmock.ActivateNonDefault(client.GetClient())
	defer httpmock.DeactivateAndReset()

	testCases := []struct {
		name               string
		setup              func()
		expectTxID         string
		expectErrorResult  bool
		expectDoubleSpend  bool
		expectAlreadyKnown bool
		expectNotes        []string
		expectBlockHeight  int64
		expectBlockHash    string
	}{
		{
			name: "success - matching txid",
			setup: func() {
				fixture.Bitails().OnBroadcast().WillReturnSuccess(calculatedTxID)
				fixture.Bitails().WillReturnTxInfo(calculatedTxID, "mocked-block-hash", 99999)
			},
			expectTxID:        calculatedTxID,
			expectErrorResult: false,
		},
		{
			name: "already in mempool",
			setup: func() {
				fixture.Bitails().OnBroadcast().WillReturnAlreadyInMempool(calculatedTxID, bitails.ErrAlreadyKnown)
				fixture.Bitails().WillReturnTxInfo(calculatedTxID, "mocked-block-hash", 99999)
			},
			expectTxID:         calculatedTxID,
			expectErrorResult:  false,
			expectAlreadyKnown: true,
			expectNotes:        []string{"already in mempool"},
		},
		{
			name: "double spend - missing inputs",
			setup: func() {
				fixture.Bitails().OnBroadcast().WillReturnDoubleSpend(calculatedTxID, bitails.ErrMissingInputs)
				fixture.Bitails().WillReturnTxInfo(calculatedTxID, "mocked-block-hash", 99999)
			},
			expectTxID:        calculatedTxID,
			expectErrorResult: false,
			expectDoubleSpend: true,
			expectNotes:       []string{"missing inputs"},
		},
		{
			name: "mismatched txid",
			setup: func() {
				fixture.Bitails().OnBroadcast().WillReturnSuccess("othertxid987")
				fixture.Bitails().WillReturnTxInfo(calculatedTxID, "mocked-block-hash", 99999)
			},
			expectTxID:        calculatedTxID,
			expectErrorResult: true,
			expectNotes:       []string{"returned txid (othertxid987) does not match expected txid (f036a074ace427c5ebc3d0de89a63d4a5e7aabeff9fbc77435eb58f8dbfd59a9)"},
		},
		{
			name: "internal error",
			setup: func() {
				fixture.Bitails().OnBroadcast().WillReturnHttpError(http.StatusInternalServerError)
				fixture.Bitails().WillReturnTxInfo(calculatedTxID, "mocked-block-hash", 99999)
			},
			expectTxID:        calculatedTxID,
			expectErrorResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			tc.setup()

			// when:
			result, err := bitailsService.PostBEEF(t.Context(), beef, []string{calculatedTxID})

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, result.TxIDResults, 1)

			assertTxIDResult(t, result.TxIDResults[0], tc)
		})
	}
}

func assertTxIDResult(t *testing.T, got wdk.PostedTxID, tc struct {
	name               string
	setup              func()
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
		assert.Condition(t, func() bool {
			for _, gotNote := range strNotes {
				if strings.Contains(gotNote, expectedNote) {
					return true
				}
			}
			return false
		}, fmt.Sprintf("Expected note to contain: %q, but got: %v", expectedNote, strNotes))
	}
}
