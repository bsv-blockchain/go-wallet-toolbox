package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
)

func TestUpdateBsvExchangeRateSuccess(t *testing.T) {
	t.Run("returns cached exchange rate if within update threshold", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.WhatsOnChain().WillRespondWithRates(500, "", nil)

		// and:
		cachedRate := defs.BSVExchangeRate{
			Timestamp: time.Now().Add(-5 * time.Minute),
			Base:      "USD",
			Rate:      100.0,
		}

		// and:
		services := given.Services().
			Config(testservices.WithBsvExchangeRate(cachedRate)).
			New()

		// when:
		result, err := services.BsvExchangeRate(t.Context())

		// then:
		require.NoError(t, err)
		assert.InDelta(t, cachedRate.Rate, result, 0.001)
	})

	t.Run("returns updated exchange rate when outside threshold", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.WhatsOnChain().WillRespondWithRates(200, `{
			"time": 123456,
			"rate": 50.5,
			"currency": "USD"
		}`, nil)

		// and:
		services := given.Services().
			Config(testservices.WithBsvExchangeRate(defs.BSVExchangeRate{
				Timestamp: time.Now().Add(-16 * time.Minute),
				Base:      "USD",
				Rate:      100.0,
			})).
			New()

		// when:
		result, err := services.BsvExchangeRate(t.Context())

		// then:
		require.NoError(t, err)
		assert.InDelta(t, 50.5, result, 0.001)
	})
}

func TestUpdateBsvExchangeRateFail(t *testing.T) {
	t.Run("returns error if HTTP request fails", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.WhatsOnChain().WillRespondWithRates(200, "", assert.AnError)

		// and:
		services := given.Services().New()

		// when:
		_, err := services.BsvExchangeRate(t.Context())

		// then:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch exchange rate")
	})

	t.Run("returns error if HTTP response is not 200", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.WhatsOnChain().WillRespondWithRates(500, "", nil)

		// and:
		services := given.Services().New()

		// when:
		_, err := services.BsvExchangeRate(t.Context())

		// then:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to retrieve successful response from WOC")
	})

	t.Run("returns error if currency is not USD", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		given.WhatsOnChain().WillRespondWithRates(200, `{
			"time": 123456,
			"rate": 50.5,
			"currency": "EUR"
      }`, nil)

		// and:
		services := given.Services().New()

		_, err := services.BsvExchangeRate(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported currency")
	})
}
