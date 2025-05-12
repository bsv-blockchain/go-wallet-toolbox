package services_test

import (
	"testing"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/assert"
)

func TestUpdateBsvExchangeRateSuccess(t *testing.T) {
	t.Run("returns cached exchange rate if within update threshold", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		given.WhatsOnChain().WillRespondWithRates(500, "", nil)

		// and:
		cachedRate := wdk.BSVExchangeRate{
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
		services := given.Services().WithBsvExchangeRate(wdk.BSVExchangeRate{
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
