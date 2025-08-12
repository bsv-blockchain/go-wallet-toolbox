# Create P2PKH Transaction Example

This example demonstrates how to create and send a Bitcoin transaction using the wallet's `CreateAction` method. It showcases the complete transaction creation process from wallet setup to transaction broadcasting.

## Overview

The process involves several steps:
1. Setting up wallet configuration and establishing connection to storage.
2. Configuring transaction parameters including recipient address, amount, and descriptions.
3. Creating transaction arguments with proper output specifications and metadata.
4. Executing the transaction creation using the wallet's `CreateAction` method.
5. Processing the response to confirm successful transaction creation and broadcasting.

This approach ensures reliable transaction creation with proper error handling and confirmation mechanisms.

## Prerequisites

For this example to work, the wallet creating the transaction must contain funds. You will need to follow the [wallet setup process](../../README.md#example-setup) to fund the wallet with spendable outputs.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable constants:

- **`RecipientAddress`**: Target address for the transaction (default: `"1A6ut1tWnfg5mAD8s1drDLM6gNsLNGvgWq"`)
- **`SatoshisToSend`**: Amount to send in satoshis (default: `100`)
- **`TransactionDescription`**: Human-readable description for the transaction (default: `"Create P2PKH Transaction Example"`)
- **`OutputDescription`**: Description for the payment output (default: `"Payment to recipient"`)
- **`Originator`**: Domain identifier for the requesting application (default: `"example.com"`)

### Request Parameters

The `CreateActionArgs` structure supports the following options:

- **`Description`**: Human-readable description of the transaction
- **`Outputs`**: Array of outputs specifying recipients, amounts, and metadata
- **`Labels`**: Tags for categorizing and tracking the transaction

### Response Analysis

The service response contains:

- **`Txid`**: The unique transaction identifier for the created transaction
- **`SendWithResults`**: Array of broadcast results with transaction status and confirmation details

## Running the Example

```bash
go run examples/wallet_examples/create_p2pkh_tx/create_p2pkh_tx.go
```

## Expected Output

```text
🚀 STARTING: Create P2PKH Transaction
============================================================

=== STEP ===
Alice is performing: Creating wallet and setting up environment
--------------------------------------------------
CreateWallet: 02c718b625dcabacb6fcd0f15d575e9cbe3ac80ab4338a11bd869f64e1f0683801
Recipient address: 1A6ut1tWnfg5mAD8s1drDLM6gNsLNGvgWq

=== STEP ===
Alice is performing: Creating transaction to send 100 satoshis
--------------------------------------------------
Transaction description: Create P2PKH Transaction Example
Output description: Payment to recipient

 WALLET CALL: CreateAction
Args: {Description:Create P2PKH Transaction Example InputBEEF:[] Inputs:[] Outputs:[{LockingScript:[118 169 20 99 215 90 127 139 69 130 10 199 153 87 39 106 150 29 236 194 12 85 39 136 172] Satoshis:100 OutputDescription:Payment to recipient Basket: CustomInstructions: Tags:[payment example]}] LockTime:<nil> Version:<nil> Labels:[create_p2pkh_tx_example] Options:<nil>}
✅ Result: {Txid:c576e40084bc0f349b881caee9e93c8caf6250b4dadda5df4bd6491860b6c078 Tx:[1 1 1 1 120 192 182 96 24 73 214 75 223 165 221 218 180 80 98 175 140 60 233 233 174 28 136 155 52 15 188 132 0 228 118 197 2 0 190 239 0 1 0 1 0 0 0 1 115 82 190 169 105 208 114 50 68 85 195 180 21 99 218 255 121 181 16 207 216 119 87 158 58 71 81 174 180 205 194 84 1 0 0 0 106 71 48 68 2 32 89 28 150 71 199 40 169 6 156 77 70 91 117 58 233 23 100 245 165 1 28 250 250 45 173 15 120 206 69 14 60 192 2 32 97 125 75 94 59 159 26 231 255 19 155 178 130 30 186 154 79 10 158 205 15 1 157 250 231 148 180 110 43 102 214 136 65 33 2 36 245 167 32 6 151 233 229 37 226 7 145 231 129 225 142 25 63 232 71 53 158 158 25 77 132 79 235 195 118 169 251 255 255 255 255 2 131 3 0 0 0 0 0 0 25 118 169 20 24 154 81 215 34 186 118 7 233 210 174 120 242 75 217 5 23 217 219 233 136 172 100 0 0 0 0 0 0 0 25 118 169 20 99 215 90 127 139 69 130 10 199 153 87 39 106 150 29 236 194 12 85 39 136 172 0 0 0 0] NoSendChange:[] SendWithResults:[{Txid:c576e40084bc0f349b881caee9e93c8caf6250b4dadda5df4bd6491860b6c078 Status:sending}] SignableTransaction:<nil>}

🔗 TRANSACTION:
   TxID: c576e40084bc0f349b881caee9e93c8caf6250b4dadda5df4bd6491860b6c078
Status: Transaction successfully created and broadcast
Broadcast status: sending
✅ SUCCESS: Transaction created and sent successfully
============================================================
🎉 COMPLETED: Create P2PKH Transaction
```

## Integration Steps

To integrate transaction creation into your application:

1. **Configure transaction parameters** including recipient addresses, amounts, and descriptions.
2. **Set up wallet connection** with appropriate storage and authentication settings.
3. **Create transaction arguments** with proper output specifications, labels, and metadata.
4. **Execute transaction creation** using the wallet's `CreateAction` method.
5. **Process response data** to extract transaction ID and broadcast status.
6. **Handle transaction states** including pending, sending, and confirmed statuses.
7. **Implement error handling** for insufficient funds, invalid addresses, or network issues.

## Additional Resources

- [Create P2PKH Transaction](./create_p2pkh_tx.go) - Complete code example for creating transactions
- [List Actions Documentation](../list_actions/list_actions.md) - View wallet transaction history
- [List Outputs Documentation](../list_outputs/list_outputs.md) - View wallet transaction outputs
