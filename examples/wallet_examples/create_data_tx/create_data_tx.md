# Create Data Transaction Example

This example demonstrates how to create and send a Bitcoin transaction that embeds data using an OP_RETURN output via the wallet's `CreateAction` method.

## Overview

The process involves:
1. Setting up wallet configuration and establishing a connection to storage.
2. Providing the data to embed and basic metadata for the transaction.
3. Building an OP_RETURN output (zero satoshis) and creating the action.
4. Processing the response to confirm creation and broadcasting.

## Prerequisites

For this example to work, the wallet creating the transaction must contain funds to cover fees. Follow the [example setup](../../README.md#example-setup) to fund the wallet with spendable outputs.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable constants:

- **`DataToEmbed`**: The string to embed in the OP_RETURN output (must be non-empty)
- **`TransactionDescription`**: Human-readable description for the transaction (default: `"Create Data Transaction Example"`)
- **`OutputDescription`**: Description for the data output (default: `"Data output"`)
- **`Originator`**: Domain identifier for the requesting application (default: `"example.com"`)

### Request Parameters

The `CreateActionArgs` structure includes:

- **`Description`**: Human-readable description of the transaction
- **`Outputs`**: Contains a single OP_RETURN output with zero satoshis and tags like `data` and `example`
- **`Labels`**: Optional tags for categorizing and tracking the transaction
- **`Options`**: For example, `AcceptDelayedBroadcast`

### Response Analysis

The service response contains:

- **`Txid`**: The transaction identifier
- **`SendWithResults`**: Broadcast results including status

## Running the Example

```bash
go run examples/wallet_examples/create_data_tx/create_data_tx.go
```

## Expected Output

```text
🚀 STARTING: Create Data Transaction
============================================================

=== STEP ===
Alice is performing: Creating wallet and setting up environment
--------------------------------------------------
CreateWallet: 029c819ea4c340ec64cafb4e2f42d5aa5df79668ea2d98d4a772fbfb636c5ded4d
Data: hello world

=== STEP ===
Alice is performing: Creating transaction with OP_RETURN data
--------------------------------------------------
Transaction description: Create Data Transaction Example
Output description: Data output

 WALLET CALL: CreateAction
Args: {Description:Create Data Transaction Example InputBEEF:[] Inputs:[] Outputs:[{LockingScript:[0 106 11 104 101 108 108 111 32 119 111 114 108 100] Satoshis:0 OutputDescription:Data output Basket: CustomInstructions: Tags:[data example]}] LockTime:<nil> Version:<nil> Labels:[create_action_example] Options:0xc000338080}
✅ Result: {Txid:7eaee51c62ee99b662ca59476c577d3e84c89898ebc131d3adecf8b2f4075655 Tx:[1 1 1 1 85 86 7 244 178 248 236 173 211 49 193 235 152 152 200 132 62 125 87 108 71 89 202 98 182 153 238 98 28 229 174 126 2 0 190 239 0 1 0 1 0 0 0 1 184 40 142 89 123 138 230 130 168 229 94 5 138 194 112 140 249 166 15 255 155 243 82 2 96 29 233 17 148 111 157 151 31 0 0 0 106 71 48 68 2 32 54 148 234 208 79 170 127 5 43 4 134 107 39 106 233 27 137 201 8 231 240 217 188 145 118 222 42 116 11 148 109 58 2 32 82 66 165 76 134 198 189 254 117 123 24 118 26 67 235 206 44 43 253 95 178 210 105 79 92 210 29 204 6 215 149 198 65 33 3 179 36 79 39 100 11 210 65 54 219 33 56 39 105 19 157 41 67 254 12 143 205 16 222 129 246 230 65 165 79 178 116 255 255 255 255 2 0 0 0 0 0 0 0 0 14 0 106 11 104 101 108 108 111 32 119 111 114 108 100 104 20 0 0 0 0 0 0 25 118 169 20 47 218 179 30 191 201 88 159 127 231 224 61 84 188 203 224 28 3 42 94 136 172 0 0 0 0] NoSendChange:[] SendWithResults:[{Txid:7eaee51c62ee99b662ca59476c577d3e84c89898ebc131d3adecf8b2f4075655 Status:unproven}] SignableTransaction:<nil>}

🔗 TRANSACTION:
   TxID: 7eaee51c62ee99b662ca59476c577d3e84c89898ebc131d3adecf8b2f4075655
Status: Transaction successfully created and broadcast
Broadcast status: unproven
✅ SUCCESS: Transaction created and sent successfully
============================================================
🎉 COMPLETED: Create Data Transaction
```

## Integration Steps

1. **Set data** to embed in the OP_RETURN output and validate it is non-empty.
2. **Create OP_RETURN output** with zero satoshis using the provided SDK helper.
3. **Build `CreateActionArgs`** with description, labels, and options.
4. **Execute `CreateAction`** using the wallet and your `Originator` string.
5. **Process response** to read `Txid` and broadcast status.
6. **Handle errors and statuses** (e.g., insufficient funds, network errors).

## Additional Resources

- [Create Data Transaction](./create_data_tx.go) - Code example for embedding data with OP_RETURN
- [List Actions Documentation](../list_actions/list_actions.md) - View wallet transaction history
- [List Outputs Documentation](../list_outputs/list_outputs.md) - View wallet transaction outputs

