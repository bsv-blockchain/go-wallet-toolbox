package defs

import (
	"fmt"
	"time"
)

// Currency represents supported currency types
type Currency string

// Supported currency types
const (
	USD Currency = "USD"
	EUR Currency = "EUR"
	GBP Currency = "GBP"
)

// ParseCurrencyType parses a string into a Currency (case-insensitive).
func ParseCurrencyType(str string) (Currency, error) {
	return parseEnumCaseInsensitive(str, USD, EUR, GBP)
}

// FiatExchangeRates is the rate struct for fiat currency
type FiatExchangeRates struct {
	Timestamp time.Time            `mapstructure:"timestamp"`
	Rates     map[Currency]float64 `mapstructure:"rates"`
	Base      Currency             `mapstructure:"base"`
}

// Validate checks the validity of the FiatExchangeRates struct
func (fe *FiatExchangeRates) Validate() error {
	var err error
	var sanitizedKey Currency

	if fe.Base, err = ParseCurrencyType(string(fe.Base)); err != nil {
		return fmt.Errorf("invalid base currency: %s", fe.Base)
	}

	for k, rate := range fe.Rates {
		if rate <= 0 {
			return fmt.Errorf("invalid rate for currency %s: %f", k, rate)
		}

		if sanitizedKey, err = ParseCurrencyType(string(k)); err != nil {
			return fmt.Errorf("invalid currency: %s", k)
		}

		if sanitizedKey != k {
			fe.Rates[sanitizedKey] = fe.Rates[k]
			delete(fe.Rates, k)
		}
	}
	return nil
}

// BSVExchangeRate is the rate struct for BSV exchange
type BSVExchangeRate struct {
	Timestamp time.Time `mapstructure:"timestamp"`
	Rate      float64   `mapstructure:"rates"`
	Base      Currency  `mapstructure:"base"`
}

// Validate checks the validity of the BSVExchangeRate struct
func (b *BSVExchangeRate) Validate() error {
	if b.Rate <= 0 {
		return fmt.Errorf("invalid rate for currency %s: %f", b.Base, b.Rate)
	}

	var err error
	if b.Base, err = ParseCurrencyType(string(b.Base)); err != nil {
		return fmt.Errorf("invalid base currency: %s", b.Base)
	}

	return nil
}
