# Internalize Wallet Action

This example demonstrates how to internalize an external transaction into a BSV wallet using the Go Wallet Toolbox SDK. This process allows you to add transactions that were created outside the wallet into the wallet's transaction history and make them available for wallet operations.

## Overview

The process involves several steps:
1. Setting up wallet configuration and establishing connection to storage.
2. Retrieving BEEF transaction data either from provided hex or fetching from blockchain API.
3. Creating internalization arguments with proper output mapping and payment remittance.
4. Submitting the internalization request to add the external transaction to wallet history.
5. Processing the response to confirm successful transaction internalization.

This approach enables wallets to track and manage transactions that originated outside the wallet system while maintaining full transaction history.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable constants:

- **`atomicBeefHex`**: BEEF hex data for the transaction - leave empty to fetch from API using TxID
- **`txID`**: The specific transaction ID to internalize from the blockchain
- **`DefaultOriginator`**: Domain identifier for the requesting application (default: `"example.com"`)

### Request Parameters

The `InternalizeActionArgs` structure supports the following options:

- **`Tx`**: The BEEF (Bitcoin Extended Exchange Format) data containing the complete transaction
- **`Outputs`**: Array of outputs to internalize with payment remittance configuration
- **`Description`**: Human-readable description of the internalization action

### Response Analysis

The service response contains:

- **`Accepted`**: Boolean indicating whether the internalization was successful
- Additional metadata about the internalized transaction

## Running the Example

To run this example:

```bash
go run ./examples/wallet_examples/internalize_action/internalize_action.go
```

## Expected Output

```text
🚀 STARTING: Internalize Action
============================================================

=== STEP ===
Alice is performing: Creating wallet and setting up environment
--------------------------------------------------
CreateWallet: 02ce33253bb3ebccf7a1a3afe38efa9a320342c89250ed4eeff08a39e1a65017d3

=== STEP ===
Alice is performing: Retrieving BEEF data for transaction
--------------------------------------------------

🔗 TRANSACTION:
   TxID: 15f47f2db5f26469c081e8d80d91a4b0f06e4a97abcc022b0b5163ac5f6cc0c8

=== STEP ===
Alice is performing: Internalizing transaction
--------------------------------------------------

 WALLET CALL: InternalizeAction
Args: {Tx:[1 0 190 239 1 254 132 158 25 0 12 2 253 207 10 2 200 192 108 95 172 99 81 11 43 2 204 171 151 74 110 240 176 164 145 13 216 232 129 192 105 100 242 181 45 127 244 21 253 206 10 0 172 5 86 94 87 157 140 66 87 49 61 144 206 123 234 117 74 164 26 221 129 122 6 22 169 25 156 132 168 157 39 51 1 253 102 5 0 114 149 139 238 156 81 209 167 81 23 89 239 108 115 170 3 160 83 55 73 232 135 192 101 20 80 76 70 106 24 95 194 1 253 178 2 0 77 16 107 117 155 118 11 66 59 5 190 142 83 183 204 212 77 177 207 140 57 253 96 158 231 12 49 107 10 41 100 223 1 253 88 1 0 53 33 178 9 104 90 230 79 90 127 65 252 252 93 72 127 225 176 22 46 229 163 17 98 12 37 47 111 72 113 74 209 1 173 0 10 241 93 234 67 157 18 211 51 13 198 91 95 172 138 142 120 109 128 201 244 232 193 13 236 145 128 124 47 160 133 56 1 87 0 14 90 157 8 138 188 198 191 181 127 106 235 127 18 164 251 230 63 160 116 119 190 247 143 168 127 150 116 102 195 116 212 1 42 0 131 200 32 119 114 251 133 134 7 16 83 232 85 175 151 58 44 206 35 45 69 214 169 14 58 212 3 1 90 0 58 209 1 20 0 7 76 214 158 114 109 31 123 159 127 31 48 31 112 30 239 61 211 108 190 198 84 221 247 238 137 125 37 103 251 194 212 1 11 0 98 14 127 155 216 72 217 18 58 173 115 231 178 139 5 232 48 234 247 215 24 143 51 34 215 155 34 86 147 64 38 249 1 4 0 187 176 202 198 164 132 172 148 247 116 176 199 149 250 159 17 111 130 81 231 205 254 123 55 73 56 184 128 101 99 243 131 1 3 0 167 7 78 90 161 231 255 197 117 79 183 98 199 133 58 220 32 168 12 37 250 214 173 96 221 191 128 182 216 173 2 228 1 0 0 156 149 109 88 26 129 28 141 69 168 27 113 111 187 156 67 73 196 190 1 31 36 196 77 35 106 73 59 36 202 95 66 1 1 0 0 0 1 34 255 174 17 230 98 194 9 184 204 94 204 227 18 175 66 91 6 244 70 104 214 103 187 139 9 252 4 241 226 86 83 1 0 0 0 107 72 48 69 2 33 0 186 195 206 160 129 108 44 136 99 182 165 32 123 239 156 34 54 113 108 88 20 4 72 217 128 36 31 133 23 5 135 47 2 32 26 145 34 132 226 84 231 111 51 250 185 184 46 181 119 146 25 80 233 9 30 223 78 121 217 166 208 4 83 162 58 78 65 33 2 49 199 46 242 41 83 77 64 208 138 245 185 165 134 182 25 208 178 238 42 206 40 116 51 156 156 188 196 167 146 129 192 255 255 255 255 2 1 0 0 0 0 0 0 0 25 118 169 20 212 48 101 75 80 69 154 160 78 48 140 7 218 244 135 17 133 239 220 48 136 172 13 0 0 0 0 0 0 0 25 118 169 20 205 94 167 6 90 66 50 154 87 75 30 183 175 159 187 202 138 148 228 75 136 172 0 0 0 0 1 0] Description:internalize transaction Labels:[] SeekPermission:<nil> Outputs:[{OutputIndex:0 Protocol:wallet payment PaymentRemittance:0xc0001190c0 InsertionRemittance:<nil>}]}
✅ Result: {Accepted:true}
✅ SUCCESS: Transaction internalized successfully
============================================================
🎉 COMPLETED: Internalize Action
```

## Integration Steps

To integrate transaction internalization into your application:

1. **Configure transaction data source** by providing either BEEF hex directly or transaction ID for API retrieval.
2. **Set up wallet connection** with appropriate storage and authentication settings.
3. **Retrieve BEEF transaction data** using the provided helper function or custom implementation.
4. **Create internalization arguments** with proper output mapping and payment remittance configuration.
5. **Submit internalization request** using the wallet's `InternalizeAction` method.
6. **Process response data** to confirm successful internalization and handle any errors.
7. **Update wallet state** to reflect the newly internalized transaction in wallet history.

## Additional Resources

- [Create Action Documentation](../create_action/create_action.md) - Create new wallet transactions
- [Internalize Action Example](./internalize_action.go) - Complete code example for internalizing transactions
- [List Actions Documentation](../list_actions/list_actions.md) - View wallet transaction history
- [List Outputs Documentation](../list_outputs/list_outputs.md) - View wallet transaction outputs
