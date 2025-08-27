# Faucet Internalize Example

This example demonstrates how to internalize a funded testnet transaction into your wallet database, completing the faucet funding process. This imports the transaction received from a testnet faucet into the local wallet storage.

## Overview

The process involves several steps:
1. Setting up the wallet instance and environment.
2. Retrieving transaction data in BEEF format using the transaction ID from the faucet.
3. Creating internalization arguments with proper derivation parameters.
4. Calling the wallet's `InternalizeAction` to import the transaction.
5. Verifying the transaction is successfully stored in the wallet database.

This example serves as the second step after generating a faucet address, completing the process of getting initial testnet funds into your wallet.

## Code Walkthrough

### Setting Up the Transaction ID

```go
var txID = "15f47f2db5f26469c081e8d80d91a4b0f06e4a97abcc022b0b5163ac5f6cc0c8"
```
First, you need the transaction ID received from the testnet faucet in the previous [show_address_for_tx_from_faucet](../show_address_for_tx_from_faucet/show_address_for_tx_from_faucet.md) example. Replace this placeholder with your actual txid.

### Creating the Wallet Instance

```go
alice := example_setup.CreateAlice()
aliceWallet, cleanup, err := alice.CreateWallet(ctx)
```
We create the same wallet instance (Alice) and establish a connection to the wallet database. The cleanup function ensures proper resource management.

### Retrieving BEEF Data

```go
beef, err = utils.WocAPIGetBeefForTX(defs.NetworkTestnet, txID)
```
The `WocAPIGetBeefForTX` function calls the WhatsonChain API to retrieve the transaction data in BEEF (Background Evalution Extended Format) hex format. BEEF is specified in [BRC-62](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0062.md) and contains the complete transaction with merkle proof data.

### Validate Vouts for Internalization
```go
var vouts []int
	for vout, output := range tx.Outputs {
		if output.LockingScript.Equals(lockingScript) {
			vouts = append(vouts, vout)
		}
	}
```
The locking script of the BRC29 address that was used in the transaction is validated against all outputs in the transaction. This process allows more than one output to be internalized if it is matched to the address.

### Creating Internalization Arguments

```go
paymentRemittance := utils.DerivationParts()
senderIdentityKey, err := ec.PublicKeyFromString(paymentRemittance.SenderIdentityKey)

internalizeArgs := sdk.InternalizeActionArgs{
    Tx: beef,
    Outputs: []sdk.InternalizeOutput{
        {
            OutputIndex: 0,
            Protocol:    "wallet payment",
            PaymentRemittance: &sdk.Payment{
                DerivationPrefix:  paymentRemittance.DerivationPrefix,
                DerivationSuffix:  paymentRemittance.DerivationSuffix,
                SenderIdentityKey: senderIdentityKey,
            },
        },
    },
    Description: "internalize from faucet",
}
```
The internalization arguments include:
- **Tx**: The BEEF hex data containing the transaction
- **Outputs**: Array specifying which outputs to internalize (typically output 0 from faucets)
- **PaymentRemittance**: The same derivation parameters used when generating the address
- **Description**: Human-readable description for the transaction

### Internalizing the Transaction

```go
iar, err := wallet.InternalizeAction(ctx, internalizeArgs, "originator")
```
Finally, we call the wallet's `InternalizeAction` method to import the transaction into the local database. This makes the funds available for spending in future transactions.

## Running the Example

**Prerequisite**: You must have a transaction ID from completing the [show_address_for_tx_from_faucet](../show_address_for_tx_from_faucet/show_address_for_tx_from_faucet.md) example.

1. **Update the transaction ID** in the code:
   ```go
   var txID = "your_actual_txid_here"
   ```

2. **Run the example**:
   ```bash
   go run ./examples/wallet_examples/internalize_tx_from_faucet/internalize_tx_from_faucet.go
   ```

## Expected Output
```text
🚀 STARTING: Faucet Transaction Internalization
============================================================

=== STEP ===
Alice is performing: Creating wallet and setting up environment
--------------------------------------------------
CreateWallet: 03d397c0f79268471ab7c582b939f70e8ea06f63a3861f23425102f89c9ed043d0

=== STEP ===
Alice is performing: Retrieving transaction data
--------------------------------------------------

🔗 TRANSACTION:
   TxID: ef13e18c0eb65b14d3a7222f44123478f196a988d45a3c5ba22412dc63ef0ca0

=== STEP ===
Wallet-Services is performing: fetching BEEF from services for txID: "ef13e18c0eb65b14d3a7222f44123478f196a988d45a3c5ba22412dc63ef0ca0"
--------------------------------------------------

=== STEP ===
Alice is performing: Internalizing transaction from faucet
--------------------------------------------------
Outputs matching to the derived address based on the payment remittance: [0]

 WALLET CALL: InternalizeAction
Args: {Tx:[1 1 1 1 160 12 239 99 220 18 36 162 91 60 90 212 136 169 150 241 120 52 18 68 47 34 167 211 20 91 182 14 140 225 19 239 2 0 190 239 1 254 23 205 25 0 3 2 6 2 160 12 239 99 220 18 36 162 91 60 90 212 136 169 150 241 120 52 18 68 47 34 167 211 20 91 182 14 140 225 19 239 7 0 111 56 41 183 59 123 196 25 144 121 107 35 28 54 34 104 253 108 66 1 131 17 75 105 0 20 224 36 28 98 14 135 1 2 0 89 50 73 44 184 15 194 59 61 9 165 213 218 105 90 140 13 121 72 162 141 231 129 71 74 237 44 182 34 117 113 117 1 0 0 119 223 96 120 58 121 96 229 226 56 231 40 81 83 196 124 61 48 120 218 78 237 132 89 177 34 222 29 113 248 106 8 1 1 0 1 0 0 0 1 251 204 183 107 193 24 156 107 48 181 54 255 229 136 239 184 42 41 195 16 179 226 42 20 168 183 213 211 19 63 21 36 2 0 0 0 106 71 48 68 2 32 22 73 187 84 71 234 69 206 152 234 208 215 226 17 115 134 150 187 220 243 40 253 192 28 195 157 49 32 183 231 7 141 2 32 9 122 255 204 198 82 212 174 213 110 153 140 52 170 214 71 67 196 35 226 94 104 173 249 26 12 46 194 118 104 204 55 65 33 3 202 40 55 226 205 129 168 105 20 206 164 83 101 58 67 84 50 32 33 144 2 241 160 242 2 134 170 73 251 180 32 37 255 255 255 255 2 232 3 0 0 0 0 0 0 25 118 169 20 82 193 123 230 178 66 18 252 122 37 186 25 32 236 110 98 25 31 201 129 136 172 112 13 0 0 0 0 0 0 25 118 169 20 210 179 41 234 113 86 220 127 131 57 141 216 233 28 79 224 193 82 134 184 136 172 0 0 0 0] Description:internalize from faucet Labels:[] SeekPermission:<nil> Outputs:[{OutputIndex:0 Protocol:wallet payment PaymentRemittance:0xc0000a3340 InsertionRemittance:<nil>}]}
✅ Result: {Accepted:true}
✅ SUCCESS: Transaction internalized successfully
============================================================
🎉 COMPLETED: Faucet Transaction Internalization
```

**Note**: If successful, you'll find a new row in `storage.sqlite` under Alice's identity key representing the internalized funds.

## Integration Steps

To integrate transaction internalization into your application:

1. **Obtain the transaction ID** from your funding source (faucet, payment, etc.).
2. **Retrieve BEEF data** using the WhatsonChain API or your preferred blockchain data source.
3. **Prepare derivation parameters** that match those used when generating the receiving address.
4. **Create InternalizeActionArgs** with the transaction data, output specifications, and payment remittance details.
5. **Call InternalizeAction** on your wallet instance to import the transaction.
6. **Verify the result** and handle any errors appropriately.
7. **Update your application state** to reflect the new available balance.

### Error Handling

Common issues and solutions:
- **Transaction not found**: Ensure the transaction ID is correct and the transaction has been confirmed
- **Invalid BEEF data**: Verify the API response and network settings
- **Derivation mismatch**: Ensure the same derivation parameters are used as when generating the address

## Additional Resources

- [Show Address for Transaction from Faucet Example](../show_address_for_tx_from_faucet/show_address_for_tx_from_faucet.md) - Previous step to generate the receiving address
- [BRC-62 BEEF Specification](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0062.md) - Transaction format specification
- [WhatsonChain API](https://docs.whatsonchain.com/) - Blockchain data API documentation
