# Examples

Here you'll find several common usage examples for **go-wallet-toolbox**. The toolbox bundles wallet features that can be managed by a wallet server. In the following examples, you'll learn how to run wallet-toolbox locally. Users can create wallets and connect to the wallet-toolbox server to perform actions.

## Getting Started - Step by Step Guide

### 1. Run Faucet Examples (Get Test Funds)
Start with these examples to get test funds into your wallet. You'll need test BSV to perform transactions:

- [Show Address For Tx From Faucet](./wallet_examples/show_address_for_tx_from_faucet/show_address_for_tx_from_faucet.md) - Generate the user address and use a testnet faucet to receive funds.
- [Internalize Tx From Faucet](./wallet_examples/internalize_tx_from_faucet/internalize_tx_from_faucet.md) - Internalize a testnet transaction to the wallet toolbox.

### 2. Check Your Balance
Verify that your funds have been received and are available for spending:

- [Get Balance](./wallet_examples/get_balance/get_balance.md) - Calculate the total balance of a wallet by summing all satoshis from outputs.

### 3. Send Data Transactions
Now you can create and broadcast transactions:

- [Create Data Transaction](./wallet_examples/create_data_tx/create_data_tx.md) - Create OP_RETURN transactions.

### 4. Additional Wallet Operations
Explore more wallet functionality:

- [Create P2pkh Transaction](./wallet_examples/create_p2pkh_tx/create_p2pkh_tx.md) - Create a new p2pkh payment from a specified user wallet.
- [Decrypt](./wallet_examples/decrypt/decrypt.md) - Decrypt an encrypted message using wallet-based cryptographic operations.
- [Encrypt](./wallet_examples/encrypt/encrypt.md) - Encrypt a message using wallet-based cryptographic operations.
- [List Actions](./wallet_examples/list_actions/list_actions.md) - Get list of wallet actions of a specified user.
- [List Outputs](./wallet_examples/list_outputs/list_outputs.md) - Get list of transaction outputs of a specified user.
- [Internalize Wallet Payment](./wallet_examples/internalize_wallet_payment/internalize_wallet_payment.md) - Record an external wallet payment of a specified user wallet.
- [NoSend + SendWith (Batch Broadcast)](./wallet_examples/no_send_send_with/no_send_send_with.md) - Construct multiple actions with NoSend and broadcast them together with SendWith.

## Configuration

### Install Dependencies
First, install the required dependencies:
```bash
go mod tidy
```

### Local Storage Setup
When you run any example, the custom setup function will automatically create `examples/examples-config.yaml` if it doesn't already exist.

The file defines two test users—**Alice** and **Bob**—each with an `identity_key` (public key) and a `private_key` (hex-encoded private key string). It also sets the BSV `network` (e.g., `test`), `server_url` for the Wallet Toolbox instance, and `server_private_key` for server authentication.

Example of config file below:

```yaml
alice:
    identity_key: 0396e909ba1d94f0073beb80935ac42bd2a3c8e7f071610a7f2349d4fbab874254
    private_key: 5e59605d936fa9390ee7c9312e3cc946df1c1405126d5455cbae83598d80b076
bob:
    identity_key: 03af7e04bb4bc4678f34add99baa299d21e1d12c78dceb861790fdddac37c29d15
    private_key: e08edef5c0b5e6ad506debad51a9d6f6d12d517e07e7851c9f3e557b7c8ab160
network: test
server_private_key: 2b32d442b25d6e7447a1f9ca41a2a15de5004498dc4ffc43b7b009a96724c30d
server_url: ""
```

**Note**: When auto-generated, `server_url` is set to an empty string. For local development, this will automatically initialize a local storage `storage.sqlite` file. For remote usage, you'll need to set the appropriate server URL.

### Remote Storage Setup
To use a remote wallet-toolbox server instead of local storage, update the `server_url` in your `examples-config.yaml`:

```yaml
server_url: https://your-remote-server.com
```

## Server Setup (Local Development)

If you want to run your own local wallet-toolbox server, follow these steps:

### Start the Server
```bash
go run ./examples/main.go
```

If the server starts successfully, you should see output similar to:

```text
{"time":"2025-07-18T15:40:10.1770867+10:00","level":"INFO","msg":"Starting task","service":"infra","worker":"77o6BFJVQs4M/Q7S","service":"monitor","task":"check_for_proofs","interval":60000000000}
{"time":"2025-07-18T15:40:10.1776014+10:00","level":"INFO","msg":"Listening","service":"infra","service":"storage_server","port":8100}
```

A `storage.sqlite` file should now appear in the `./examples` directory. This file stores every user action along with the relevant metadata.

## Services Examples
These examples demonstrate how to interact with blockchain services and APIs to retrieve blockchain data, validate transactions, and broadcast transactions to the network.

- [Get BEEF](./services_examples/get_beef/get_beef.md) - Retrieve a transaction in BEEF format using a specific transaction ID.
- [Get Block Header form Block Hash](./services_examples/get_block_header_from_block_hash/get_block_header_from_block_hash.md) - Get a complete block header using a specific block hash.
- [Get Chain Tip Header](./services_examples/get_chain_tip_header/get_chain_tip_header.md) - Finds the latest block header in the longest chain.
- [Get Current Block Height](./services_examples/get_current_block_height/get_current_block_height.md) - Fetch current block height.
- [Get Merkle Path For Tx](./services_examples/get_merkle_path_for_tx/get_merkle_path_for_tx.md) - Fetching Merkle Path for specified txID.
- [Get Raw Transaction from TxID](./services_examples/get_rawtx_from_txid/get_rawtx_from_txid.md) - Get raw transaction hexadecimal from a txID value.
- [Get Script Hash History](./services_examples/get_script_hash_history/get_script_hash_history.md) - Fetch transaction history for a specified script hash.
- [Get Status For TxIDs](./services_examples/get_status_for_txids/get_status_for_txids.go) - Query depth/status for multiple transaction IDs.
- [Get UTXO Status](./services_examples/get_utxo_status/get_utxo_status.md) - Check the status of unspent transaction outputs.
- [Hash Output Script](./services_examples/hash_output_script/hash_output_script.go) - Hash an output script to get its script hash.
- [Is UTXO](./services_examples/is_utxo/is_utxo.md) - Check if a transaction output is unspent.
- [Is Valid Root For Block Height](./services_examples/is_valid_root_for_block_height/is_valid_root_for_block_height.md) - Validates that a root hex is valid for a specified block height.
- [NLockTime Is Final](./services_examples/nlock_time_is_final/nlock_time_is_final.go) - Check if a transaction's nLockTime is final (past timestamp or block height).
- [Post BEEF](./services_examples/post_beef/post_beef.md) - Broadcasting a single BSV transaction.
- [Post BEEF Hex](./services_examples/post_beef_hex/post_beef_hex.md) - Broadcasting a single BSV transaction hex format.
- [Post BEEF Multiple txs](./services_examples/post_beef_with_multiple_txs/post_beef_with_multiple_txs.md) - Broadcasting multiple transactions (grandparent, parent, and child).

## Explanations
### BEEF (Background Evaluation Extended Format)
[BEEF](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0062.md) is a binary format for sending transactions between peers to allow [Simple Payment Verification (SPV)](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0067.md). The format is optimized for minimal bandwidth while maintaining all data required to independently validate transactions in full.
BEEF includes transactions along with their Merkle paths using the [BSV Universal Merkle Path (BUMP)](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0074.md) format. This allows the transaction to be validated without requiring access to the blockchain, making it ideal for efficient transaction broadcasting.
