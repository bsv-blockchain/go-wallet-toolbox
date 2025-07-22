# Validate Merkle Root for Block Height

This example demonstrates how to verify if a given merkle root is valid for a specific block height on the BSV network.

## What is Merkle Root Validation?

A merkle root is the top hash of a merkle tree constructed from all transactions in a block. This validation confirms that a provided merkle root actually corresponds to the transactions that were included in the block at the specified height.

## Parameters

The validation requires two inputs:

- **Height**: The specific block height to validate against (e.g., `903321`)
- **Merkle Root**: The hexadecimal hash of the merkle root to validate (e.g., `559ce1f8394df2f008a9c4d23e71256c999ea05aba47e8620ab66f1f24c8a0fd`)

## Response

The response is a boolean value indicating the validation result:

- **Valid**: `true` if the merkle root matches the actual merkle root for the specified block height
- **Invalid**: `false` if the merkle root does not match or the block height is invalid

## Example Output

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
