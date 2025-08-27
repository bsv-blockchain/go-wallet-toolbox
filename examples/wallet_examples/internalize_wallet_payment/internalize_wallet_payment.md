# Internalize Wallet Payment

This example demonstrates how to internalize an external Atomic BEEF transaction into a BSV wallet using the Go Wallet Toolbox SDK. This process allows you to add transactions that were created outside the wallet into the wallet's transaction history and make them available for wallet operations.

## Overview

The process involves several steps:

1. Setting up wallet configuration and establishing connection to storage.
2. Decoding the provided Atomic BEEF transaction data and payment remittance parameters.
3. Creating internalization arguments with payment remittance configuration including derivation prefix/suffix.
4. Submitting the internalization request to add the external transaction to wallet history.
5. Processing the response to confirm successful transaction internalization.

Atomic BEEF transactions start with the prefix `01010101` followed by the subject transaction ID, ensuring that all included transaction data relates to validating a single transaction and its dependencies.

The transaction outputs that match the address derivation provided by the payment remittance values will be internalized to the wallet-tool box database.

## Code Walkthrough

### Configuration Parameters

The example uses the following required configurable constants:

- **`AtomicBeefHex`**: Atomic BEEF hex data for the transaction (required - must not be empty). Contains the `01010101` prefix, subject TXID, and transaction dependencies
- **`Prefix`**: Base64-encoded derivation prefix for payment remittance (required)
- **`Suffix`**: Base64-encoded derivation suffix for payment remittance (required)
- **`IdentityKey`**: Hex-encoded sender identity key for payment remittance (required)
- **`Originator`**: Domain identifier for the requesting application (default: `"example.com"`)

### Request Parameters

The `InternalizeActionArgs` structure supports the following options:

- **`Tx`**: The decoded Atomic BEEF (BRC-95) data containing the subject transaction and its dependencies
- **`Outputs`**: Array of `InternalizeOutput` with output index, protocol, and payment remittance configuration
- **`Description`**: Human-readable description of the internalization action

The Atomic BEEF format ensures that:
- All included transactions relate to a single subject transaction
- The structure starts with `01010101` prefix followed by the subject TXID
- Only necessary dependency transactions are included for validation

The payment remittance includes:
- **`DerivationPrefix`**: Decoded base64 derivation prefix
- **`DerivationSuffix`**: Decoded base64 derivation suffix  
- **`SenderIdentityKey`**: Public key parsed from the provided hex-encoded identity key

### Response Analysis

The service response contains:

- **`Accepted`**: Boolean indicating whether the internalization was successful
- Additional metadata about the internalized transaction

## Running the Example

To run this example:

```bash
go run examples/wallet_examples/internalize_wallet_payment/internalize_wallet_payment.go
```

## Expected Output

```text
🚀 STARTING: Internalize Wallet Payment
============================================================

=== STEP ===
Alice is performing: Creating wallet and setting up environment
--------------------------------------------------
CreateWallet: 03d397c0f79268471ab7c582b939f70e8ea06f63a3861f23425102f89c9ed043d0
Outputs matching to the derived address based on the payment remittance: [0]

=== STEP ===
Alice is performing: Internalizing transaction
--------------------------------------------------

 WALLET CALL: InternalizeAction
Args: {Tx:[1 1 1 1 201 132 168 190 77 91 55 224 69 173 113 138 248 15 2 17 31 168 253 76 147 131 34 233 16 55 165 39 80 203 72 99 2 0 190 239 1 254 205 204 25 0 1 2 0 0 163 129 67 121 22 123 145 193 228 245 164 126 129 175 253 84 108 227 134 183 173 94 37 83 133 151 194 0 235 16 104 120 1 2 201 132 168 190 77 91 55 224 69 173 113 138 248 15 2 17 31 168 253 76 147 131 34 233 16 55 165 39 80 203 72 99 1 1 0 1 0 0 0 1 79 34 110 230 197 231 94 165 82 130 25 201 233 138 211 114 252 181 205 60 154 195 0 209 205 37 104 3 112 144 61 208 27 0 0 0 107 72 48 69 2 33 0 226 104 45 223 221 129 21 227 62 40 194 13 240 41 50 157 41 144 58 240 119 224 231 11 145 98 76 182 12 198 84 81 2 32 18 200 99 187 42 111 191 218 138 14 36 140 146 8 21 82 240 127 107 184 218 1 10 174 8 74 199 229 122 104 191 90 65 33 2 146 172 219 87 199 136 193 232 200 60 219 10 232 242 62 7 145 57 186 123 161 188 207 103 179 22 83 199 175 18 196 180 255 255 255 255 1 64 134 1 0 0 0 0 0 25 118 169 20 82 193 123 230 178 66 18 252 122 37 186 25 32 236 110 98 25 31 201 129 136 172 0 0 0 0] Description:internalize transaction Labels:[internalize 6348cb5027a53710e92283934cfda81f11020ff88a71ad45e0375b4dbea884c9] SeekPermission:<nil> Outputs:[{OutputIndex:0 Protocol:wallet payment PaymentRemittance:0xc0002c8280 InsertionRemittance:<nil>}]}
✅ Result: {Accepted:true}
✅ SUCCESS: Transaction internalized successfully
============================================================
🎉 COMPLETED: Internalize Wallet Payment
```

**Note:** The example requires all four parameters (`AtomicBeefHex`, `Prefix`, `Suffix`, `IdentityKey`) to be provided before running, or it will panic with a validation error. The `AtomicBeefHex` must be a valid Atomic BEEF structure starting with `01010101`.

## Integration Steps

To integrate Atomic BEEF transaction internalization into your application:

1. **Prepare required parameters**: Ensure you have the Atomic BEEF hex data (with `01010101` prefix and subject TXID), base64-encoded derivation prefix, suffix, and hex-encoded identity key.
2. **Set up wallet connection** with appropriate storage and authentication settings.
3. **Decode parameters**: Convert base64 prefix/suffix to bytes, hex Atomic BEEF data to bytes, and parse the hex identity key to public key format.
4. **Create internalization arguments** with decoded Atomic BEEF transaction data and payment remittance configuration.
5. **Submit internalization request** using the wallet's `InternalizeAction` method.
6. **Process response data** to confirm successful internalization and handle any errors.

## Additional Resources

- [Create P2PKH Transaction Documentation](../create_p2pkh_tx/create_p2pkh_tx.md) - Create new wallet transactions
- [Internalize Wallet Payment Example](./internalize_wallet_payment.go) - Complete code example for internalizing transactions
- [List Actions Documentation](../list_actions/list_actions.md) - View wallet transaction history
- [List Outputs Documentation](../list_outputs/list_outputs.md) - View wallet transaction outputs
