# Merkle Path Example

This example demonstrates how to retrieve the Merkle path for a specific transaction ID on the BSV network. A Merkle path provides cryptographic proof that a transaction was included in a specific block without requiring the full block data.

## Overview

The process involves several steps:
1. Configuring the services stack with network settings.
2. Defining the transaction ID to retrieve the Merkle path for.
3. Calling `MerklePath()` which attempts multiple blockchain data services with fallback logic.
4. Processing the returned Merkle path data including block information and path nodes.
5. Using the path data for SPV verification or transaction proof validation.

A Merkle path consists of sibling hashes needed to reconstruct the Merkle root from a specific transaction, enabling cryptographic verification of transaction inclusion.

## Code Walkthrough

### Defining Transaction Parameters

```go
txID := "9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6"
network := defs.NetworkMainnet
```
First, we define the specific transaction ID to lookup and the network (mainnet). The example uses a real transaction from block 903321.

### Configuring Services

```go
serviceCfg := defs.DefaultServicesConfig(network)
walletServices := services.New(slog.Default(), serviceCfg)
```
We configure the services stack with default settings for the specified network. This provides access to multiple blockchain data providers with automatic fallback.

### Retrieving Merkle Path

```go
result, err := walletServices.MerklePath(context.Background(), txID)
```
The `MerklePath()` method attempts to fetch the Merkle path from multiple services, returning the first successful result. This includes both block metadata and the actual path nodes.

### Processing Results

```go
show.MerklePathOutput(result)
```
The returned result contains comprehensive Merkle path information displayed in a structured format showing the service used, block details, and path nodes.

## Running the Example

To run this example:

```bash
go run ./examples/services_examples/merkle_path/merkle_path.go
```

Expected output:

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

1. **Configure services** with appropriate network settings for your target blockchain.
2. **Prepare transaction ID** in hexadecimal format for the transaction to verify.
3. **Call MerklePath()** with context and transaction ID parameters.
4. **Process the response** which contains block metadata and path node arrays.
5. **Extract required data** based on your verification needs (block height, path nodes, etc.).
6. **Implement error handling** for cases where the transaction is not found or services fail.
7. **Use path data** for SPV verification or proof generation in your application.

### Response Structure

The Merkle path response includes:

- **Service**: Which blockchain data service provided the path
- **Block Hash**: Hash of the block containing the transaction
- **Block Height**: Height of the block in the blockchain
- **Merkle Root**: Root hash of the block's Merkle tree
- **Path Nodes**: Array of nodes with depth, offset, hash, and duplicate flag

### Path Node Format

Each path node contains:
- **Depth**: Level in the Merkle tree (0 = leaf level)
- **Offset**: Position at that depth level  
- **Hash**: Hash value of the node
- **Duplicate**: Whether this node is duplicated (handles odd transaction counts)

### Use Cases

Merkle paths are commonly used for:

- **SPV Wallet Verification**: Proving transaction inclusion without full blocks
- **Payment Proof Generation**: Creating compact proofs for received payments
- **Transaction Auditing**: Verifying historical transactions against blockchain data
- **Lightweight Client Operations**: Enabling verification with minimal data downloads

### Error Handling

```go
result, err := walletServices.MerklePath(ctx, txID)
if err != nil {
    // Handle failure - transaction not found, network issues, etc.
    log.Printf("Failed to get Merkle path: %v", err)
    return err
}

// Verify we have valid path data
if len(result.MerklePath) == 0 {
    return fmt.Errorf("no Merkle path data received for transaction %s", txID)
}
```

## Additional Resources

- [Merkle Path Example](./merkle_path.go) - Get the Merkle path for a specific transaction ID
- [SPV Documentation](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0067.md) - BRC-67 SPV specification
- [Validate Merkle Root Example](../is_valid_root/is_valid_root_for_height.md) - Verify Merkle roots against block heights
