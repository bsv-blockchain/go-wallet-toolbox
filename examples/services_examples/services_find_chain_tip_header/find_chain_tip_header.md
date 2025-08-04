# Find Chain Tip Header

This example demonstrates how to find the latest chain tip (block header) on the longest chain on the BSV network.

## Response Fields

The following fields will be included in the response:

- **Height**: The latest block height on the network that has been mined
- **Hash**: The hash of all transactions in the block
- **Version**: The version of the Bitcoin node software that created this block
- **Prev-Hash**: The hash of all transactions in the previous block
- **Merkle-Root**: The root hash of the merkle tree constructed from all transactions in the block
- **Time**: The UTC timestamp when the block was mined
- **Bits**: The target difficulty threshold for this block (in compact format)
- **Nonce**: A random number that miners increment to find a valid block hash

## Example Output

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
