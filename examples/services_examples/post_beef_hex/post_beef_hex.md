# Post BEEF Hex

This example demonstrates broadcasting a BSV transaction from an existing BEEF hex string using multiple wallet services.
Unlike creating transactions from scratch, this focuses purely on the broadcasting mechanism with pre-encoded BEEF data.

## Overview

The process involves several steps:
1. Decoding the provided BEEF hex string into binary format for processing.
2. Parsing the binary BEEF data to create a structured BEEF object.
3. Configuring multiple blockchain service providers for redundant broadcasting.
4. Submitting the BEEF data to all configured services simultaneously.
5. Processing and displaying detailed responses from each service.

This approach ensures reliable transaction broadcasting by leveraging multiple service providers with automatic fallback mechanisms.

## Code Walkthrough

### Configuration Parameters

```go
const (
    transactionID = "c7218bcddee6e7a2ad097007d50831837bb174ad78c078f65260d7971a46d620"
    beefHex = "0200beef01fef695190002..." // Complete BEEF hex string
    network = defs.NetworkTestnet
)
```
The example uses configurable constants for the transaction ID to broadcast, the complete BEEF hex string containing the transaction data, and the target network. The transaction ID must match the transaction contained in the BEEF data.

### Decoding BEEF Hex Data

```go
beefBytes, err := hex.DecodeString(beefHex)
beef, err := transaction.NewBeefFromBytes(beefBytes)
```
The BEEF hex string is first decoded into binary format, then parsed into a BEEF object. This BEEF object contains the transaction data along with any required merkle proofs and dependencies.

### Broadcasting to Services

```go
serviceCfg := defs.DefaultServicesConfig(network)
walletServices := services.New(slog.Default(), serviceCfg)
results, err := walletServices.PostBEEF(context.Background(), beef, []string{transactionID})
```
The parsed BEEF data is submitted to multiple configured services simultaneously. The transaction ID is provided to track the broadcast status of the specific transaction contained in the BEEF.

### Processing Results

```go
show.PostBEEFOutput(results)
```
Results from each service are processed and displayed, showing success status and detailed information for the broadcasted transaction. This allows comparison of how different services handle the same BEEF data.

## Running the Example

**Prerequisites**:
- Update `transactionID` with your transaction ID
- Update `beefHex` with valid BEEF hex data
- Ensure the transaction matches the BEEF content

```bash
go run ./examples/services_examples/post_beef_hex/post_beef_hex.go
```

## Expected Output

```text
🚀 STARTING: Post BEEF Hex
============================================================

=== STEP ===
Transaction is performing: parsing BEEF hex data
--------------------------------------------------

=== STEP ===
Wallet-Services is performing: broadcasting transaction c7218bcddee6e7a2ad097007d50831837bb174ad78c078f65260d7971a46d620
--------------------------------------------------
✅ SUCCESS: Posted BEEF to services

============================================================
POST BEEF RESULTS
============================================================

========================================
Service: ARC
✅ Success

  📋 Transaction Result:
    TX ID: c7218bcddee6e7a2ad097007d50831837bb174ad78c078f65260d7971a46d620
    Result: success
    Already Known: false
    Double Spend: false
    Block Hash: 0000000083da78df17e4616ace62a455850db086e957e6cc4c2cc0b4ba78527c
    Block Height: 1676910
    Merkle Path: <nil>
    Competing TXs: []
    Notes: [0xc0000c03c0]
    Data:

========================================
Service: WhatsOnChain
✅ Success

  📋 Transaction Result:
    TX ID: c7218bcddee6e7a2ad097007d50831837bb174ad78c078f65260d7971a46d620
    Result: success
    Already Known: false
    Double Spend: false
    Block Hash: 0000000083da78df17e4616ace62a455850db086e957e6cc4c2cc0b4ba78527c
    Block Height: 1676910
    Merkle Path: <nil>
    Competing TXs: []
    Notes: [0xc0000c03c0]
    Data:

========================================
Service: Bitails
✅ Success

  📋 Transaction Result:
    TX ID: c7218bcddee6e7a2ad097007d50831837bb174ad78c078f65260d7971a46d620
    Result: success
    Already Known: false
    Double Spend: false
    Block Hash:
    Block Height: 0
    Merkle Path: <nil>
    Competing TXs: []
    Notes: [0xc000332080]
    Data:
============================================================
🎉 COMPLETED: Post BEEF Hex
```

## Integration Steps

To integrate BEEF hex broadcasting into your application:

1. **Obtain BEEF hex data** from existing transactions or external sources that provide BEEF-encoded transaction data.
2. **Validate BEEF format** by ensuring the hex string is properly formatted and contains valid BEEF data.
3. **Extract transaction ID** from the BEEF data or ensure you have the correct transaction ID that matches the BEEF content.
4. **Decode and parse BEEF** using `hex.DecodeString()` and `transaction.NewBeefFromBytes()` to create BEEF objects.
5. **Configure services** with appropriate network settings and API credentials for all target services.
6. **Submit BEEF data** using `walletServices.PostBEEF()` with the corresponding transaction ID.
7. **Process results** from multiple services to determine broadcast success.
8. **Implement error handling** for malformed BEEF data, network issues, and service-specific errors.
9. **Monitor transaction status** using the returned transaction ID to track broadcast progress.

### Response Analysis

Each service returns detailed results for the broadcasted transaction:

- **Success**: Boolean indicating if the broadcast succeeded
- **TxID**: The transaction identifier matching the BEEF content
- **Result**: Status string (`"success"`, `"error"`, `"already_known"`, etc.)
- **AlreadyKnown**: Whether the transaction was already in the service's mempool
- **DoubleSpend**: Information about potential double-spend conflicts
- **BlockHash/BlockHeight**: Block information if the transaction was mined
- **MerklePath**: Merkle proof data if available
- **CompetingTxs**: List of conflicting transactions
- **Error**: Detailed error information for failed broadcasts

## Additional Resources

- [BEEF Specification](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0062.md) - BRC-62 BEEF format documentation
- [Post BEEF Documentation](../post_beef/post_beef.md) - Broadcast a BSV transaction using BEEF format
- [Post BEEF Hex Example](./post_beef_hex.go) - Broadcast from existing BEEF hex
- [Post Multiple BEEF Documentation](../post_beef_with_multiple_txs/post_beef_with_multiple_txs.md) - Broadcasting multiple transactions
