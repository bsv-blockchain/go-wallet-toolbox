# Get BEEF (Background Evaluation Extended Format)

This example demonstrates how to retrieve a transaction in BEEF format using a specific transaction ID (txID) on the BSV blockchain with the Go Wallet Toolbox SDK. It showcases how to construct a portable and verifiable transaction package, including its ancestry.

## Overview

BEEF (Background Evaluation Extended Format) is a standardized way to package a transaction along with its Merkle path (if mined) and the full transactions of its inputs. This allows for the verification of a transaction's validity and history without relying on a trusted third-party node to provide the inputs.

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
Wallet-Services is performing: fetching BEEF from services for txID: "f164e822e38f456f94de9f2b5089276b62dc7365ee68eb06c2919f9f5dcc55e3"
--------------------------------------------------
2025/08/08 10:31:43 WARN error when calling service service=services.MerklePath service.name=ARC error="tx f164e822e38f456f94de9f2b5089276b62dc7365ee68eb06c2919f9f5dcc55e3 not found"
✅ SUCCESS: Success, found a BEEF that contains 1 transactions and 1 BUMPS

============================================================
BEEF HEX
============================================================
"0200beef01feccc019000802ba02e355cc5d9f9f91c206eb68ee6573dc626b2789502b9fde946f458fe322e864f1bb01015c0038a1edb4465400fe88a39655970d3948900cea36faf193fce092806051af7cfe012f00383b00b52ce49c70058eb805b7d8daba4b560aca4585bc8dbe6b3734a597d580011600e65e8fdca6b7ba6d98f59242678afee369cbb217240ea25ecabb7a4351d92f75010a00dc6a35c7167370801207f356964acd79ee1d640b07239626ebe7034f0123f5230104005f48e1e33bfda67455da2018e9ae1f2f7a8ee002e83b9286e5463e0010d7998d0103003c1e5e67fb51947498ddf1a132a2bdf3a707cea586f1b963ac9508265207e8df0100006bdfb81a46bb81f71df0cdf73238de58b9300e194462da33c37614f39bc8d57a0101000100000001ef8471be22a93b36dbedfaa6354c052ade0265a7a00897e3b0aff5bef66b5de4570000006b483045022100dcc5aaad606dd1cd81305f293b3d8a01214a76fd224bd1c8592fb3669ab2b10802200cd1236554250e23fe7f19d47a263671c3db5925bf640e9321a6ca89b8d6134741210292acdb57c788c1e8c83cdb0ae8f23e079139ba7ba1bccf67b31653c7af12c4b4ffffffff0140860100000000001976a914c0ffe0da73403a55ae0e0d7e90f42d9db607efd288ac00000000"

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

