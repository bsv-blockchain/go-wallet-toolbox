# Faucet – Internalize

In this section, we’ll finish the example by **internalizing** a Testnet transaction on your local Wallet Toolbox server.
<br>**Note**:💡 This example is created as a helper to get funds into the wallet so you can use other examples.

## 0. Prerequisite: Transaction ID from the faucet.
In the [previous step](../faucet_address/README) you received a **txid** from a public Testnet faucet. If you have not completed the Faucet Address step yet, please refer [here](../faucet_address/README.md) before continuing.

## 1. Copy the TxID
Open `faucet_internalize.go` and add to txID value:

```go
// The txID is the transaction ID of the transaction to internalize
// Pass in your txID from the faucet_address example
var txID = "" //example: 15f47f2db5f26469c081e8d80d91a4b0f06e4a97abcc022b0b5163ac5f6cc0c8
```

A helper function—[`woc_get_beef_from_txid`](../../internal/utils/woc_get_beef_from_txid.go)—retrieves the full transaction **AtomicBEEF** hexadecimal ([BRC-62 spec](https://github.com/bitcoin-sv/BRCs/blob/master/transactions/0095.md)).  
That transaction is then passed to `InternalizeAction`, crediting Alice’s wallet.

## 2. Internalize the Transaction
Run the example:

```bash
go run ./examples/wallet_examples/faucet_internalize/faucet_internalize.go
```

Typical output:

```text
🚀 STARTING: Faucet Transaction Internalization
============================================================

=== STEP ===
Alice is performing: Creating wallet and setting up environment
--------------------------------------------------
CreateWallet: 03d2276c31630d6614f65c6634f40c0735a822d3501cc403ff459200971f747970

=== STEP ===
Alice is performing: Retrieving BEEF data for transaction
--------------------------------------------------

🔗 TRANSACTION:
   TxID: 15f47f2db5f26469c081e8d80d91a4b0f06e4a97abcc022b0b5163ac5f6cc0c8

=== STEP ===
Alice is performing: Internalizing transaction from faucet
--------------------------------------------------

 WALLET CALL: InternalizeAction
Args: {Tx:[1 1 1 1 200 192 108 95 172 99 81 11 43 2 204 171 151 74 110 240 176 164 145 13 216 232 129 192 105 100 242 181 45 127 244 21 2 0 190 239 1 254 132 158 25 0 12 2 253 206 10 0 172 5 86 94 87 157 140 66 87 49 61 144 206 123 234 117 74 164 26 221 129 122 6 22 169 25 156 132 168 157 39 51 253 207 10 2 200 192 108 95 172 99 81 11 43 2 204 171 151 74 110 240 176 164 145 13 216 232 129 192 105 100 242 181 45 127 244 21 1 253 102 5 0 114 149 139 238 156 81 209 167 81 23 89 239 108 115 170 3 160 83 55 73 232 135 192 101 20 80 76 70 106 24 95 194 1 253 178 2 0 77 16 107 117 155 118 11 66 59 5 190 142 83 183 204 212 77 177 207 140 57 253 96 158 231 12 49 107 10 41 100 223 1 253 88 1 0 53 33 178 9 104 90 230 79 90 127 65 252 252 93 72 127 225 176 22 46 229 163 17 98 12 37 47 111 72 113 74 209 1 173 0 10 241 93 234 67 157 18 211 51 13 198 91 95 172 138 142 120 109 128 201 244 232 193 13 236 145 128 124 47 160 133 56 1 87 0 14 90 157 8 138 188 198 191 181 127 106 235 127 18 164 251 230 63 160 116 119 190 247 143 168 127 150 116 102 195 116 212 1 42 0 131 200 32 119 114 251 133 134 7 16 83 232 85 175 151 58 44 206 35 45 69 214 169 14 58 212 3 1 90 0 58 209 1 20 0 7 76 214 158 114 109 31 123 159 127 31 48 31 112 30 239 61 211 108 190 198 84 221 247 238 137 125 37 103 251 194 212 1 11 0 98 14 127 155 216 72 217 18 58 173 115 231 178 139 5 232 48 234 247 215 24 143 51 34 215 155 34 86 147 64 38 249 1 4 0 187 176 202 198 164 132 172 148 247 116 176 199 149 250 159 17 111 130 81 231 205 254 123 55 73 56 184 128 101 99 243 131 1 3 0 167 7 78 90 161 231 255 197 117 79 183 98 199 133 58 220 32 168 12 37 250 214 173 96 221 191 128 182 216 173 2 228 1 0 0 156 149 109 88 26 129 28 141 69 168 27 113 111 187 156 67 73 196 190 1 31 36 196 77 35 106 73 59 36 202 95 66 1 1 0 1 0 0 0 1 34 255 174 17 230 98 194 9 184 204 94 204 227 18 175 66 91 6 244 70 104 214 103 187 139 9 252 4 241 226 86 83 1 0 0 0 107 72 48 69 2 33 0 186 195 206 160 129 108 44 136 99 182 165 32 123 239 156 34 54 113 108 88 20 4 72 217 128 36 31 133 23 5 135 47 2 32 26 145 34 132 226 84 231 111 51 250 185 184 46 181 119 146 25 80 233 9 30 223 78 121 217 166 208 4 83 162 58 78 65 33 2 49 199 46 242 41 83 77 64 208 138 245 185 165 134 182 25 208 178 238 42 206 40 116 51 156 156 188 196 167 146 129 192 255 255 255 255 2 1 0 0 0 0 0 0 0 25 118 169 20 212 48 101 75 80 69 154 160 78 48 140 7 218 244 135 17 133 239 220 48 136 172 13 0 0 0 0 0 0 0 25 118 169 20 205 94 167 6 90 66 50 154 87 75 30 183 175 159 187 202 138 148 228 75 136 172 0 0 0 0] Description:internalize from faucet Labels:[] SeekPermission:<nil> Outputs:[{OutputIndex:0 Protocol:wallet payment PaymentRemittance:0xc0003d0500 InsertionRemittance:<nil>}]}
✅ Result: {Accepted:true}
✅ SUCCESS: Transaction internalized successfully
============================================================
🎉 COMPLETED: Faucet Transaction Internalization
```

If successful, you’ll find a new row in `storage.sqlite` under Alice’s identity key.

---

### What you just did

1. **Fetched** raw transaction data from its transaction ID.  
2. **Internalized** that transaction via `InternalizeAction`.  
3. **Verified** the result in your local database.

You can now continue with other examples [provided](../../README.md).
