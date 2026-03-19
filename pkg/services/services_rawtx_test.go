package services_test

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"

	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestRawTxSuccess(t *testing.T) {
	t.Run("returns raw transaction when found", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		txID := "3c64c621c0070ea56ca2ef13ef699483c3938f48e030b184f1d094678eda7ab8"
		rawTxHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1703117b1900000000005f7c477c327c437c5f0006000000ffffffff016e2e5702000000001976a9147a112f6a373b80b4ebb2b02acef97f35aef7494488ac00000000"
		given.WhatsOnChain().WillRespondWithRawTx(200, txID, rawTxHex, nil)

		// and:
		services := given.Services().New()

		// and:
		decodedTx, err := hex.DecodeString(rawTxHex)
		require.NoError(t, err)
		expectedResult := wdk.RawTxResult{
			TxID:  txID,
			Name:  "WhatsOnChain",
			RawTx: decodedTx,
		}

		// when:
		result, err := services.RawTx(t.Context(), txID)

		// then:
		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})
	t.Run("returns raw transaction from Bitails when found", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		txID := "3c64c621c0070ea56ca2ef13ef699483c3938f48e030b184f1d094678eda7ab8"
		rawTxHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1703117b1900000000005f7c477c327c437c5f0006000000ffffffff016e2e5702000000001976a9147a112f6a373b80b4ebb2b02acef97f35aef7494488ac00000000"
		given.Bitails().WillReturnRawTxHex(txID, rawTxHex)

		services := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		decodedTx, err := hex.DecodeString(rawTxHex)
		require.NoError(t, err)

		expectedResult := wdk.RawTxResult{
			TxID:  txID,
			Name:  "Bitails",
			RawTx: decodedTx,
		}

		// when:
		result, err := services.RawTx(t.Context(), txID)

		// then:
		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})
}

func TestRawTxFailure(t *testing.T) {
	t.Run("returns error when HTTP request fails", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		txID := "3c64c621c0070ea56ca2ef13ef699483c3938f48e030b184f1d094678eda7ab8"
		given.WhatsOnChain().WillRespondWithRawTx(400, txID, "", assert.AnError)

		// and:
		services := given.Services().New()

		// when:
		_, err := services.RawTx(t.Context(), txID)

		// then:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch raw tx hex")
		assert.Contains(t, err.Error(), "WhatsOnChain")
	})

	t.Run("returns error when HTTP request returns empty response", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		txID := "3c64c621c0070ea56ca2ef13ef699483c3938f48e030b184f1d094678eda7ab8"
		given.WhatsOnChain().WillRespondWithRawTx(200, txID, "", assert.AnError)

		// and:
		services := given.Services().New()

		// when:
		_, err := services.RawTx(t.Context(), txID)

		// then:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch raw tx hex")
		assert.Contains(t, err.Error(), "WhatsOnChain")
	})

	t.Run("returns error when HTTP request returns 404 Not Found", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		txID := "3c64c621c0070ea56ca2ef13ef699483c3938f48e030b184f1d094678eda7ab8"
		given.WhatsOnChain().WillRespondWithRawTx(404, txID, "404 Not Found", nil)

		// and:
		services := given.Services().New()

		// when:
		_, err := services.RawTx(t.Context(), txID)

		// then:
		require.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("transaction with txID: %s not found", txID))
	})

	t.Run("returns error when HTTP request returns status other than 200", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		txID := "3c64c621c0070ea56ca2ef13ef699483c3938f48e030b184f1d094678eda7ab8"
		given.WhatsOnChain().WillRespondWithRawTx(500, txID, "some internal error", nil)

		// and:
		services := given.Services().New()

		// when:
		_, err := services.RawTx(t.Context(), txID)

		// then:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to retrieve successful response from WOC")
	})

	t.Run("returns error when it fails to decode hex string", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		txID := "3c64c621c0070ea56ca2ef13ef699483c3938f48e030b184f1d094678eda7ab8"
		given.WhatsOnChain().WillRespondWithRawTx(200, txID, "illegal-%-hex-char-$", nil)

		// and:
		services := given.Services().New()

		// when:
		_, err := services.RawTx(t.Context(), txID)

		// then:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode raw transaction hex")
	})

	t.Run("returns error when computed txid doesn't match requested txid", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		txID := "otherTransactionId"
		// Valid hex but will hash to a different txid
		rawTxHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1703117b1900000000005f7c477c327c437c5f0006000000ffffffff016e2e5702000000001976a9147a112f6a373b80b4ebb2b02acef97f35aef7494488ac00000000"
		given.WhatsOnChain().WillRespondWithRawTx(200, txID, rawTxHex, nil)

		// and:
		services := given.Services().New()

		// when:
		_, err := services.RawTx(t.Context(), txID)

		// then:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "doesn't match requested value otherTransactionId")
	})

	t.Run("returns error when all services fail", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		txID := "abc123"

		// All services fail
		given.WhatsOnChain().WillRespondWithRawTx(500, txID, "", nil)

		// and:
		services := given.Services().New()

		// when:
		_, err := services.RawTx(t.Context(), txID)

		// then:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "all services failed")
	})
	t.Run("Bitails returns error when raw tx hex is malformed", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		txID := "abc123"
		given.Bitails().WillReturnRawTxHex(txID, "bad-$$$-hex")

		services := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		// when:
		_, err := services.RawTx(t.Context(), txID)

		// then:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode hex failed")
		assert.Contains(t, err.Error(), "Bitails")
	})
}

func TestWalletServices_RawTx_ErrorCases(t *testing.T) {
	txID := "3c64c621c0070ea56ca2ef13ef699483c3938f48e030b184f1d094678eda7ab8"
	malformedHex := "illegal-%-hex-char-$"

	tests := []struct {
		name                 string
		setup                func(testservices.ServicesFixture)
		expectedErrorMessage string
	}{
		{
			name: "WOC unreachable - Bitails returns malformed hex",
			setup: func(f testservices.ServicesFixture) {
				err := f.WhatsOnChain().WillBeUnreachable()
				require.Error(t, err)
				f.Bitails().WillReturnRawTxHex(txID, malformedHex)
			},
			expectedErrorMessage: "Bitails: decode hex failed",
		},
		{
			name: "WOC unreachable - Bitails returns mismatched txid",
			setup: func(f testservices.ServicesFixture) {
				err := f.WhatsOnChain().WillBeUnreachable()
				require.Error(t, err)
				otherTx := testvectors.GivenTX().WithInput(1).WithP2PKHOutput(1).TX()
				otherRawHex := hex.EncodeToString(otherTx.Bytes())
				f.Bitails().WillReturnRawTxHex(txID, otherRawHex)
			},
			expectedErrorMessage: "txID mismatch",
		},
		{
			name: "WOC unreachable - Bitails returns 404",
			setup: func(f testservices.ServicesFixture) {
				err := f.WhatsOnChain().WillBeUnreachable()
				require.Error(t, err)
				f.Bitails().WillReturnRawTx404(txID)
			},
			expectedErrorMessage: fmt.Sprintf("transaction with txID: %s not found", txID),
		},
		{
			name: "WOC unreachable - Bitails returns HTTP error",
			setup: func(f testservices.ServicesFixture) {
				err := f.WhatsOnChain().WillBeUnreachable()
				require.Error(t, err)
				f.Bitails().WillReturnRawTxHttpError(txID, http.StatusInternalServerError)
			},
			expectedErrorMessage: "Bitails: unexpected HTTP 500",
		},
		{
			name: "all providers unreachable",
			setup: func(f testservices.ServicesFixture) {
				err := f.WhatsOnChain().WillBeUnreachable()
				require.Error(t, err)
				err = f.Bitails().WillBeUnreachable()
				require.Error(t, err)
			},
			expectedErrorMessage: "all services failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			given := testservices.GivenServices(t)
			tc.setup(given)

			svc := given.Services().Config(testservices.WithEnabledBitails(true)).New()

			// when:
			_, err := svc.RawTx(t.Context(), txID)

			// then:
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErrorMessage)
		})
	}
}
