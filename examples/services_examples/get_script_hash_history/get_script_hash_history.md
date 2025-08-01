# Get Script Hash History

This example demonstrates how to retrieve the transaction history for a specific script hash on the BSV blockchain using the Go Wallet Toolbox SDK. It showcases fetching comprehensive transaction data associated with a script hash from blockchain service providers.

## Overview

The process involves several steps:
1. Setting up services configuration with network settings for blockchain data access.
2. Defining the script hash to retrieve transaction history for analysis purposes.
3. Calling `GetScriptHashHistory()` which queries blockchain data services for transaction records.
4. Processing the returned transaction history data including status and block information.
5. Displaying comprehensive transaction records with confirmation status and block heights.

This approach enables comprehensive transaction tracking and analysis for specific script hashes with automatic service redundancy.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable settings:

- **`Script Hash`**: Specific script hash to retrieve transaction history for (default: `"c79e8d823c1ce9b80c9c340a389409f489989800044466c9d05bfef12c472232"`)
- **`Network`**: Blockchain network to query (default: `NetworkMainnet`)
- **`Services Config`**: Default configuration with automatic fallback across multiple blockchain data providers

### Service Setup

The `GetScriptHashHistory` method requires:

- **`Context`**: Request context for lifecycle management
- **`Script Hash`**: Hexadecimal script hash identifier for transaction history retrieval
- **`Services Instance`**: Configured services with fallback logic across WhatsOnChain and other providers

### Response Analysis

The service response contains:

- **`Service Name`**: Which blockchain data service provided the successful transaction history response
- **`Script Hash`**: The queried script hash for verification and confirmation
- **`Transaction History`**: Array of transaction records with hash, status, and block height information
- **`Status Information`**: Confirmation status (Confirmed/Unconfirmed) for each transaction record
- **`Block Heights`**: Block numbers for confirmed transactions, empty for unconfirmed transactions

## Running the Example

To run this example:

```bash
go run ./examples/services_examples/get_script_hash_history/get_script_hash_history.go
```

## Expected Output

```text
🚀 STARTING: Script Hash History
============================================================

=== STEP ===
Wallet-Services is performing: fetching script history for scripthash c79e8d823c1ce9b80c9c340a389409f489989800044466c9d05bfef12c472232
--------------------------------------------------
✅ SUCCESS: Fetched Script Hash History

============================================================
SCRIPT HASH HISTORY
============================================================
Service: WhatsOnChain
ScriptHash: c79e8d823c1ce9b80c9c340a389409f489989800044466c9d05bfef12c472232
Transaction History:
TxHash                                                            Status       Block Height
----------------------------------------------------------------  -----------  ------------
bfe594bc56b11e8f1030e7f6fc53fdc6e58ae05d75838252ab7f7d5f75a09e56  Confirmed    906308
b79f46413372002a2bc01e1c13ab90c896cae993b68cbbab801998f07f834e62  Confirmed    906310
4bf03b27d8fd0e9f7a4f989cbd0834974b2aa00709475246b06d9bd567729f93  Confirmed    906310
4e36756ecf452864560c00b5e60aa8ddf2f1b8a08645cc044b485ab4d234d37a  Confirmed    906311
...
44256b5a821f83ebf70d578c310a536eb8d51f5d6af68560f36befdd6df0f9e3  Unconfirmed  -
d0a55d2f91f884ae7ff313168003dd197b3cae3db68280c002ad3bd9de75d0ab  Unconfirmed  -
============================================================
🎉 COMPLETED: Script Hash History
```

## Integration Steps

To integrate script hash history retrieval into your application:

1. **Configure services** with appropriate network settings for your target blockchain environment.
2. **Prepare script hash** in hexadecimal format for the script requiring transaction history analysis.
3. **Submit history request** using `GetScriptHashHistory()` with context and script hash parameters.
4. **Process response data** to extract transaction records with status and block height information.
5. **Handle transaction records** by parsing confirmed and unconfirmed transaction data for analysis.
6. **Implement filtering logic** for transaction status, block heights, or specific time periods as needed.
7. **Add monitoring** for script hash activity and new transaction detection across service providers.

## Additional Resources

- [Get Script Hash History Example](./get_script_hash_history.go) - Complete code example for getting script hash transaction history
- [Get Raw Transaction from Transaction ID Documentation](../get_rawtx_from_txid/get_rawtx_from_txid.md) - Get raw transaction data
- [Get Merkle Path for Transaction Documentation](../get_merkle_path_for_tx/get_merkle_path_for_tx.md) - Get cryptographic proof for transactions 
