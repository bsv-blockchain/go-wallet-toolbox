package services_test

import (
	"encoding/hex"
	"fmt"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"testing"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateBsvExchangeRateSuccess(t *testing.T) {
	t.Run("returns cached exchange rate if within update threshold", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		given.WhatsOnChain().WillRespondWithRates(500, "", nil)

		// and:
		cachedRate := defs.BSVExchangeRate{
			Timestamp: time.Now().Add(-5 * time.Minute),
			Base:      "USD",
			Rate:      100.0,
		}

		// and:
		services := given.Services().WithBsvExchangeRate(cachedRate)

		// when:
		result, err := services.BsvExchangeRate()

		// then:
		assert.NoError(t, err)
		assert.Equal(t, cachedRate.Rate, result)
	})

	t.Run("returns updated exchange rate when outside threshold", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		given.WhatsOnChain().WillRespondWithRates(200, `{
			"time": 123456,
			"rate": 50.5,
			"currency": "USD"
		}`, nil)

		// and:
		services := given.Services().WithBsvExchangeRate(defs.BSVExchangeRate{
			Timestamp: time.Now().Add(-16 * time.Minute),
			Base:      "USD",
			Rate:      100.0,
		})

		// when:
		result, err := services.BsvExchangeRate()

		// then:
		assert.NoError(t, err)
		assert.Equal(t, 50.5, result)
	})
}

func TestUpdateBsvExchangeRateFail(t *testing.T) {
	t.Run("returns error if HTTP request fails", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		given.WhatsOnChain().WillRespondWithRates(200, "", assert.AnError)

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		_, err := services.BsvExchangeRate()

		// then:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch exchange rate")
	})

	t.Run("returns error if HTTP response is not 200", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		given.WhatsOnChain().WillRespondWithRates(500, "", nil)

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		_, err := services.BsvExchangeRate()

		// then:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to retrieve successful response from WOC")
	})

	t.Run("returns error if currency is not USD", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		given.WhatsOnChain().WillRespondWithRates(200, `{
			"time": 123456,
			"rate": 50.5,
			"currency": "EUR"
      }`, nil)

		// and:
		services := given.Services().WithDefaultConfig()

		_, err := services.BsvExchangeRate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported currency")
	})
}

func TestRawTxSuccess(t *testing.T) {
	t.Run("returns raw transaction when found", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		txID := "3c64c621c0070ea56ca2ef13ef699483c3938f48e030b184f1d094678eda7ab8"
		rawTxHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1703117b1900000000005f7c477c327c437c5f0006000000ffffffff016e2e5702000000001976a9147a112f6a373b80b4ebb2b02acef97f35aef7494488ac00000000"
		given.WhatsOnChain().WillRespondWithRawTx(200, txID, rawTxHex, nil)

		// and:
		services := given.Services().WithDefaultConfig()

		// and:
		decodedTx, err := hex.DecodeString(rawTxHex)
		require.NoError(t, err)
		expectedResult := wdk.RawTxResult{
			TxID:  txID,
			Name:  "WhatsOnChain",
			RawTx: decodedTx,
		}

		// when:
		result, err := services.RawTx(txID)

		// then:
		assert.NoError(t, err)
		assert.EqualValues(t, expectedResult, result)
	})
}

func TestRawTxFailure(t *testing.T) {
	t.Run("returns error when HTTP request fails", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		txID := "3c64c621c0070ea56ca2ef13ef699483c3938f48e030b184f1d094678eda7ab8"
		given.WhatsOnChain().WillRespondWithRawTx(400, txID, "", assert.AnError)

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		_, err := services.RawTx(txID)

		// then:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch raw tx hex")
		assert.Contains(t, err.Error(), "WhatsOnChain")
	})

	t.Run("returns error when HTTP request returns empty response", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		txID := "3c64c621c0070ea56ca2ef13ef699483c3938f48e030b184f1d094678eda7ab8"
		given.WhatsOnChain().WillRespondWithRawTx(200, txID, "", assert.AnError)

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		_, err := services.RawTx(txID)

		// then:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch raw tx hex")
		assert.Contains(t, err.Error(), "WhatsOnChain")
	})

	t.Run("returns error when HTTP request returns 404 Not Found", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		txID := "3c64c621c0070ea56ca2ef13ef699483c3938f48e030b184f1d094678eda7ab8"
		given.WhatsOnChain().WillRespondWithRawTx(404, txID, "404 Not Found", nil)

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		_, err := services.RawTx(txID)

		// then:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("transaction with txID: %s not found", txID))
	})

	t.Run("returns error when HTTP request returns status other than 200", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		txID := "3c64c621c0070ea56ca2ef13ef699483c3938f48e030b184f1d094678eda7ab8"
		given.WhatsOnChain().WillRespondWithRawTx(500, txID, "some internal error", nil)

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		_, err := services.RawTx(txID)

		// then:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to retrieve successful response from WOC")
	})

	t.Run("returns error when it fails to decode hex string", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		txID := "3c64c621c0070ea56ca2ef13ef699483c3938f48e030b184f1d094678eda7ab8"
		given.WhatsOnChain().WillRespondWithRawTx(200, txID, "illegal-%-hex-char-$", nil)

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		_, err := services.RawTx(txID)

		// then:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode raw transaction hex")
	})

	t.Run("returns error when computed txid doesn't match requested txid", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		txID := "otherTransactionId"
		// Valid hex but will hash to a different txid
		rawTxHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1703117b1900000000005f7c477c327c437c5f0006000000ffffffff016e2e5702000000001976a9147a112f6a373b80b4ebb2b02acef97f35aef7494488ac00000000"
		given.WhatsOnChain().WillRespondWithRawTx(200, txID, rawTxHex, nil)

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		_, err := services.RawTx(txID)

		// then:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "doesn't match requested value otherTransactionId")
	})

	t.Run("returns error when all services fail", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		txID := "abc123"

		// All services fail
		given.WhatsOnChain().WillRespondWithRawTx(500, txID, "", nil)

		// and:
		services := given.Services().WithDefaultConfig()

		// when:
		_, err := services.RawTx(txID)

		// then:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "all services failed")
	})
}
