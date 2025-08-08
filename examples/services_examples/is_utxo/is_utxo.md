# Is UTXO

This example demonstrates how to check whether a specific outpoint (transaction output) is an unspent transaction output (UTXO) on the BSV blockchain using the Go Wallet Toolbox SDK. It showcases verifying the spending status of a transaction output by querying blockchain service providers.

## Overview

The process involves several steps:
1. Setting up services configuration with network settings for blockchain data access.
2. Defining the script hash and transaction outpoint (txid + index) to verify.
3. Creating an outpoint structure with transaction ID and output index.
4. Calling `IsUtxo()` which queries blockchain data services to check spending status.
5. Processing the returned boolean result indicating whether the output is unspent.

This approach enables efficient UTXO verification with automatic service redundancy across multiple blockchain data providers.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable settings:

- **`Script Hash`**: Script hash associated with the output to verify (default: `"b3005d46af31c4b5675b73c17579b7bd366dfe10635b7b43ac111aea5226efb6"`)
- **`Transaction ID`**: Hexadecimal transaction identifier (default: `"ab0f76f957662335f98ee430a665f924c28310ec5126c2aede56086f9233326f"`)
- **`Output Index`**: Index of the output within the transaction (default: `1`)
- **`Network`**: Blockchain network to query (default: `NetworkMainnet`)
- **`Services Config`**: Default configuration with automatic fallback across multiple blockchain data providers

### Service Setup

The `IsUtxo` method requires:

- **`Context`**: Request context for lifecycle management
- **`Script Hash`**: Hexadecimal script hash identifier for the output being verified
- **`Outpoint`**: Transaction outpoint structure containing txid and output index
- **`Services Instance`**: Configured services with fallback logic across WhatsOnChain and other providers

### Response Analysis

The service response contains:

- **`Boolean Result`**: Simple true/false indicating whether the outpoint is a UTXO
- **`UTXO Status`**: True means the output is unspent and available for spending
- **`Spent Status`**: False means the output has already been spent in another transaction
- **`Service Verification`**: Automatic verification across multiple blockchain data sources

## Running the Example

To run this example:

```bash
go run ./examples/services_examples/is_utxo/is_utxo.go
```

## Expected Output

```text
🚀 STARTING: Is UTXO
============================================================

=== STEP ===
Wallet-Services is performing: checking if outpoint is a UTXO
--------------------------------------------------

WALLET CALL: IsUtxo
Args: map[index:1 scriptHash:b3005d46af31c4b5675b73c17579b7bd366dfe10635b7b43ac111aea5226efb6 txid:ab0f76f957662335f98ee430a665f924c28310ec5126c2aede56086f9233326f]
✅ Result: true
============================================================
🎉 COMPLETED: Is UTXO
```

## Integration Steps

To integrate UTXO verification into your application:

1. **Configure services** with appropriate network settings for your target blockchain environment.
2. **Prepare outpoint data** including transaction ID in hexadecimal format and output index.
3. **Create outpoint structure** using the `transaction.Outpoint` type with txid and index fields.
4. **Submit UTXO request** using `IsUtxo()` with context, script hash, and outpoint parameters.
5. **Process boolean response** to determine whether the output is available for spending.
6. **Handle verification results** by implementing appropriate logic for spent vs unspent outputs.
7. **Add error handling** for invalid transaction IDs, network issues, or service failures.
8. **Implement caching logic** for UTXO status to reduce API calls when appropriate.

## Additional Resources

- [Is UTXO Example](./is_utxo.go) - Complete code example for checking UTXO status
- [Get UTXO Status Documentation](../get_utxo_status/get_utxo_status.go) - Get detailed UTXO information
- [Get Script Hash History Documentation](../get_script_hash_history/get_script_hash_history.md) - Get transaction history for script hashes
