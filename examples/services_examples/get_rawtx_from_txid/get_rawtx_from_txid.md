# Get Raw Transaction from Transaction ID

This example demonstrates how to fetch a raw BSV transaction using its transaction ID through the Go Wallet Toolbox SDK. It showcases multiple wallet services with automatic fallback to retrieve transaction data in hexadecimal format from blockchain service providers.

## Overview

The process involves several steps:
1. Setting up services configuration with network settings and multiple blockchain service providers.
2. Configuring the target transaction ID for raw transaction data retrieval.
3. Submitting the transaction ID request to configured services with automatic failover.
4. Retrieving the raw transaction data in hexadecimal format from successful services.
5. Processing and displaying the raw transaction results with service attribution.

This approach ensures reliable transaction data retrieval by leveraging multiple service providers with automatic fallback mechanisms for redundancy.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable settings:

- **`Transaction ID`**: Specific transaction to retrieve raw data for (default: `"9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6"`)
- **`Network`**: Blockchain network to query (default: `NetworkMainnet`)
- **`Service Providers`**: Multiple blockchain services (WhatsOnChain and Bitails) with automatic fallback

### Service Setup

The `RawTx` method requires:

- **`Transaction ID`**: Hexadecimal transaction identifier for data retrieval
- **`Services Instance`**: Configured services with redundant provider access and fallback logic
- **`Network Configuration`**: Mainnet settings for accessing production blockchain data

### Response Analysis

The service response contains:

- **`TxID`**: The requested transaction identifier for verification and confirmation
- **`RawTx`**: The complete raw transaction data in hexadecimal format for parsing
- **`Service`**: The name of the blockchain service that provided the successful data response
- **`Success Status`**: Boolean indicating successful query completion and data retrieval

## Running the Example

To run this example:

```bash
go run ./examples/services_examples/get_rawtx_from_txid/get_rawtx_from_txid.go
```

## Expected Output

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

1. **Configure transaction ID** with the specific transaction you want to retrieve from the blockchain.
2. **Set network settings** appropriate for your target blockchain environment (mainnet, testnet, etc.).
3. **Configure services** with appropriate API credentials and endpoints for your target providers.
4. **Submit transaction query** using `RawTx()` with the transaction ID for data retrieval.
5. **Process response data** to extract the raw transaction hexadecimal data for further operations.
6. **Handle service fallback** by monitoring which providers successfully return transaction data.
7. **Parse transaction data** using appropriate transaction parsing libraries for detailed analysis if needed.

## Additional Resources

- [Get Raw Transaction from Transaction ID Example](./get_rawtx_from_txid.go) - Complete code example for fetching raw transaction data
- [Post BEEF Documentation](../post_beef/post_beef.md) - Broadcast a BSV transaction using BEEF format
- [Post BEEF Hex Documentation](../post_beef_hex/post_beef_hex.md) - Broadcast from existing BEEF hex
- [Post Multiple Transactions Documentation](../post_beef_with_multiple_txs/post_beef_with_multiple_txs.md) - Broadcasting multiple transactions
