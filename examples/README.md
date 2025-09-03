# Examples

Here you’ll find several common usage examples for **go-wallet-toolbox**. The toolbox bundles wallet features that can be managed by a wallet server. In the following examples, you’ll learn how to run wallet-toolbox locally. Users can create wallets and connect to the wallet-toolbox server to perform actions.

## Getting Started

### Install dependencies
```bash
go mod tidy
```

Before running the examples, start a local wallet-toolbox server. This launches a local SQLite database that records wallet activity.

### `infra-config.yaml`
First, generate a new infra-config file that provides the parameters for a fresh wallet-toolbox instance:

```bash
go run ./cmd/infra_config_gen/main.go -k
```

### Start the server
```bash
go run ./cmd/infra/main.go
```

If the server starts successfully, you should see output similar to:

```text
{"time":"2025-07-18T15:40:10.1770867+10:00","level":"INFO","msg":"Starting task","service":"infra","worker":"77o6BFJVQs4M/Q7S","service":"monitor","task":"check_for_proofs","interval":60000000000}
{"time":"2025-07-18T15:40:10.1776014+10:00","level":"INFO","msg":"Listening","service":"infra","service":"storage_server","port":8100}
```

A `storage.sqlite` file should now appear in the project root. This file stores every user action along with the relevant metadata.

## Example Setup

In this section, we’ll show you the basic methods for getting started with client-side wallets. If you haven’t set up a local instance of Wallet Toolbox yet, follow the instructions in [Getting Started](./README.md#getting-started).

### Configuration: `examples-config.yaml`

When you run any example, the custom setup function will automatically create `examples/examples-config.yaml` if it doesn’t already exist.

The file defines two test users—**Alice** and **Bob**—each with an `identity_key` (public key) and a `private_key` (hex-encoded private key string). It also sets the BSV `network` (e.g., `test`) and the `server_url` for the local Wallet Toolbox instance.

Example of config file below: 

```go
alice:
    identity_key: 020c0ca23c75f7312bad0c5d81bff858bdcf468d3ad69a60b46ae90cafef557b03 // Alice identity key (hexadecimal format)
    private_key: 5a39d6a914e96be64873f7b954efa926a7d79f648810fad2e2b3aa11d31f9f69 // Alice private key (hexadecimal format)
bob:
    identity_key: 03e14a6f57e27ed5399307641be23ec497f19df99ff1ce7ef04ec82200a6f90b2b // Bob identity key (hexadecimal format)
    private_key: ca9e9dcb29fd7c7cf5ecebadd1a0dab029e571a570021e7ec699eb90acee333d // Bob private key (hexadecimal format)
network: test // network type ('test', 'main')
server_url: http://localhost:8100 // wallet toolbox URL location
```

Provided below are two methods to get started with using the wallet toolbox.
<br>**Note**:💡 These Faucet Examples are created as a helpers to get funds into the wallet to be able to use other examples.

## Faucet Examples
- [Show Address For Tx From Faucet](./wallet_examples/show_address_for_tx_from_faucet/show_address_for_tx_from_faucet.md) - Generate the user address and use a testnet faucet to receive funds.
- [Internalize Tx From Faucet](./wallet_examples/internalize_tx_from_faucet/internalize_tx_from_faucet.md) - Internalize a testnet transaction to the wallet toolbox.

## Wallet Examples
- [List Actions](./wallet_examples/list_actions/list_actions.md) - Get list of wallet actions of a specified user.
- [List Outputs](./wallet_examples/list_outputs/list_outputs.md) - Get list of transaction outputs of a specified user.
- [Internalize Wallet Payment](./wallet_examples/internalize_wallet_payment/internalize_wallet_payment.md) - Record an external wallet payment of a specified user wallet.
- [Create P2pkh Transaction](./wallet_examples/create_p2pkh_tx/create_p2pkh_tx.md) - Create a new p2pkh payment from a specified user wallet.
- [NoSend + SendWith (Batch Broadcast)](./wallet_examples/no_send_send_with/no_send_send_with.md) - Construct multiple actions with NoSend and broadcast them together with SendWith.

## Services Examples
- [Get Block Header form Block Hash](./services_examples/get_block_header_from_block_hash/get_block_header_from_block_hash.md) - Get a complete block header using a specific block hash.
- [Get Chain Tip Header](./services_examples/get_chain_tip_header/get_chain_tip_header.md) - Finds the latest block header in the longest chain.
- [Get Current Block Height](./services_examples/get_current_block_height/get_current_block_height.md) - Fetch current block height.
- [Get Raw Transaction from TxID](./services_examples/get_rawtx_from_txid/get_rawtx_from_txid.md) - Get raw transaction hexadecimal from a txID value.
- [Get Script Hash History](./services_examples/get_script_hash_history/get_script_hash_history.md) - Fetch transaction history for a specified script hash.
- [Is Valid Root For Block Height](./services_examples/is_valid_root_for_block_height/is_valid_root_for_block_height.md) - Validates that a root hex is valid for a specified block height.
- [Get Merkle Path For Tx](./services_examples/get_merkle_path_for_tx/get_merkle_path_for_tx.md) - Fetching Merkle Path for specified txID.
- [Post BEEF](./services_examples/post_beef/post_beef.md) - Broadcasting a single BSV transaction.
- [Post BEEF Hex](./services_examples/post_beef_hex/post_beef_hex.md) - Broadcasting a single BSV transaction hex format.
- [Post BEEF Multiple txs](./services_examples/post_beef_with_multiple_txs/post_beef_with_multiple_txs.md) - Broadcasting multiple transactions (grandparent, parent, and child).

## Explanations
### BEEF (Background Evaluation Extended Format)
[BEEF](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0062.md) is a binary format for sending transactions between peers to allow [Simple Payment Verification (SPV)](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0067.md). The format is optimized for minimal bandwidth while maintaining all data required to independently validate transactions in full.
BEEF includes transactions along with their Merkle paths using the [BSV Universal Merkle Path (BUMP)](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0074.md) format. This allows the transaction to be validated without requiring access to the blockchain, making it ideal for efficient transaction broadcasting.
