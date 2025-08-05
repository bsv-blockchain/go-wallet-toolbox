# Get Block Header from Hash

This example demonstrates how to retrieve a complete block header using a specific block hash on the BSV blockchain using the Go Wallet Toolbox SDK. It showcases fetching detailed block metadata when you have the block hash but need comprehensive header information.

## Overview

The process involves several steps:
1. Setting up services configuration with network settings for blockchain data access.
2. Defining the specific block hash to retrieve header information for analysis purposes.
3. Calling `HashToHeader()` which queries blockchain data services for complete header data.
4. Processing the returned block header data including height, version, and metadata information.
5. Displaying comprehensive block information with all header fields and blockchain context.

This approach enables block analysis and verification when starting with a known block hash to retrieve complete metadata.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable settings:

- **`Block Hash`**: Specific block hash to retrieve header for (default: `"000000000000000004a288072ebb35e37233f419918f9783d499979cb6ac33eb"`)
- **`Network`**: Blockchain network to query (default: `NetworkMainnet`)
- **`Services Config`**: Default configuration with automatic fallback across multiple blockchain data providers

### Service Setup

The `HashToHeader` method requires:

- **`Context`**: Request context for lifecycle management
- **`Block Hash`**: Hexadecimal block hash identifier for header retrieval
- **`Services Instance`**: Configured services with fallback logic across multiple blockchain providers

### Response Analysis

The service response contains:

- **`Block Height`**: The height of the block in the blockchain for reference
- **`Block Hash`**: The queried block hash for verification and confirmation
- **`Version`**: Bitcoin protocol version used for this block
- **`Previous Hash`**: Hash of the previous block linking to the blockchain
- **`Merkle Root`**: Root hash of all transactions included in this block
- **`Timestamp`**: When the block was mined (Unix timestamp)
- **`Bits`**: Difficulty target in compact format
- **`Nonce`**: Proof-of-work solution found by miners

## Running the Example

To run this example:

```bash
go run ./examples/services_examples/get_block_header_from_block_hash/get_block_header_from_block_hash.go
```

## Expected Output

```text
🚀 STARTING: Hash To Header
============================================================

=== STEP ===
Wallet-Services is performing: fetching block header for hash 000000000000000004a288072ebb35e37233f419918f9783d499979cb6ac33eb
--------------------------------------------------
✅ SUCCESS: Fetched block header from hash
Chain Tip Header:
Height  Hash                                                              Version   Prev-Hash                                                         Merkle-Root                                                       Time        Bits      Nonce
------  ----------------------------------------------------------------  --------  ----------------------------------------------------------------  ----------------------------------------------------------------  ----------  --------  --------
575045  000000000000000004a288072ebb35e37233f419918f9783d499979cb6ac33eb  2000e000  00000000000000000988156c7075dc9147a5b62922f1310862e8b9000d46dd9b  4ebcba09addd720991d03473f39dce4b9a72cc164e505cd446687a54df9b1585  1553416668  180997ee  87914848
============================================================
🎉 COMPLETED: Hash To Header
```

## Integration Steps

To integrate block header retrieval from hash into your application:

1. **Configure services** with appropriate network settings for your target blockchain environment.
2. **Prepare block hash** in hexadecimal format for the block requiring header information.
3. **Submit header request** using `HashToHeader()` with context and block hash parameters.
4. **Process response data** to extract complete block header information and metadata.
5. **Handle header fields** including height, version, previous hash, merkle root, timestamp, and difficulty data.
6. **Implement validation logic** for block header verification or blockchain analysis as needed.
7. **Add caching strategies** for frequently accessed block headers to improve performance.

## Additional Resources

- [Get Block Header from Block Hash Example](./get_block_header_from_block_hash.go) - Complete code example for getting block headers from hash
- [Get Chain Tip Header Documentation](../get_chain_tip_header/get_chain_tip_header.md) - Get the latest block header
- [Get Current Block Height Documentation](../get_current_block_height/get_current_block_height.md) - Get current blockchain height
