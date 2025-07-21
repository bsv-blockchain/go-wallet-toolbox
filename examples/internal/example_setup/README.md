# Example Setup

In this section, we’ll show you the basic methods for getting started with client-side wallets. If you haven’t set up a local instance of Wallet Toolbox yet, follow the instructions in [Getting Started](../../README.md).

### Configuration: `examples-config.yaml`

When you run any example, the custom setup function will automatically create `examples/examples-config.yaml` if it doesn’t already exist.

The file defines two test users—**Alice** and **Bob**—each with an `identity_key` (public key) and a `private_key` (hex-encoded private key string). It also sets the BSV `network` (e.g., `test`) and the `server_url` for the local Wallet Toolbox instance.

Example of config file below : 

```go
alice:
    identity_key: 020c0ca23c75f7312bad0c5d81bff858bdcf468d3ad69a60b46ae90cafef557b03 // Alice identity key (hexadecimal format)
    private_key: 5a39d6a914e96be64873f7b954efa926a7d79f648810fad2e2b3aa11d31f9f69 // Alice private key (hexadecimal format)
bob:
    identity_key: 03e14a6f57e27ed5399307641be23ec497f19df99ff1ce7ef04ec82200a6f90b2b // Bob identity key (hexadecimal format)
    private_key: ca9e9dcb29fd7c7cf5ecebadd1a0dab029e571a570021e7ec699eb90acee333d // Bob private key (hexdecimal format)
network: test // network type ('test', 'main')
server_url: http://localhost:8100 // wallet toolbox URL location
```

Provided below are two methods to get started with using the wallet toolbox.
## Faucet Address
[ Faucet Address](../../wallet_examples/faucet_address/README) - Generate the user address and use a testnet faucet to receive funds.

## Faucet Internalize
[Faucet Internalize](../../wallet_examples/faucet_internalize/README) - Internalize a testnet transaction to the wallet toolbox.
