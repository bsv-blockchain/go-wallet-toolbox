# Raw Transaction Example

This example demonstrates how to fetch a raw BSV transaction using its transaction ID through multiple wallet services with automatic fallback. The raw transaction data is retrieved in hexadecimal format directly from blockchain service providers.

## Overview

The process involves several steps:
1. Configuring the target transaction ID and network settings.
2. Setting up multiple blockchain service providers for redundant querying.
3. Submitting the transaction ID request to configured services simultaneously.
4. Retrieving the raw transaction data in hexadecimal format.
5. Processing and displaying the raw transaction results from each service.

This approach ensures reliable transaction data retrieval by leveraging multiple service providers with automatic fallback mechanisms.

## Code Walkthrough

### Configuration Parameters

```go
txID := "9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6"
network := defs.NetworkMainnet
```
The example uses a specific transaction ID on the mainnet for demonstration. You can replace this with any valid transaction ID you want to query.

### Service Configuration

```go
cfg := defs.DefaultServicesConfig(network)
srv := services.New(slog.Default(), cfg)
```
The code configures multiple blockchain service providers (WhatsOnChain and Bitails) with default settings for the specified network. This enables redundant querying with automatic fallback.

### Fetching Raw Transaction

```go
rawTx, err := srv.RawTx(txID)
```
The `RawTx` method queries the configured services to retrieve the raw transaction data. The method automatically handles:
- Parallel requests to multiple services
- Fallback to alternative services if one fails
- Response validation and error handling

### Processing Results

```go
show.RawTxOutput(&rawTx)
```
The results are processed and displayed, showing the transaction ID and raw transaction data in hexadecimal format from each successful service response.

## Running the Example

To run this example:

```bash
go run ./examples/services_examples/raw_tx/raw_tx.go
```

Expected output:

```text
🚀 STARTING: Raw Transaction from WhatsOnChain and Bitails
============================================================

=== STEP ===
Wallet-Services is performing: fetching RawTx for txID 9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6 using WhatsOnChain and Bitails
--------------------------------------------------
✅ SUCCESS: Success, Fetched Raw Transaction

============================================================
RAW TRANSACTION RESULT
============================================================
Service: WhatsOnChain
TxID:   9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6
RawTx:  01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff170399c80d2f43555656452f0150cbfa27d51703e1a32500ffffffff01f3d4a112000000001976a914d648686cf603c11850f39600e37312738accca8f88ac00000000
============================================================
🎉 COMPLETED: Raw Transaction fetching completed for txID 9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6
```

## Integration Steps

To integrate raw transaction fetching into your application:

1. **Configure transaction ID** with the specific transaction you want to retrieve.
2. **Set network settings** appropriate for your target blockchain (mainnet, testnet, etc.).
3. **Configure services** with appropriate API credentials and endpoints for your target providers.
4. **Submit transaction query** using `walletServices.RawTx()` with the transaction ID.
5. **Process results** to extract the raw transaction data for further processing.
6. **Implement retry logic** for failed queries or service errors.
7. **Parse raw transaction data** using appropriate transaction parsing libraries if needed.

### Response Analysis

The service response contains:

- **TxID**: The requested transaction identifier for verification
- **RawTx**: The complete raw transaction data in hexadecimal format
- **Service**: The name of the service that provided the data
- **Success**: Boolean indicating if the query succeeded
- **Error**: Detailed error information for failed queries

### Error Handling

```go
rawTx, err := srv.RawTx(txID)
if err != nil {
    log.Printf("Failed to fetch raw transaction: %v", err)
    return
}

// Validate that we received data
if rawTx.RawTx == "" {
    log.Printf("No raw transaction data received for TxID: %s", txID)
    return
}
```

### Use Cases

Raw transaction data retrieval is useful for:

- **Transaction Analysis**: Examining transaction structure, inputs, and outputs
- **Wallet Reconstruction**: Rebuilding wallet state from blockchain data
- **Audit Purposes**: Verifying transaction details and signatures
- **Backup and Recovery**: Storing transaction data for offline analysis
- **Integration Testing**: Using real transaction data in test scenarios

## Additional Resources

- [Post BEEF Documentation](../post_beef/post_beef.md) - Broadcast a BSV transaction using BEEF format
- [Post BEEF Hex Documentation](../post_beef_hex/post_beef_hex.md) - Broadcast from existing BEEF hex
- [Post Multiple Transactions Documentation](../post_beef_with_multiple_txs/post_beef_with_multiple_txs.md) - Broadcasting multiple transactions
- [Raw Transaction Example](./raw_tx.go) - Fetch raw transaction data by transaction ID 
