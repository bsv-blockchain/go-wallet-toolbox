# Create Action Example

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
- **`DefaultTransactionDescription`**: Human-readable description for the transaction (default: `"Create action example transaction"`)
- **`DefaultOutputDescription`**: Description for the payment output (default: `"Payment to recipient"`)
- **`DefaultOriginator`**: Domain identifier for the requesting application (default: `"example.com"`)

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
go run examples/wallet_examples/create_action/create_action.go
```

## Expected Output

```text
🚀 STARTING: Create Action
============================================================

=== STEP ===
Alice is performing: Creating wallet and setting up environment
--------------------------------------------------
CreateWallet: 03d2276c31630d6614f65c6634f40c0735a822d3501cc403ff459200971f747970
Recipient address: 1A6ut1tWnfg5mAD8s1drDLM6gNsLNGvgWq

=== STEP ===
Alice is performing: Creating transaction to send 100 satoshis
--------------------------------------------------
Transaction description: Create action example transaction
Output description: Payment to recipient

 WALLET CALL: CreateAction
Args: {Description:Create action example transaction InputBEEF:[] Inputs:[] Outputs:[{LockingScript:[118 169 20 99 215 90 127 139 69 130 10 199 153 87 39 106 150 29 236 194 12 85 39 136 172] Satoshis:100 OutputDescription:Payment to recipient Basket: CustomInstructions: Tags:[payment example]}] LockTime:<nil> Version:<nil> Labels:[create_action_example] Options:0xc000338080}
✅ Result: {Txid:cc1a1ec745a425ed0030a6643190faa67d64c81dbb7d8f39da32b4d500d9d10d Tx:[1 1 1 1 13 209 217 0 213 180 50 218 57 143 125 187 29 200 100 125 166 250 144 49 100 166 48 0 237 37 164 69 199 30 26 204 2 0 190 239 0 1 0 1 0 0 0 1 49 121 97 213 1 46 33 22 255 249 162 183 36 214 245 139 18 93 82 232 128 62 67 21 146 109 237 106 25 190 177 30 0 0 0 0 0 255 255 255 255 32 135 9 0 0 0 0 0 0 25 118 169 20 89 54 93 4 124 234 64 85 44 240 166 4 216 207 182 13 148 224 8 34 136 172 38 17 0 0 0 0 0 0 25 118 169 20 204 221 229 145 130 49 216 184 144 160 20 86 155 113 199 92 6 73 170 166 136 172 136 10 0 0 0 0 0 0 25 118 169 20 119 151 216 99 85 218 47 227 65 143 175 252 117 155 219 89 227 61 146 128 136 172 156 14 0 0 0 0 0 0 25 118 169 20 209 154 108 172 246 130 219 92 55 252 147 168 250 24 228 20 225 180 225 244 136 172 147 12 0 0 0 0 0 0 25 118 169 20 107 11 245 193 109 173 219 157 205 254 244 217 88 95 34 222 142 189 14 156 136 172 100 0 0 0 0 0 0 0 25 118 169 20 99 215 90 127 139 69 130 10 199 153 87 39 106 150 29 236 194 12 85 39 136 172 159 15 0 0 0 0 0 0 25 118 169 20 54 39 114 113 37 150 92 71 119 164 239 72 167 193 34 134 93 187 189 118 136 172 82 17 0 0 0 0 0 0 25 118 169 20 132 142 82 124 113 6 246 12 217 196 10 179 89 13 216 13 57 140 148 26 136 172 51 9 0 0 0 0 0 0 25 118 169 20 158 40 102 69 184 170 182 51 148 101 81 206 246 77 142 40 20 109 11 137 136 172 82 18 0 0 0 0 0 0 25 118 169 20 27 147 101 6 72 210 53 121 150 199 181 179 233 149 152 12 198 17 69 190 136 172 169 10 0 0 0 0 0 0 25 118 169 20 124 50 125 243 26 203 6 203 101 103 64 136 118 50 240 245 115 228 117 194 136 172 17 15 0 0 0 0 0 0 25 118 169 20 203 59 166 144 203 38 73 165 79 80 6 181 41 228 209 251 220 165 7 245 136 172 89 14 0 0 0 0 0 0 25 118 169 20 86 63 178 150 242 222 241 95 221 3 94 1 186 121 10 87 182 9 69 37 136 172 198 13 0 0 0 0 0 0 25 118 169 20 43 75 159 226 255 46 59 180 134 131 180 255 43 232 168 38 137 134 198 104 136 172 139 7 0 0 0 0 0 0 25 118 169 20 247 54 211 234 219 164 244 195 163 1 246 47 14 140 146 87 239 190 144 33 136 172 158 14 0 0 0 0 0 0 25 118 169 20 174 120 105 21 119 121 97 188 111 212 61 112 110 172 217 253 252 142 150 16 136 172 11 11 0 0 0 0 0 0 25 118 169 20 21 252 43 63 209 134 204 154 20 79 249 160 249 202 125 167 44 56 233 155 136 172 212 7 0 0 0 0 0 0 25 118 169 20 252 4 198 179 181 240 32 176 78 251 172 217 26 133 170 187 19 136 20 101 136 172 21 10 0 0 0 0 0 0 25 118 169 20 87 55 56 92 248 164 174 1 178 64 254 190 210 248 220 176 0 56 151 61 136 172 243 15 0 0 0 0 0 0 25 118 169 20 181 222 250 150 116 98 105 178 118 74 170 51 199 249 107 79 101 148 19 2 136 172 96 11 0 0 0 0 0 0 25 118 169 20 43 31 228 102 148 73 192 62 208 234 167 219 210 100 129 233 123 21 26 97 136 172 205 10 0 0 0 0 0 0 25 118 169 20 111 195 169 146 247 145 77 52 125 129 77 93 113 114 7 150 84 136 229 103 136 172 125 14 0 0 0 0 0 0 25 118 169 20 20 29 224 5 10 175 158 119 149 164 112 252 16 242 134 43 143 184 177 12 136 172 0 8 0 0 0 0 0 0 25 118 169 20 121 67 142 134 84 233 92 226 182 176 193 203 148 219 125 235 199 182 248 100 136 172 50 11 0 0 0 0 0 0 25 118 169 20 106 60 97 30 20 177 40 104 146 191 113 56 25 117 229 17 72 145 95 184 136 172 244 13 0 0 0 0 0 0 25 118 169 20 134 83 153 89 98 216 97 183 232 131 39 178 191 156 32 42 96 164 199 52 136 172 100 10 0 0 0 0 0 0 25 118 169 20 102 226 240 132 251 25 192 72 95 189 47 54 29 180 173 64 195 242 95 51 136 172 138 10 0 0 0 0 0 0 25 118 169 20 173 137 113 52 33 233 101 32 126 189 121 203 255 214 244 64 218 166 80 15 136 172 27 14 0 0 0 0 0 0 25 118 169 20 23 63 49 180 118 171 229 44 122 205 163 84 67 20 218 110 153 142 109 254 136 172 155 17 0 0 0 0 0 0 25 118 169 20 60 210 141 195 186 109 85 146 244 20 95 16 178 12 166 171 239 218 88 49 136 172 225 6 0 0 0 0 0 0 25 118 169 20 158 80 158 117 241 208 107 34 115 79 196 15 11 211 15 245 239 230 9 124 136 172 194 14 0 0 0 0 0 0 25 118 169 20 254 216 132 160 4 110 23 61 226 156 2 241 97 204 245 123 2 109 141 37 136 172 0 0 0 0] NoSendChange:[] SendWithResults:[{Txid:cc1a1ec745a425ed0030a6643190faa67d64c81dbb7d8f39da32b4d500d9d10d Status:sending}] SignableTransaction:<nil>}

🔗 TRANSACTION:
   TxID: cc1a1ec745a425ed0030a6643190faa67d64c81dbb7d8f39da32b4d500d9d10d
Status: Transaction successfully created and broadcast
Broadcast status: sending
✅ SUCCESS: Transaction created and sent successfully
============================================================
🎉 COMPLETED: Create Action
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

- [Create Action Example](./create_action.go) - Complete code example for creating transactions
- [List Actions Documentation](../list_actions/list_actions.md) - View wallet transaction history
- [List Outputs Documentation](../list_outputs/list_outputs.md) - View wallet transaction outputs
