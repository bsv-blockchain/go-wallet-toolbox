# Faucet - Address

In this section, we’ll generate a new address for the user and then fund it from a public **Testnet** faucet.
<br>**Note**:💡 This example is created as a helper to get funds into the wallet so you can use other examples.

## Generate a Testnet Address

Generate an address for **Alice**:

```bash
go run ./examples/wallet_examples/faucet_address/faucet_address.go
```

Expected output:

```text
============================================================
FAUCET ADDRESS
============================================================

💡  NOTICE: Fund this address using a Testnet faucet.

📧  ADDRESS:
    mqG1q3y6CVaDoQed4cbCsgSfm3cgDHugsG

Available Testnet faucets:
• https://scrypt.io/faucet
• https://witnessonchain.com/faucet/tbsv

⚠️  WARNING: Use **Testnet** faucets only!
```

### Funding the address

1. Open any faucet listed above.
2. Paste the generated address and request coins.  
3. The faucet will return a **txid** if the request succeeds. Copy this value—you’ll need it to import the funds into Alice’s wallet on the Wallet Toolbox local server.

Continue with **[faucet_internalize](../../wallet_examples/faucet_internalize/README)** to internalize the transaction.
<br>

## Details
### Address Derivation

Creating a user address requires a **keyID** that includes both a derivation prefix and suffix, as specified by the [BRC-29 payment protocol](https://bsv.brc.dev/payments/0029). We use a helper [derivation method](../../internal/utils/key_derivation.go) to provide these parameters. The derivation values are later stored with each transaction so the wallet knows how to spend the outputs.
