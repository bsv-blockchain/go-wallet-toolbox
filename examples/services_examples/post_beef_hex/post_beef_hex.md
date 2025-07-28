# Post BEEF Transaction from Hex

This example demonstrates how to broadcast a BSV transaction from an existing [BEEF](../../README.md#beef-background-evaluation-extended-format) hex string through multiple wallet services with automatic fallback.

## Difference from Post BEEF Example

Unlike the `post_beef.go` example which creates a new transaction from scratch, this example:

- **Takes existing BEEF**: Uses a pre-encoded BEEF hex string containing the transaction
- **Direct broadcasting**: Focuses purely on the broadcasting mechanism

## Process Overview

The example follows these simplified steps:

1. **Decode BEEF Hex**: Converts the hex string to binary BEEF format
2. **Parse BEEF**: Creates a BEEF object from the binary data
3. **Broadcast**: Submits the BEEF to multiple services for network propagation
4. **Display Results**: Shows detailed responses from each service

## Configuration Parameters

The example uses the following configurable constants:

- **`transactionID`**: The transaction ID of the transaction to be broadcast (must match the transaction in the BEEF)
- **`beefHex`**: The complete BEEF hex string containing the transaction and its dependencies
- **`network`**: The BSV network to use (`defs.NetworkTestnet` or `defs.NetworkMainnet`)

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
		TX ID: c7218bcddee6e7a2ad097007d50831837bb174ad78c078f65260d7971a46d620
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
		TX ID: c7218bcddee6e7a2ad097007d50831837bb174ad78c078f65260d7971a46d620
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
