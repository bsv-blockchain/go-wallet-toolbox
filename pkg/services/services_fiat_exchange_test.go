package services_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/stretchr/testify/assert"
)

func TestWalletServices_FiatExchangeRate(t *testing.T) {
	t.Run("returns 1 when same currency used for base", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		usd := defs.USD
		services := given.Services().Config(func(cfg *defs.WalletServices) {
			cfg.FiatExchangeRates = defs.FiatExchangeRates{
				Rates: map[defs.Currency]float64{
					usd: 1.0,
				},
			}
		}).New()

		// when:
		rate := services.FiatExchangeRate(usd, &usd)

		// then:
		assert.Equal(t, 1.0, rate)
	})

	t.Run("converts correctly from EUR to USD", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		eur := defs.EUR
		usd := defs.USD

		services := given.Services().Config(func(cfg *defs.WalletServices) {
			cfg.FiatExchangeRates = defs.FiatExchangeRates{
				Rates: map[defs.Currency]float64{
					usd: 1.0,
					eur: 0.85,
				},
			}
		}).New()

		// when:
		rate := services.FiatExchangeRate(eur, &usd)

		// then:
		assert.Equal(t, 0.85, rate)
	})

	t.Run("converts correctly from GBP to EUR", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		gbp := defs.GBP
		eur := defs.EUR

		services := given.Services().Config(func(cfg *defs.WalletServices) {
			cfg.FiatExchangeRates = defs.FiatExchangeRates{
				Rates: map[defs.Currency]float64{
					gbp: 0.6,
					eur: 0.9,
				},
			}
		}).New()

		// when:
		rate := services.FiatExchangeRate(gbp, &eur)

		// then:
		assert.InDelta(t, 0.6666, rate, 0.0001)
	})

	t.Run("returns 0 when currency is missing", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)

		usd := defs.USD
		gbp := defs.GBP

		services := given.Services().Config(func(cfg *defs.WalletServices) {
			cfg.FiatExchangeRates = defs.FiatExchangeRates{
				Rates: map[defs.Currency]float64{
					usd: 1.0,
				},
			}
		}).New()

		// when:
		rate := services.FiatExchangeRate(gbp, &usd)

		// then:
		assert.Equal(t, 0.0, rate)
	})
}
