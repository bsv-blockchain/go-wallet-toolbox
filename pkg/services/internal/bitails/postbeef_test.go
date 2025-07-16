package bitails_test

import (
	"net/http"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBitails_PostBEEF(t *testing.T) {
	txSpec := testvectors.GivenTX().
		WithInput(100).
		WithP2PKHOutput(90)
	givenTxID := txSpec.TX().TxID().String()

	beef, err := transaction.NewBeefFromTransaction(txSpec.TX())
	require.NoError(t, err)

	tests := map[string]struct {
		setup        func(testabilities.BitailsServiceFixture)
		resultStatus wdk.PostedTxIDResultStatus
		alreadyKnown bool
	}{
		"success - matching txid": {
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().OnBroadcast().WillReturnSuccess(givenTxID)
				given.Bitails().WillReturnTxInfo(givenTxID, "mocked-block-hash", 99999)
			},
			resultStatus: wdk.PostedTxIDResultSuccess,
		},
		"success - already in mempool": {
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().OnBroadcast().WillReturnAlreadyInMempool(givenTxID, bitails.ErrAlreadyKnown)
				given.Bitails().WillReturnTxInfo(givenTxID, "mocked-block-hash", 99999)
			},
			resultStatus: wdk.PostedTxIDResultAlreadyKnown,
			alreadyKnown: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)
			bitailsService := given.NewBitailsService()

			client := given.Bitails().HttpClient()
			httpmock.ActivateNonDefault(client.GetClient())
			defer httpmock.DeactivateAndReset()

			// and:
			test.setup(given)

			// when:
			result, err := bitailsService.PostBEEF(t.Context(), beef, []string{givenTxID})

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, result.TxIDResults, 1)

			singleResult := result.TxIDResults[0]
			assert.Equal(t, test.resultStatus, singleResult.Result)
			assert.Equal(t, givenTxID, singleResult.TxID)
			assert.Nil(t, singleResult.Error)
			assert.False(t, singleResult.DoubleSpend)
			assert.Equal(t, test.alreadyKnown, singleResult.AlreadyKnown)
			assert.Len(t, singleResult.CompetingTxs, 0)
			assert.Len(t, singleResult.Notes, 1)
		})
	}
}

func TestBitails_PostBEEF_ErrorCases(t *testing.T) {
	txSpec := testvectors.GivenTX().
		WithInput(100).
		WithP2PKHOutput(90)
	givenTxID := txSpec.TX().TxID().String()

	beef, err := transaction.NewBeefFromTransaction(txSpec.TX())
	require.NoError(t, err)

	tests := map[string]struct {
		setup         func(testabilities.BitailsServiceFixture)
		resultStatus  wdk.PostedTxIDResultStatus
		doubleSpend   bool
		additionalErr bool
	}{
		"double spend - missing inputs": {
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().OnBroadcast().WillReturnDoubleSpend(givenTxID, bitails.ErrMissingInputs)
				given.Bitails().WillReturnTxInfo(givenTxID, "mocked-block-hash", 99999)
			},
			resultStatus: wdk.PostedTxIDResultDoubleSpend,
			doubleSpend:  true,
		},
		"mismatched txid": {
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().OnBroadcast().WillReturnSuccess("othertxid987")
				given.Bitails().WillReturnTxInfo(givenTxID, "mocked-block-hash", 99999)
			},
			resultStatus:  wdk.PostedTxIDResultError,
			additionalErr: true,
		},
		"internal error": {
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().OnBroadcast().WillReturnHttpError(http.StatusInternalServerError)
				given.Bitails().WillReturnTxInfo(givenTxID, "mocked-block-hash", 99999)
			},
			resultStatus:  wdk.PostedTxIDResultError,
			additionalErr: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)
			bitailsService := given.NewBitailsService()

			client := given.Bitails().HttpClient()
			httpmock.ActivateNonDefault(client.GetClient())
			defer httpmock.DeactivateAndReset()

			// and:
			test.setup(given)

			// when:
			result, err := bitailsService.PostBEEF(t.Context(), beef, []string{givenTxID})

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, result.TxIDResults, 1)

			singleResult := result.TxIDResults[0]
			assert.Equal(t, test.resultStatus, singleResult.Result)
			assert.Equal(t, givenTxID, singleResult.TxID)
			assert.Equal(t, test.doubleSpend, singleResult.DoubleSpend)
			assert.False(t, singleResult.AlreadyKnown)
			assert.Len(t, singleResult.Notes, 1)

			if test.additionalErr {
				assert.Error(t, singleResult.Error)
			}
		})
	}
}
