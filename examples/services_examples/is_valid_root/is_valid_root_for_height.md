# Validate Merkle Root Example

This example demonstrates how to verify if a given Merkle root is valid for a specific block height on the BSV network. This validation is essential for SPV (Simplified Payment Verification) implementations to verify transaction inclusion in blocks.

## Overview

The process involves several steps:
1. Configuring the services stack with network settings and API credentials.
2. Defining the block height and Merkle root to validate.
3. Converting the hex-encoded Merkle root to a proper hash format.
4. Calling `IsValidRootForHeight()` to perform the validation against blockchain data.
5. Processing the boolean result indicating whether the Merkle root matches the specified block.

This validation confirms that a provided Merkle root corresponds to the actual transactions included in the block at the specified height.

## Code Walkthrough

### Defining Validation Parameters

```go
const (
    height = uint32(903321)
    // https://whatsonchain.com/block-height/903321?tab=json
    rootHex = "559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd"
)
```
First, we define the specific block height and Merkle root to validate. The example uses real data from block 903321 on the BSV mainnet, with the root hash obtained from blockchain explorers.

### Configuring Services

```go
cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
cfg.BHS.APIKey = "..." // API key for Block Headers Service
srv := services.New(slog.Default(), cfg)
```
We configure the services stack for mainnet with API credentials. This provides access to blockchain data services needed for validation.

### Converting Hash Format

```go
root, err := chainhash.NewHashFromHex(rootHex)
```
The hex-encoded Merkle root string is converted to a proper `chainhash.Hash` format required by the validation method.

### Performing Validation

```go
ok, err := srv.IsValidRootForHeight(context.Background(), root, height)
```
The `IsValidRootForHeight()` method performs the core validation by:
- Retrieving the actual block header for the specified height
- Comparing the provided Merkle root with the block's actual Merkle root
- Returning a boolean indicating whether they match

### Processing Results

```go
show.IsValidRootForHeightOutput(height, rootHex, ok)
```
The validation result is displayed, showing the height, root hash, and whether the validation was successful.

## Running the Example

To run this example:

```bash
go run ./examples/services_examples/is_valid_root/is_valid_root_for_height.go
```

Expected output:

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

1. **Configure services** with appropriate network settings and API credentials.
2. **Prepare validation data** including the block height and Merkle root to verify.
3. **Convert hex strings** to proper hash format using `chainhash.NewHashFromHex()`.
4. **Call IsValidRootForHeight()** with context, hash, and height parameters.
5. **Handle the response** which returns a boolean validation result.
6. **Implement error handling** for malformed hashes, network issues, or service failures.
7. **Consider caching** validation results for frequently checked block/root combinations.

### Use Cases

Merkle root validation is commonly used for:

- **SPV Wallet Verification**: Confirming transaction inclusion without downloading full blocks
- **Payment Verification**: Validating that received payments are included in confirmed blocks  
- **Transaction Proof Validation**: Verifying Merkle proofs provided by third parties
- **Blockchain Data Integrity**: Ensuring consistency between different data sources

### Error Handling

```go
ok, err := srv.IsValidRootForHeight(ctx, root, height)
if err != nil {
    // Handle validation failure - network issues, invalid height, etc.
    log.Printf("Failed to validate root for height: %v", err)
    return err
}

if !ok {
    // Merkle root does not match the block at this height
    return fmt.Errorf("invalid merkle root for block %d", height)
}
```

## Additional Resources

- [Find Chain Tip Header Documentation](../find_chain_tip_header/find_chain_tip_header.md) - Get complete block header data
- [SPV Documentation](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0067.md) - BRC-67 SPV specification
