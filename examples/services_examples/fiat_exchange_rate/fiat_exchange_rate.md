# Fiat Exchange Rate

This example demonstrates how to retrieve and convert fiat currency exchange rates using the Go Wallet Toolbox SDK. It supports converting one fiat currency to another based on the latest rates (e.g., EUR to USD, GBP to EUR).

## Overview

The process involves:
1. Setting up the service with a mock configuration including fiat exchange rates.
2. Calling the `FiatExchangeRate()` method to retrieve the exchange rate of one currency relative to another.
3. Processing the returned `float64` result representing the conversion rate.
4. Handling invalid currencies or missing data.

This showcases a simplified currency conversion mechanism used internally within the wallet toolbox.

## Code Walkthrough

### Configuration

The fiat exchange rates are mocked for testing or example purposes using a `map[defs.Currency]float64`:

```go
{
  USD: 1.0,
  EUR: 0.85,
  GBP: 0.65,
}
```

You can replace or update this map with actual rates from a provider or service.

### Method Signature

```go
func (s *WalletServices) FiatExchangeRate(currency defs.Currency, base *defs.Currency) float64
```

- **`currency`**: Target fiat currency (e.g., EUR).
- **`base`**: Base fiat currency to convert against (e.g., USD). Defaults to USD if `nil`.
- **Returns**: The conversion rate from `currency` to `base`.

### Example Scenarios

- `FiatExchangeRate(EUR, USD)` → 0.85 (EUR to USD)
- `FiatExchangeRate(GBP, EUR)` → 0.65 / 0.85 ≈ 0.7647
- `FiatExchangeRate(GBP, nil)` → 0.65 (GBP to default USD)
- `FiatExchangeRate(ABC, USD)` → Error (unknown currency)

## Running the Example

```bash
go run ./examples/services_examples/fiat_exchange_rate/fiat_exchange_rate.go
```

## Expected Output

```text
🚀 STARTING: Fiat Exchange Rate Conversion
============================================================

=== STEP ===
FiatExchangeRate is performing: Getting fiat rate for EUR per USD
--------------------------------------------------

 WALLET CALL: FiatExchangeRate
Args: EUR/USD
✅ Result: 0.85

=== STEP ===
FiatExchangeRate is performing: Getting fiat rate for GBP per EUR
--------------------------------------------------

 WALLET CALL: FiatExchangeRate
Args: GBP/EUR
✅ Result: 0.7647058823529412

=== STEP ===
FiatExchangeRate is performing: Getting fiat rate for GBP per <nil>
--------------------------------------------------

 WALLET CALL: FiatExchangeRate
Args: GBP/<nil>
✅ Result: 0.65

=== STEP ===
FiatExchangeRate is performing: Getting fiat rate for ABC per USD
--------------------------------------------------

 WALLET CALL: FiatExchangeRate
Args: ABC/USD
❌ Error: rate not found
============================================================
🎉 COMPLETED: Fiat Exchange Rate Conversion
```

## Integration Steps

1. **Add fiat exchange rates** to your service configuration or implement a live provider.
2. **Invoke `FiatExchangeRate()`** with the desired currency and optional base.
3. **Use the returned value** to display or convert currency values in your app.
4. **Handle errors gracefully** if a currency is missing or unknown.
5. **Cache frequently-used rates** to avoid redundant calls in performance-critical paths.

## Additional Resources

- [FiatExchangeRate Code](./fiat_exchange_rate.go) - Main example implementation
- [BSVExchangeRate](../bsv_exchange_rate/bsv_exchange_rate.go) - Check BSV/USD rate
- [Currency Definitions](../../../pkg/defs/currency.go) - Enum of supported fiat currencies
