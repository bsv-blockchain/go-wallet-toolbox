# Post BEEF with Multiple Chained Transactions

This example demonstrates how to construct and broadcast a chain of three dependent transactions (grandparent, parent, and child) using [BEEF](../../README.md#beef-background-evaluation-extended-format) format through multiple wallet services with automatic fallback.

## What is Transaction Chaining?

Transaction chaining occurs when one transaction spends outputs from another transaction, creating a dependency chain. This example creates:

1. **Grandparent Transaction**: Spends from the source BEEF transaction
2. **Parent Transaction**: Spends from the grandparent transaction (output index 0)
3. **Child Transaction**: Spends from the parent transaction (output index 0)

All three transactions are bundled together in a single BEEF and broadcast simultaneously.

## Process Overview

The example follows these steps:

1. **Load Source Transaction**: Decode the BEEF-encoded source transaction
2. **Create Grandparent**: Build a transaction that spends from the source transaction
3. **Create Parent**: Build a transaction that spends from the grandparent transaction  
4. **Create Child**: Build a transaction that spends from the parent transaction
5. **Generate BEEF**: Convert the entire transaction chain to BEEF format
6. **Broadcast**: Submit the BEEF containing all three transactions for network propagation
7. **Display Results**: Show detailed responses for each transaction from each service

## Configuration Parameters

The example uses the following configurable constants:

- **`sourceTxBEEF`**: The BEEF-encoded source transaction hex (replace with your own)
- **`wif`**: The private key in Wallet Import Format for unlocking transaction outputs
- **`sourceOutputIndex`**: The output index from the source transaction to spend (typically `0`)
- **`network`**: The BSV network to use (`defs.NetworkTestnet` or `defs.NetworkMainnet`)

## Response Fields

Each service returns detailed results for each transaction in the chain:

- **Service**: Name of the service that processed the request
- **Success**: Whether the broadcast was successful
- **TX ID**: The transaction identifier for each transaction
- **Result**: Status of each transaction (`success`, `error`, etc.)
- **AlreadyKnown**: Whether each transaction was already known to the service
- **DoubleSpend**: Information about any double-spend conflicts
- **BlockHash**: Hash of the block containing each transaction (if mined)
- **BlockHeight**: Height of the block containing each transaction (if mined)
- **MerklePath**: Merkle path proof for each transaction
- **CompetingTxs**: List of competing transactions (in case of conflicts)
- **Error**: Detailed error information (if broadcast failed)

## Example Output

```text
===========================================================
Service ARC PostBEEF result:
	Success:
	TX ID: 1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef
		Result:	 success
		AlreadyKnown:	 false
		DoubleSpend:	 false
		BlockHash:	 
		BlockHeight:	 0
		MerklePath:	 
		CompetingTxs:	 []
		Notes:	 
		Data:	 
	TX ID: 2345678901bcdefg2345678901bcdefg2345678901bcdefg2345678901bcdefg
		Result:	 success
		AlreadyKnown:	 false
		DoubleSpend:	 false
		BlockHash:	 
		BlockHeight:	 0
		MerklePath:	 
		CompetingTxs:	 []
		Notes:	 
		Data:	 
	TX ID: 3456789012cdefgh3456789012cdefgh3456789012cdefgh3456789012cdefgh
		Result:	 success
		AlreadyKnown:	 false
		DoubleSpend:	 false
		BlockHash:	 
		BlockHeight:	 0
		MerklePath:	 
		CompetingTxs:	 []
		Notes:	 
		Data:	 
```
