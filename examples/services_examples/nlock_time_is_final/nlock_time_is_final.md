# NLockTime Finality

This example demonstrates how to check whether a given `nLockTime` value is considered **final** on the BSV blockchain using the Go Wallet Toolbox SDK. It supports evaluating both timestamp-based and block-height-based locktimes and determines their finality status based on current blockchain state.

## Overview

The `NLockTimeIsFinal` method determines whether a transaction can be accepted for mining based on its `nLockTime` value. A transaction is considered final if:
- It has a locktime of 0
- The locktime is less than the current block height (for block-based locktime)
- The locktime is less than the current UNIX timestamp (for time-based locktime)
- All input sequence numbers are `0xffffffff` (final)

## Code Walkthrough

The example demonstrates three scenarios:

### 1. Timestamp LockTime (Past)
```go
lockTime := uint32(time.Now().Unix() - 3600)
isFinal, err := srv.NLockTimeIsFinal(ctx, lockTime)
// ✅ Result: true
```

### 2. Timestamp LockTime (Future)
```go
lockTime := uint32(time.Now().Unix() + 3600)
isFinal, err := srv.NLockTimeIsFinal(ctx, lockTime)
// ❌ Result: false
```

### 3. Block Height LockTime
```go
lockTime := uint32(800000)
isFinal, err := srv.NLockTimeIsFinal(ctx, lockTime)
// ✅ Result: depends on current blockchain height
```

## Method Signature

```go
func (s *WalletServices) NLockTimeIsFinal(ctx context.Context, txOrLockTime any) (bool, error)
```

- **`txOrLockTime`**: Can be a `uint32`, `int`, transaction hex string, `sdk.Transaction`, etc.
- **Returns**: Whether the locktime is final and the transaction is ready to be accepted into a block.

## Running the Example

```bash
go run ./examples/services_examples/nlocktime_finality/nlocktime_finality.go
```

## Expected Output

```text
🚀 STARTING: Check nLockTime Finality
============================================================

=== STEP ===
Wallet-Services is performing: Checking finality for past timestamp locktime: 1754897347
--------------------------------------------------

 WALLET CALL: NLockTimeIsFinal
Args: 1754897347
✅ Result: true

=== STEP ===
Wallet-Services is performing: Checking finality for future timestamp locktime: 1754904547
--------------------------------------------------

 WALLET CALL: NLockTimeIsFinal
Args: 1754904547
✅ Result: false

=== STEP ===
Wallet-Services is performing: Checking finality for block height locktime: 800000
--------------------------------------------------

 WALLET CALL: NLockTimeIsFinal
Args: 800000
✅ Result: true
============================================================
🎉 COMPLETED: Check nLockTime Finality
```

## Integration Steps

1. **Import Wallet Toolbox** and initialize `WalletServices` with proper network config.
2. **Pass a locktime or transaction** into `NLockTimeIsFinal()`.
3. **Check returned boolean** to determine if the transaction is final.
4. **Handle errors gracefully**, especially with malformed inputs or failed service lookups.

## Additional Resources

- [NLockTimeIsFinal Code](./nlocktime_finality.go) - Full example implementation
- [Go-SDK Transaction Type](https://pkg.go.dev/github.com/bsv-blockchain/go-sdk/transaction) - For parsing raw transactions
- [Bitcoin nLockTime Reference](https://en.bitcoin.it/wiki/NLockTime) - Understanding nLockTime usage
