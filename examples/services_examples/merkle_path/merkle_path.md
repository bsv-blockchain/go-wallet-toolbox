# Get Merkle Path for Transaction

This example demonstrates how to retrieve the merkle path for a specific transaction ID on the BSV network. A merkle path provides cryptographic proof that a transaction was included in a specific block.

## What is a Merkle Path?

A merkle path is a sequence of hash nodes that allows you to cryptographically prove a transaction's inclusion in a block without downloading the entire block. It consists of the sibling hashes needed to reconstruct the merkle root from a specific transaction.

## Service Fallback Strategy

The wallet-services stack uses the following fallback approach:

1. **Primary**: ARC service - First attempts to fetch the merkle path from ARC
2. **Secondary**: WhatsOnChain - Falls back to WoC if ARC doesn't have the transaction
3. **Result**: Returns the first successful merkle path obtained

## Parameters

- **Transaction ID**: The hexadecimal transaction ID to get the merkle path for (e.g., `9ca4300a599b48638073cb35f833475a8c6cfca0d4bbe6dd7244d174e7a0e7f6`)

## Response Fields

The response includes block information and the merkle path:

- **Service**: Which service provided the merkle path (e.g., "WhatsOnChain")
- **Block Hash**: The hash of the block containing the transaction
- **Block Height**: The height of the block containing the transaction
- **Merkle Root**: The merkle root of the block
- **Path Nodes**: Array of merkle path nodes with the following format:
  - **Depth**: The level in the merkle tree (0 is leaf level)
  - **Offset**: The position at that depth level
  - **Hash**: The hash value of the node
  - **Duplicate**: Whether this node is a duplicate (for odd number of transactions)

## Example Output

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
