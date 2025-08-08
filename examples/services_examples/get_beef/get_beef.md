# Get BEEF (Bitcoin Extended Transaction Format)

This example demonstrates how to retrieve a transaction in BEEF format using a specific transaction ID (txID) on the BSV blockchain with the Go Wallet Toolbox SDK. It showcases how to construct a portable and verifiable transaction package, including its ancestry.

## Overview

BEEF (Bitcoin Extended Transaction Format) is a standardized way to package a transaction along with its Merkle path (if mined) and the full transactions of its inputs. This allows for the verification of a transaction's validity and history without relying on a trusted third-party node to provide the inputs.

The process involves these steps:
1.  Setting up the services configuration for the desired network.
2.  Defining the transaction ID (txID) for which to generate the BEEF.
3.  Calling the `GetBEEF()` method, which recursively fetches the transaction and its parents.
4.  The recursion stops when it encounters a mined transaction (one with a Merkle path) or reaches a predefined depth limit.
5.  Processing and displaying the resulting BEEF data, which includes the full transaction data for the specified txID and its unmined ancestors.

This approach is useful for applications that require a high degree of security and verifiability for transactions, especially in scenarios involving unconfirmed (zero-conf) transactions.

## Code Walkthrough

### Configuration Parameters

The example uses the following parameters:

- **`txID`**: The specific transaction ID to retrieve in BEEF format (default: `"323f6413e49b46fe58810b84f8aa912c53f6ef436b9e5dfcb9a78a6000efbb32"`)
- **`Network`**: The blockchain network to query (default: `defs.NetworkMainnet`)

### Service Method: `GetBEEF`

The `GetBEEF` method requires:

- **`Context`**: A context for managing the request lifecycle.
- **`txID`**: The hexadecimal transaction ID for which to generate the BEEF.
- **`knownTxIDs`**: An optional slice of transaction IDs that are already known to the caller. The service will not fetch transactions present in this list, which can optimize the process. In this example, it's `nil`.

### Response Analysis

The service response contains:

- **`*transaction.Beef`**: A BEEF object containing:
    - The full transaction data for the requested `txID`.
    - Full transaction data for all unmined parent transactions, recursively.
    - The Merkle path for any transactions in the ancestry that are confirmed in a block.

## Running the Example

To run this example:

```bash
go run ./examples/services_examples/get_beef/get_beef.go
```

## Expected Output

```text
🚀 STARTING: Get BEEF by TxID
============================================================

=== STEP ===
Wallet-Services: fetching BEEF from services for txID: "323f6413e49b46fe58810b84f8aa912c53f6ef436b9e5dfcb9a78a6000efbb32"
--------------------------------------------------
✅ SUCCESS: Fetched BEEF
BEEF Hex: 0100000002000000010000000100000032bbef00a78acb9e5d9e6b43eff6532c91aaf8840b8158fe469be413643f32010000000100000001000000010000000100000000
============================================================
🎉 COMPLETED: Get BEEF by TxID
```

## Integration Steps

To integrate BEEF retrieval into your application:

1.  **Configure services**: Set up the `services.WalletServices` with the appropriate network.
2.  **Provide txID**: Identify the transaction ID you need to verify.
3.  **Call `GetBEEF`**: Use the `srv.GetBEEF(ctx, txID, knownTxIDs)` method.
4.  **Process BEEF**: Use the returned `beef` object for verification, storage, or transmission. You can inspect the transactions and their Merkle paths within the object.
5.  **Handle errors**: Implement robust error handling for cases where transactions cannot be found or the service fails.

## Additional Resources

- [Get BEEF Example Code](./get_beef.go) - The complete, runnable Go source file for this example.
- [Get Raw Transaction from TxID Documentation](../get_rawtx_from_txid/get_rawtx_from_txid.md) - For fetching a single raw transaction.
- [Get Merkle Path for Transaction Documentation](../get_merkle_path_for_tx/get_merkle_path_for_tx.md) - For fetching just the Merkle proof of a transaction.

