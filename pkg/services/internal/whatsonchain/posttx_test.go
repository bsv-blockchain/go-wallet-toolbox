package whatsonchain_test

import (
	"net/http"
	"testing"

	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	mockBlockHash   = "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	mockBlockHeight = 123456
)

func TestWhatsOnChain_PostTX(t *testing.T) {
	txSpec := testvectors.GivenTX().
		WithInput(100).
		WithP2PKHOutput(90)
	givenTxID := txSpec.TX().TxID().String()

	rawTx := txSpec.TX().Bytes()

	tests := map[string]struct {
		setup        func(testabilities.WoCServiceFixture)
		resultStatus wdk.PostedTxIDResultStatus
		alreadyKnown bool
	}{
		"success - matching txid": {
			setup: func(given testabilities.WoCServiceFixture) {
				given.WhatsOnChain().WillRespondWithBroadcast(http.StatusOK, `{"txid":"`+givenTxID+`"}`)
				given.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{
					ExpectBlockHash:   mockBlockHash,
					ExpectBlockHeight: mockBlockHeight,
				})
			},
			resultStatus: wdk.PostedTxIDResultSuccess,
		},
		"success - already in mempool": {
			setup: func(given testabilities.WoCServiceFixture) {
				given.WhatsOnChain().WillRespondWithBroadcast(http.StatusInternalServerError, "already in mempool")
				given.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{
					ExpectBlockHash:   mockBlockHash,
					ExpectBlockHeight: mockBlockHeight,
				})
			},
			resultStatus: wdk.PostedTxIDResultAlreadyKnown,
			alreadyKnown: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			given := testabilities.Given(t)
			woc := given.NewWoCService()

			// and:
			test.setup(given)

			// when:
			result, err := woc.PostTX(t.Context(), rawTx)

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, test.resultStatus, result.Result)
			assert.Equal(t, givenTxID, result.TxID)
			require.NoError(t, result.Error)
			assert.False(t, result.DoubleSpend)
			assert.Equal(t, test.alreadyKnown, result.AlreadyKnown)
			assert.Empty(t, result.CompetingTxs)
			assert.Len(t, result.Notes, 1)
		})
	}
}

func TestWhatsOnChain_PostTX_ErrorCases(t *testing.T) {
	txSpec := testvectors.GivenTX().
		WithInput(100).
		WithP2PKHOutput(90)
	givenTxID := txSpec.TX().TxID().String()

	rawTx := txSpec.TX().Bytes()

	tests := map[string]struct {
		setup         func(testabilities.WoCServiceFixture)
		resultStatus  wdk.PostedTxIDResultStatus
		doubleSpend   bool
		additionalErr bool
	}{
		"double spend - missing inputs": {
			setup: func(given testabilities.WoCServiceFixture) {
				given.WhatsOnChain().WillRespondWithBroadcast(http.StatusInternalServerError, "txn-mempool-conflict")
				given.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{
					ExpectBlockHash:   mockBlockHash,
					ExpectBlockHeight: mockBlockHeight,
				})
			},
			resultStatus: wdk.PostedTxIDResultDoubleSpend,
			doubleSpend:  true,
		},
		"mismatched txid": {
			setup: func(given testabilities.WoCServiceFixture) {
				given.WhatsOnChain().WillRespondWithBroadcast(http.StatusInternalServerError, `{"txid":"othertxid987"}`)
			},
			resultStatus:  wdk.PostedTxIDResultError,
			additionalErr: true,
		},
		"internal error": {
			setup: func(given testabilities.WoCServiceFixture) {
				given.WhatsOnChain().WillRespondWithBroadcast(http.StatusInternalServerError, `unexpected response code 500: unknown failure`)
			},
			resultStatus:  wdk.PostedTxIDResultError,
			additionalErr: true,
		},
		"missing inputs": {
			setup: func(given testabilities.WoCServiceFixture) {
				given.WhatsOnChain().WillRespondWithBroadcast(http.StatusInternalServerError, `missing inputs`)
			},
			resultStatus: wdk.PostedTxIDResultMissingInputs,
			doubleSpend:  true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			given := testabilities.Given(t)
			woc := given.NewWoCService()

			// and:
			test.setup(given)

			// when:
			result, err := woc.PostTX(t.Context(), rawTx)

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, test.resultStatus, result.Result)
			assert.Equal(t, givenTxID, result.TxID)
			assert.Equal(t, test.doubleSpend, result.DoubleSpend)
			assert.False(t, result.AlreadyKnown)
			assert.Len(t, result.Notes, 1)

			if test.additionalErr {
				assert.Error(t, result.Error)
			}
		})
	}
}
