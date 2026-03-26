# Post Multiple BEEF Transactions

This example demonstrates broadcasting a chain of three dependent transactions using BEEF format through multiple wallet services.
Creates grandparent, parent, and child transactions where each spends from the previous transaction in the chain.

## Overview

The process involves several steps:
1. Loading and decoding the source BEEF-encoded transaction for the chain foundation.
2. Creating a chain of three dependent transactions (grandparent, parent, and child).
3. Converting the entire transaction chain to BEEF format with all dependencies.
4. Configuring multiple blockchain service providers for redundant broadcasting.
5. Submitting the BEEF data containing all three transactions to services simultaneously.
6. Processing and displaying detailed responses for each transaction from each service.

This approach ensures reliable transaction broadcasting by leveraging multiple service providers with automatic fallback mechanisms.

## Code Walkthrough

### Configuration Parameters

```go
const (
    wif = "cQFwZHWLNTd31aE8ZPtJ48gxQFg3PPSEyrumghwNN3znjARNgLYX"
    sourceTxBEEF = "hex here" // BEEF-encoded source transaction
    sourceOutputIndex = uint32(0)
    network = defs.NetworkTestnet
)
```
The example uses configurable constants for the private key (WIF format), source transaction BEEF data, output index to spend, and target network. These should be updated with your own values for actual use.

### Creating the Transaction Chain

```go
grandParentTx, err := prepareTransaction(sourceTxBEEF, privKey, sourceOutputIndex, network)
parentTx, err := addNextTransaction(grandParentTx, privKey, network)
tx, err := addNextTransaction(parentTx, privKey, network)
```
The example creates a chain of three dependent transactions:
- **Grandparent**: Spends from the source BEEF transaction
- **Parent**: Spends from the grandparent transaction (output index 0)
- **Child**: Spends from the parent transaction (output index 0)

Each transaction is built using the previous transaction as input, creating a dependency chain.

### Converting to BEEF Format

```go
beef, err := transaction.NewBeefFromTransaction(tx)
```
The entire transaction chain is converted to BEEF format. The BEEF automatically includes all dependent transactions (grandparent, parent, and child) along with any required merkle proofs for validation by blockchain services.

### Broadcasting to Services

```go
serviceCfg := defs.DefaultServicesConfig(network)
walletServices := services.New(slog.Default(), serviceCfg)
results, err := walletServices.PostBEEF(context.Background(), beef, []string{
    grandParentTx.TxID().String(),
    parentTx.TxID().String(),
    tx.TxID().String(),
})
```
The BEEF data containing all three transactions is submitted to multiple configured services simultaneously. All three transaction IDs are provided to track the broadcast status of each transaction in the chain.

### Processing Results

```go
show.PostBEEFOutput(results)
```
Results from each service are processed and displayed, showing success status and detailed information for each of the three transactions. This allows comparison of how different services handle the transaction chain.

## Running the Example

**Prerequisites**:
- Update `sourceTxBEEF` with valid BEEF hex data
- Update `wif` with your private key
- Ensure the source transaction has unspent outputs

```bash
go run ./examples/services_examples/post_beef_with_multiple_txs/post_beef_with_multiple_txs.go
```

## Expected Output

```text
🚀 STARTING: Post Multiple BEEF Transactions
============================================================

=== STEP ===
Transaction is performing: creating transaction chain from BEEF source
--------------------------------------------------

=== STEP ===
Wallet-Services is performing: broadcasting 3 chained transactions
--------------------------------------------------
✅ SUCCESS: Posted BEEF with multiple transactions to services

============================================================
POST BEEF RESULTS
============================================================

========================================
Service: ARC
✅ Success

  📋 Transaction Result:
    TX ID: 3914a8a1e9a73c2b462977826e43c612641050ad6c3777e6dd328535f23a328c
    Result: success
    Already Known: false
    Double Spend: false
    Block Hash:
    Block Height: 0
    Merkle Path: <nil>
    Competing TXs: []
    Notes: [0xc00007c340]
    Data:

  📋 Transaction Result:
    TX ID: 04bf7738c37ff8cf65ced6d12f2ae095da1d501f18fc764f59063681c4da56bc
    Result: success
    Already Known: false
    Double Spend: false
    Block Hash:
    Block Height: 0
    Merkle Path: <nil>
    Competing TXs: []
    Notes: [0xc00007c340]
    Data:

  📋 Transaction Result:
    TX ID: 23555c4de4a5e5ce3b90f48cc35a3210aa8c7d8579033bb32ad019ab3e95100e
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
    TX ID: 3914a8a1e9a73c2b462977826e43c612641050ad6c3777e6dd328535f23a328c
    Result: success
    Already Known: false
    Double Spend: false
    Block Hash:
    Block Height: 0
    Merkle Path: <nil>
    Competing TXs: []
    Notes: [0xc00007c340]
    Data:

  📋 Transaction Result:
    TX ID: 23555c4de4a5e5ce3b90f48cc35a3210aa8c7d8579033bb32ad019ab3e95100e
    Result: success
    Already Known: false
    Double Spend: false
    Block Hash:
    Block Height: 0
    Merkle Path: <nil>
    Competing TXs: []
    Notes: [0xc00007c340]
    Data:

  📋 Transaction Result:
    TX ID: 04bf7738c37ff8cf65ced6d12f2ae095da1d501f18fc764f59063681c4da56bc
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
    TX ID: 3914a8a1e9a73c2b462977826e43c612641050ad6c3777e6dd328535f23a328c
    Result: success
    Already Known: false
    Double Spend: false
    Block Hash:
    Block Height: 0
    Merkle Path: <nil>
    Competing TXs: []
    Notes: [0xc00007c340]
    Data:

  📋 Transaction Result:
    TX ID: 23555c4de4a5e5ce3b90f48cc35a3210aa8c7d8579033bb32ad019ab3e95100e
    Result: success
    Already Known: false
    Double Spend: false
    Block Hash:
    Block Height: 0
    Merkle Path: <nil>
    Competing TXs: []
    Notes: [0xc00007c340]
    Data:

  📋 Transaction Result:
    TX ID: 04bf7738c37ff8cf65ced6d12f2ae095da1d501f18fc764f59063681c4da56bc
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
🎉 COMPLETED: Post Multiple BEEF Transactions
```

## Integration Steps

To integrate chained BEEF transaction broadcasting into your application:

1. **Prepare transaction chain data** including source BEEF and private keys for signing all transactions.
2. **Create transaction dependencies** by building each transaction to spend from the previous one in the chain.
3. **Validate chain integrity** to ensure each transaction properly references the previous transaction's outputs.
4. **Convert to BEEF format** using `transaction.NewBeefFromTransaction()` on the final transaction to include the entire chain.
5. **Configure services** with appropriate network settings and API credentials for all target services.
6. **Submit BEEF data** using `walletServices.PostBEEF()` with all transaction IDs in the chain.
7. **Process results** from multiple services to determine broadcast success for each transaction in the chain.
8. **Implement retry logic** for failed broadcasts, considering that failure of parent transactions affects child transactions.
9. **Monitor transaction status** using all returned transaction IDs to track the entire chain's progress.

### Response Analysis

Each service returns detailed results for each transaction in the chain:

- **Success**: Boolean indicating if the broadcast succeeded for the entire chain
- **TxID**: The transaction identifier for each transaction in the chain (grandparent, parent, child)
- **Result**: Status string for each transaction (`"success"`, `"error"`, `"missing_inputs"`, etc.)
- **AlreadyKnown**: Whether each transaction was already in the service's mempool
- **DoubleSpend**: Information about potential double-spend conflicts for any transaction
- **BlockHash/BlockHeight**: Block information if any transaction was mined
- **MerklePath**: Merkle proof data if available for any transaction
- **CompetingTxs**: List of conflicting transactions that may affect the chain
- **Error**: Detailed error information for failed broadcasts at any level

## Additional Resources

- [BEEF Specification](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0062.md) - BRC-62 BEEF format documentation
- [Post BEEF Documentation](../post_beef/post_beef.md) - Broadcast a BSV transaction using BEEF format
- [Post BEEF Hex Documentation](../post_beef_hex/post_beef_hex.md) - Broadcast from existing BEEF hex
- [Post Multiple BEEF Example](./post_beef_with_multiple_txs.go) - Broadcasting multiple transactions
