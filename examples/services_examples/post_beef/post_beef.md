# Post BEEF Example

This example demonstrates how to broadcast a BSV transaction using BEEF (Background Evaluation Extended Format) through multiple wallet services with automatic fallback. BEEF provides complete transaction data with merkle proofs for efficient network propagation.

## Overview

The process involves several steps:
1. Configuring transaction parameters including source BEEF data and private key.
2. Creating a new transaction that spends from the source transaction.
3. Converting the signed transaction to BEEF format for broadcasting.
4. Submitting the BEEF data to multiple blockchain services simultaneously.
5. Processing and displaying detailed responses from each service.

This approach ensures reliable transaction broadcasting by leveraging multiple service providers with automatic fallback mechanisms.

## Code Walkthrough

### Configuration Parameters

```go
const (
    wif = "cQFwZHWLNTd31aE8ZPtJ48gxQFg3PPSEyrumghwNN3znjARNgLYX"
    sourceTxBEEF = "0100beef01fe098c19000102..." // BEEF-encoded source transaction
    sourceOutputIndex = uint32(0)
    network = defs.NetworkTestnet
)
```
The example uses configurable constants for the private key (WIF format), source transaction BEEF data, output index to spend, and target network. These should be updated with your own values for actual use.

### Preparing the Transaction

```go
tx, err := prepareTransaction(sourceTxBEEF, wif, sourceOutputIndex, network)
```
The `prepareTransaction` function handles:
- Decoding the source BEEF transaction
- Creating unlocking scripts using the provided private key
- Building a new transaction with appropriate inputs and outputs
- Applying fees and signing the transaction

### Converting to BEEF Format

```go
beef, err := transaction.NewBeefFromTransaction(tx)
```
The signed transaction is converted to BEEF format, which includes the transaction data along with any required merkle proofs for validation by blockchain services.

### Broadcasting to Services

```go
serviceCfg := defs.DefaultServicesConfig(network)
walletServices := services.New(slog.Default(), serviceCfg)
results, err := walletServices.PostBEEF(context.Background(), beef, []string{tx.TxID().String()})
```
The BEEF data is submitted to multiple configured services simultaneously. The method returns results from all attempted services, allowing for comparison and fallback handling.

### Processing Results

```go
for _, result := range results {
    if !result.Success() {
        fmt.Println("Error:", result.Error)
    } else {
        // Display success details for each transaction
    }
}
```
Results from each service are processed and displayed, showing success status, transaction IDs, and any error information.

## Running the Example

**Prerequisites**:
- Update the `wif` constant with your private key
- Update `sourceTxBEEF` with valid BEEF data from a transaction you control
- Ensure the source transaction has unspent outputs

To run this example:

```bash
go run ./examples/services_examples/post_beef/post_beef.go
```

## Expected Output

```text
🚀 STARTING: Post BEEF
============================================================

=== STEP ===
Transaction is performing: preparing transaction from BEEF source
--------------------------------------------------

=== STEP ===
Wallet-Services is performing: broadcasting transaction 14f6d37f952d38398597df924342994124939f35f692676257a86bd8c52ab035
--------------------------------------------------
✅ SUCCESS: Posted BEEF to services

============================================================
POST BEEF RESULTS
============================================================

========================================
Service: ARC
✅ Success

  📋 Transaction Result:
    TX ID: 14f6d37f952d38398597df924342994124939f35f692676257a86bd8c52ab035
    Result: success
    Already Known: false
    Double Spend: false
    Block Hash:
    Block Height: 0
    Merkle Path: <nil>
    Competing TXs: []
    Notes: [0xc00007c340]
    Data:

========================================
Service: WhatsOnChain
✅ Success

  📋 Transaction Result:
    TX ID: 14f6d37f952d38398597df924342994124939f35f692676257a86bd8c52ab035
    Result: success
    Already Known: false
    Double Spend: false
    Block Hash:
    Block Height: 0
    Merkle Path: <nil>
    Competing TXs: []
    Notes: [0xc00007c340]
    Data:

========================================
Service: Bitails
✅ Success

  📋 Transaction Result:
    TX ID: 14f6d37f952d38398597df924342994124939f35f692676257a86bd8c52ab035
    Result: success
    Already Known: false
    Double Spend: false
    Block Hash:
    Block Height: 0
    Merkle Path: <nil>
    Competing TXs: []
    Notes: [0xc00007c340]
    Data:

============================================================
🎉 COMPLETED: Post BEEF

```

## Integration Steps

To integrate BEEF broadcasting into your application:

1. **Prepare transaction data** including source BEEF and private keys for signing.
2. **Create and sign transactions** using the go-sdk transaction building tools.
3. **Convert to BEEF format** using `transaction.NewBeefFromTransaction()`.
4. **Configure services** with appropriate network settings and API credentials.
5. **Submit BEEF data** using `walletServices.PostBEEF()` with transaction IDs.
6. **Process results** from multiple services to determine broadcast success.
7. **Implement retry logic** for failed broadcasts or service errors.
8. **Monitor transaction status** using the returned transaction IDs.

### Response Analysis

Each service response contains:

- **Success**: Boolean indicating if the broadcast succeeded
- **TxID**: The transaction identifier for tracking
- **Result**: Status string (`"success"`, `"error"`, etc.)
- **AlreadyKnown**: Whether the transaction was already in the service's mempool
- **DoubleSpend**: Information about potential double-spend conflicts
- **BlockHash/BlockHeight**: Block information if the transaction was mined
- **MerklePath**: Merkle proof data if available
- **CompetingTxs**: List of conflicting transactions
- **Error**: Detailed error information for failed broadcasts

## Additional Resources

- [BEEF Specification](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0062.md) - BRC-62 BEEF format documentation
- [Post BEEF Example](./post_beef.go) - Broadcast a BSV transaction using BEEF format
- [Post BEEF Hex Documentation](../post_beef_hex/post_beef_hex.md) - Broadcast from existing BEEF hex
- [Post Multiple Transactions Documentation](../post_beef_with_multiple_txs/post_beef_with_multiple_txs.md) - Broadcasting multiple transactions
