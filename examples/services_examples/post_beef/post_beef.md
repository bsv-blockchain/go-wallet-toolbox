# Post BEEF Transaction

This example demonstrates how to broadcast a BSV transaction using the [BEEF (Background Evaluation Extended Format)](../../README.md#beef-background-evaluation-extended-format) through multiple wallet services with automatic fallback.

## Process Overview

The example follows these steps:

1. **Load Source Transaction**: Decodes a BEEF-encoded source transaction
2. **Create New Transaction**: Builds a new transaction that spends from the source transaction
3. **Generate BEEF**: Converts the new transaction to BEEF format
4. **Broadcast**: Submits the BEEF to multiple services for network propagation
5. **Display Results**: Shows detailed responses from each service

## Configuration Parameters

The example uses the following configurable constants:

- **`sourceTxBEEF`**: The BEEF-encoded source transaction hex (replace with your own)
- **`wif`**: The private key in Wallet Import Format for unlocking the source transaction output
- **`sourceOutputIndex`**: The output index from the source transaction to spend (typically `0`)
- **`network`**: The BSV network to use (`defs.NetworkTestnet` or `defs.NetworkMainnet`)

## Service Broadcasting

The wallet services automatically handle broadcasting with built-in fallback strategies across multiple services:

- **ARC**: Primary broadcasting service
- **WhatsOnChain**: Secondary fallback service  
- **Bitails**: Additional fallback option

## Response Fields

Each service returns detailed results including:

- **Service**: Name of the service that processed the request
- **Success**: Whether the broadcast was successful
- **TX ID**: The transaction identifier
- **Result**: Status of the transaction (`success`, `error`, etc.)
- **AlreadyKnown**: Whether the transaction was already known to the service
- **DoubleSpend**: Information about any double-spend conflicts
- **BlockHash**: Hash of the block containing the transaction (if mined)
- **BlockHeight**: Height of the block containing the transaction (if mined)
- **MerklePath**: Merkle path proof for the transaction
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
===========================================================
Service WhatsOnChain PostBEEF result:
	Success:
		TX ID: 1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef
		Result:	 success
		AlreadyKnown:	 true
		DoubleSpend:	 false
		BlockHash:	 
		BlockHeight:	 0
		MerklePath:	 
		CompetingTxs:	 []
		Notes:	 
		Data:	 
```
