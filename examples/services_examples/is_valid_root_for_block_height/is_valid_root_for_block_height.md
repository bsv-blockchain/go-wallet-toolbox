# Validate Merkle Root for Height

This example demonstrates how to verify if a given Merkle root is valid for a specific block height on the BSV blockchain using the Go Wallet Toolbox SDK. It showcases essential validation for SPV (Simplified Payment Verification) implementations to verify transaction inclusion in blocks.

## Overview

The process involves several steps:
1. Setting up services configuration with network settings and API credentials for blockchain data access.
2. Defining the block height and Merkle root to validate against blockchain records.
3. Converting the hex-encoded Merkle root to proper hash format for validation processing.
4. Calling `IsValidRootForHeight()` to perform validation against blockchain data services.
5. Processing the boolean result indicating whether the Merkle root matches the specified block.

This approach confirms that a provided Merkle root corresponds to the actual transactions included in the block at the specified height.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable settings:

- **`Block Height`**: Specific block height to validate Merkle root against (default: `903321`)
- **`Merkle Root Hex`**: Hex-encoded Merkle root to validate (default: `"559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd"`)
- **`Network`**: Blockchain network for validation (default: `NetworkMainnet`)
- **`BHS API Key`**: Block Headers Service authentication credentials for accessing blockchain data

### Service Setup

The `IsValidRootForHeight` method requires:

- **`Context`**: Request context for lifecycle management
- **`Hash Object`**: Converted Merkle root from hex string to chainhash.Hash format
- **`Block Height`**: Specific block number for Merkle root validation
- **`Services Instance`**: Configured services with BHS access for blockchain data retrieval

### Response Analysis

The service response contains:

- **`Boolean Result`**: Simple true/false validation result indicating Merkle root match
- **`Block Height`**: The validated block height for confirmation
- **`Merkle Root`**: The validated Merkle root hash for verification
- **`Validation Status`**: Success or failure of the validation process

## Running the Example

To run this example:

```bash
go run ./examples/services_examples/is_valid_root_for_block_height/is_valid_root_for_block_height.go
```

## Expected Output

```text
🚀 STARTING: Is Valid Root For Height
============================================================

=== STEP ===
Wallet-Services is performing: checking if root 559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd is valid for height 903321
--------------------------------------------------
✅ SUCCESS: Checked if root is valid for height

Height: 903321 | Merkle Root: 559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd | Valid: true
============================================================
🎉 COMPLETED: Is Valid Root For Height
```

**Note**: The example uses real blockchain data, so the validation should return `true` when services are properly configured.

## Integration Steps

To integrate Merkle root validation into your application:

1. **Configure services** with appropriate network settings and API credentials for blockchain access.
2. **Prepare validation data** including the specific block height and Merkle root requiring verification.
3. **Convert hex strings** to proper hash format using `chainhash.NewHashFromHex()` for processing.
4. **Submit validation request** using `IsValidRootForHeight()` with context, hash, and height parameters.
5. **Process validation results** to handle boolean response indicating Merkle root validity.
6. **Implement validation logic** for SPV operations, payment verification, or blockchain data integrity checks.
7. **Add caching strategies** for validation results of frequently checked block/root combinations.

## Additional Resources

- [Validate Merkle Root for Block Height Example](./is_valid_root_for_block_height.go) - Complete code example for Merkle root validation
- [Get Chain Tip Header Documentation](../get_chain_tip_header/get_chain_tip_header.md) - Get complete block header data
- [SPV Documentation](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0067.md) - BRC-67 SPV specification
