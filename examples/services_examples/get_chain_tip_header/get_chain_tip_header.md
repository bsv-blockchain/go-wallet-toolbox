# Get Chain Tip Header

This example demonstrates how to retrieve the complete block header information for the latest block (chain tip) on the BSV blockchain using the Go Wallet Toolbox SDK. It showcases accessing blockchain data services to get detailed block metadata.

## Overview

The process involves several steps:
1. Setting up services configuration with network settings and API credentials.
2. Creating a services instance with logging configuration.
3. Calling `FindChainTipHeader()` to retrieve the latest block header data.
4. Processing and displaying the complete block header information.
5. Handling response data with comprehensive blockchain metadata.

This approach provides access to essential blockchain state information including block hash, merkle root, difficulty, and timestamp data.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable settings:

- **`Network`**: Blockchain network to connect to (default: `NetworkMainnet`)
- **`BHS.URL`**: Block Headers Service endpoint URL (default: `"http://localhost:8080"`)
- **`BHS.APIKey`**: API key for Block Headers Service authentication (default: `"..."` - use DefaultAppToken)

### Service Setup

The `FindChainTipHeader` method requires:

- **`Context`**: Request context for lifecycle management
- **`Services Instance`**: Configured services with BHS connection settings
- **`Network Configuration`**: Mainnet settings for accessing production blockchain data

### Response Analysis

The service response contains:

- **`Block Header`**: Complete header structure with all blockchain metadata fields
- **`Height`**: Current block height on the longest chain
- **`Hash`**: Unique block identifier
- **`Merkle Root`**: Root hash of all transactions in the block
- **`Timestamp`**: When the block was mined
- **`Difficulty Data`**: Target bits and nonce values

## Running the Example

To run this example:

```bash
go run ./examples/services_examples/get_chain_tip_header/get_chain_tip_header.go
```

## Expected Output

```text
🚀 STARTING: Find Chain Tip Header
============================================================

=== STEP ===
FindChainTipHeader is performing: Finds the latest block header in the longest chain
--------------------------------------------------
✅ SUCCESS: Fetched chain tip header
Chain Tip Header:
Height  Hash                                                              Version   Prev-Hash                                                         Merkle-Root                                                       Time        Bits      Nonce
------  ----------------------------------------------------------------  --------  ---------------------------------------------------------------- ----------------------------------------------------------------  ----------  --------  ---------
905604  000000000000000005698beb20b1d7ff4ad1860314bd3c395c6db123f91c7ffd  283e2000  00000000000000000e9ee9c173a140cdc20e7f9f9f708ee276a9922c4fd6dea3  5ab8bf3278ab9d2912ade1260cacd5df9ee0b78670bbc87b9fb05a7ea5755b90  1752570909  1817a94f  342927395
============================================================
🎉 COMPLETED: Find Chain Tip Header
```

## Integration Steps

To integrate chain tip header retrieval into your application:

1. **Configure services** with appropriate network settings and BHS API credentials.
2. **Create services instance** with logging and your configuration.
3. **Submit header request** using `FindChainTipHeader()` with context for request management.
4. **Process response data** to extract block header information and metadata.
5. **Handle blockchain data** including height, hash, merkle root, and timestamp fields.
6. **Implement caching logic** for header data to reduce API calls when appropriate.
7. **Add monitoring** for blockchain state changes and new block detection.

## Additional Resources

- [Get Current Block Height Documentation](../get_current_block_height/get_current_block_height.md) - Get just the block height
- [Get Chain Tip Header Example](./get_chain_tip_header.go) - Complete code example for getting block header information
