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
go run ./cmd/infra_config_gen/main.go
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

- [Example Setup](./internal/example_setup/README) - Start guide to creating a user wallet and interacting with the wallet toolbox local setup.

## Wallet Examples

- [List Actions](./wallet_examples/list_actions/README) - Get list of wallet actions of a specific user. //TODO: Add README
- [List Outputs](./wallet_examples/list_outputs/README) - Get list of transaction outputs of a specific user. //TODO: Add README

## Services Examples

- [Find Chain Tip Header](./services_examples//find_chain_tip_header/README) - Finds the latest block header in the longest chain. //TODO: Add README
- [Height](./services_examples/height/README) - Fetch current block height. //TODO: Add README
- [Is Valid Root](./services_examples/is_valid_root/README) - Validates a root hex is valid for a specified height. //TODO: Add README
- [Merkle Path](./services_examples/services_merkle_path/README) - Fetching Merkle Path for specified txID. //TODO: Add README
- [Post BEEF](./services_examples/services_post_beef/README) - Broadcasting a single BSV transaction. //TODO: Add README
- [Post BEEF Hex](./services_examples/services_post_beef_hex/README) - Broadcasting a single BSV transaction hex format. //TODO: Add README
- [Post BEEF Multiple txs](./services_examples/services_post_multiple_txs/README) - Broadcasting multiple transactions (grandparent, parent, and child). //TODO: Add README
