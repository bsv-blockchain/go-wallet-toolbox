package main

import (
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

func exampleFiatExchangeRate(srv *services.WalletServices) {
	examples := []struct {
		currency defs.Currency
		base     *defs.Currency
	}{
		{currency: defs.EUR, base: ptr(defs.USD)},
		{currency: defs.GBP, base: ptr(defs.EUR)},
		{currency: defs.GBP, base: nil},        // defaults to USD
		{currency: "ABC", base: ptr(defs.USD)}, // invalid case
	}

	for _, ex := range examples {
		base := "<nil>"
		if ex.base != nil {
			base = string(*ex.base)
		}
		step := fmt.Sprintf("Getting fiat rate for %s per %s", ex.currency, base)
		show.Step("FiatExchangeRate", step)

		rate := srv.FiatExchangeRate(ex.currency, ex.base)
		if rate == 0 {
			show.WalletError("FiatExchangeRate", fmt.Sprintf("%s/%s", ex.currency, base), fmt.Errorf("rate not found"))
		} else {
			show.WalletSuccess("FiatExchangeRate", fmt.Sprintf("%s/%s", ex.currency, base), rate)
		}
	}
}

func ptr(c defs.Currency) *defs.Currency {
	return &c
}

func main() {
	show.ProcessStart("Fiat Exchange Rate Conversion")

	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
	cfg.FiatExchangeRates = defs.FiatExchangeRates{
		Rates: map[defs.Currency]float64{
			defs.USD: 1.0,
			defs.EUR: 0.85,
			defs.GBP: 0.65,
		},
	}

	srv := services.New(slog.Default(), cfg)

	exampleFiatExchangeRate(srv)

	show.ProcessComplete("Fiat Exchange Rate Conversion")
}
