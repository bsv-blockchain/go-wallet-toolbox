# Faucet Address Example

This example demonstrates how to generate a BRC-29 testnet address for receiving funds from public testnet faucets. This is typically used as a first step to get initial funds into a wallet for testing other examples.

## Overview

The process involves several steps:
1. Creating a wallet instance (Alice).
2. Generating BRC-29 derivation parameters (prefix and suffix).
3. Creating a BRC-29 testnet address using the wallet's private key and identity key.
4. Displaying the generated address with instructions for funding from testnet faucets.

This example serves as a helper to get funds into the wallet so you can use other wallet examples that require existing funds.

## Code Walkthrough

### Creating the Wallet Instance

```go
alice := example_setup.CreateAlice()
```
First, we create a wallet instance called "Alice" using the example setup utility. This creates a wallet with a private key and identity key that will be used for address generation.

### Generating Derivation Parameters

```go
parts := utils.DerivationParts()

keyID := brc29.KeyID{
    DerivationPrefix: string(parts.DerivationPrefix),
    DerivationSuffix: string(parts.DerivationSuffix),
}
```
The `DerivationParts()` function generates the required BRC-29 derivation parameters:
- **DerivationPrefix**: Base64-encoded prefix for key derivation
- **DerivationSuffix**: Base64-encoded suffix for key derivation

These parameters are essential for BRC-29 address generation and are later stored with transactions so the wallet knows how to spend the outputs.

### Creating the BRC-29 Address

```go
address, err := brc29.Address(
    wallet.PrivateKey,
    keyID,
    wallet.IdentityKey,
    brc29.WithTestNet(),
)
```
Using the wallet's private key, the generated keyID (containing derivation parameters), and the wallet's identity key, we create a BRC-29 testnet address. The `brc29.WithTestNet()` option ensures the address is formatted for the Bitcoin testnet.

### Displaying the Address and Instructions

```go
show.FaucetInstructions(address.AddressString)
```
Finally, the example displays formatted instructions including:
- The generated testnet address
- Links to available testnet faucets
- Warnings about using testnet faucets only

## Running the Example

To run this example:

```bash
go run ./examples/wallet_examples/show_address_for_tx_from_faucet/show_address_for_tx_from_faucet.go
```

## Expected Output

```text
============================================================
FAUCET ADDRESS
============================================================

💡  NOTICE: You need to fund this address from a testnet faucet

📧  ADDRESS:
    mqG1q3y6CVaDoQed4cbCsgSfm3cgDHugsG

Available Testnet Faucets:
• https://scrypt.io/faucet
• https://witnessonchain.com/faucet/tbsv

⚠️  WARNING: Make sure to use TESTNET faucets only!
```

**Note**: Each run generates the same address for Alice since the wallet is deterministically created. The address will remain consistent across runs for the same wallet instance.

## Integration Steps

To integrate address generation into your application:

1. **Create or load a wallet instance** with the required private key and identity key.
2. **Generate derivation parameters** using the provided utility functions or your own implementation.
3. **Create the BRC-29 address** by calling `brc29.Address()` with the wallet keys, derivation parameters, and network configuration.
4. **Fund the address** by using the generated address with a testnet faucet.
5. **Store the derivation parameters** along with the transaction data for later spending.
6. **Continue to internalization** using the [internalize_tx_from_faucet](../internalize_tx_from_faucet/internalize_tx_from_faucet.md) example to import the funded transaction into your wallet.

### Funding the Address

1. Copy the generated address from the console output.
2. Open any of the listed testnet faucets.
3. Paste the address and request coins.
4. The faucet will return a **txID** if successful - save this value for the next step.
5. Use the txID with the **internalize tx from faucet** example to import the funds into your wallet.

## Additional Resources

- [BRC-29 Payment Protocol](https://bsv.brc.dev/payments/0029) - Specification for payment address derivation
- [Internalize Transaction from Faucet Example](../internalize_tx_from_faucet/internalize_tx_from_faucet.md) - Next step to import faucet funds
- [Scrypt Testnet Faucet](https://scrypt.io/faucet) - Get free testnet BSV
- [Witnessonchain Testnet Faucet](https://witnessonchain.com/faucet/tbsv) - Another source for testnet BSV
