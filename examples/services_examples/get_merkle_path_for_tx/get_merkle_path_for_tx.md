# Get Merkle Path for Transaction

This example demonstrates how to retrieve the Merkle path for a specific transaction ID on the BSV blockchain using the Go Wallet Toolbox SDK. It showcases cryptographic proof generation that verifies transaction inclusion in a block without requiring full block data.

## Overview

The process involves several steps:
1. Setting up services configuration with network settings for blockchain data access.
2. Defining the transaction ID to retrieve the Merkle path for verification purposes.
3. Calling `MerklePath()` which attempts multiple blockchain data services with fallback logic.
4. Processing the returned Merkle path data including block information and path nodes.
5. Using the path data for SPV verification or transaction proof validation.

This approach enables cryptographic verification of transaction inclusion through Merkle tree proof generation with automatic service redundancy.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable settings:

- **`Transaction ID`**: Specific transaction to retrieve Merkle path for (default: `"9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6"`)
- **`Network`**: Blockchain network to query (default: `NetworkMainnet`)
- **`Services Config`**: Default configuration with automatic fallback across multiple blockchain data providers

### Service Setup

The `MerklePath` method requires:

- **`Context`**: Request context for lifecycle management
- **`Transaction ID`**: Hexadecimal transaction identifier for path generation
- **`Services Instance`**: Configured services with fallback logic across ARC, WhatsOnChain, and other providers

### Response Analysis

The service response contains:

- **`Service Name`**: Which blockchain data service provided the successful path response
- **`Block Hash`**: Hash of the block containing the target transaction
- **`Block Height`**: Height of the block in the blockchain for verification
- **`Merkle Root`**: Root hash of the block's Merkle tree structure
- **`Path Nodes`**: Array of sibling hashes with depth, offset, hash, and duplicate flag information

## Running the Example

To run this example:

```bash
go run ./examples/services_examples/get_merkle_path_for_tx/get_merkle_path_for_tx.go
```

## Expected Output

```text
🚀 STARTING: Merkle Path
============================================================

=== STEP ===
Wallet-Services is performing: fetching Merkle Path for txID 9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6
--------------------------------------------------
2025/07/14 11:41:02 WARN error when calling service service=services.MerklePath service.name=ARC error="tx 9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6 not found"
✅ SUCCESS: Fetched Merkle Path
service,WhatsOnChain
block_hash,000000000000000004f576c9cdc2b0ee65f04c3f03c08529c380d6a76d262641
block_height,903321
merkle_root,559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd

0,0,9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6,true
0,1,7614658ca0007fa36b4634a53ae3d4be5207414cccd2a418578b77df5ecce63b,false
1,1,1580364a629685228cb2527893da2553e93a0c8963d9993f76daf1a0d9becd36,false
2,1,f45a57b6c15a3ca2aa849fa85e224c75a9d9fcc3dffb783ec6445b872079d00f,false
3,1,a18f3c6fc6fd079a7a8a89a71ad134138418e2e1e8d42654eb7d4b788b47d800,false
4,1,44f1abc430ea7717f86ca084fd4a5cb20d71d9cb66e2395ec88b5d7bc58f441f,false
5,1,e8298fc5360ecfe64f22d2442097afcc6307b02d8b718d5588c8b2b07111407b,false
6,1,e27a8ad3d36d00ad37de836dde518fcfcba6c3067f6a5c227a37cddac877fec0,false
7,1,56b45af75b2f3d53f80baa93b7ec249b734c5655092805c0fe1d8933d36d517c,false
8,1,4cf9c5fffb8ee4f2d6c68786059bc54a980f050f99da9f627e21c82f2f1787c6,false
9,1,2d321206df2b0faea962902329fdd0a519e1d154925714bd284dc80c97b32cbd,false
10,1,3a27e54bf59f2612512519ce7d6315da551e4572d948fc8c9c5d0058ccfca608,false
11,1,53bb438fa84b1d17289d5bd5ce696350dc5a3887ab4011ea28dea8eecf1b137e,false
============================================================
🎉 COMPLETED: Merkle Path
```

**Note**: Warning messages about individual service failures are normal and demonstrate the automatic fallback mechanism working as designed.

## Integration Steps

To integrate Merkle path retrieval into your application:

1. **Configure services** with appropriate network settings for your target blockchain environment.
2. **Prepare transaction ID** in hexadecimal format for the transaction requiring proof verification.
3. **Submit path request** using `MerklePath()` with context and transaction ID parameters.
4. **Process response data** to extract block metadata and path node arrays for verification.
5. **Handle path nodes** with depth, offset, hash, and duplicate flag information for SPV operations.
6. **Implement verification logic** for SPV validation or transaction proof generation as needed.
7. **Add monitoring** for service fallback patterns and successful path retrieval across providers.

## Additional Resources

- [Get Merkle Path for Transaction Example](./get_merkle_path_for_tx.go) - Complete code example for getting Merkle paths
- [SPV Documentation](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0067.md) - BRC-67 SPV specification
- [Is Valid Root for Block Height](../is_valid_root_for_block_height/is_valid_root_for_block_height.md) - Verify Merkle roots against block heights
