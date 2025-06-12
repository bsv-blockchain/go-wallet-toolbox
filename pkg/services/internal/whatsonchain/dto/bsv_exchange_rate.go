package dto

// BSVExchangeRateResponse represents the exchange rate information for BSV
// as returned by the WhatsOnChain API.
//
// It includes the timestamp of the rate, the exchange rate value itself,
// and the fiat currency that the rate is quoted in.
type BSVExchangeRateResponse struct {
	// Time is the Unix timestamp (in seconds) when the exchange rate was recorded.
	Time int `json:"time"`

	// Rate is the exchange rate of 1 BSV in the given fiat currency.
	Rate float64 `json:"rate"`

	// Currency is the fiat currency code (e.g., "USD", "EUR") for the exchange rate.
	Currency string `json:"currency"`
}
