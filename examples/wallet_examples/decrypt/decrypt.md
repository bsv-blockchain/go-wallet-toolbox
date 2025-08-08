# Decrypt Message

This example demonstrates how to decrypt an encrypted message using a BSV wallet with the Go Wallet Toolbox SDK. It showcases the complete decryption process from wallet setup to plaintext message recovery using protocol-based decryption.

## Overview

The process involves several steps:
1. Setting up wallet configuration and establishing connection to storage.
2. Configuring decryption parameters including protocol ID, key ID, and counterparty information.
3. Creating decryption arguments with encrypted ciphertext and decryption metadata.
4. Executing the decryption using the wallet's `Decrypt` method.
5. Processing and displaying the decrypted plaintext message result.

This approach ensures secure message decryption using wallet-based cryptographic operations with proper protocol identification and key management.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable constants:

- **`keyID`**: The key identifier for the decryption key (default: `"key-id"`)
- **`originator`**: The originator domain or FQDN used to identify the source of the decryption request (default: `"example.com"`)
- **`protocolID`**: The protocol identifier for the decryption operation (default: `"encryption"`)
- **`ciphertext`**: The encrypted message data to be decrypted.

### Decryption Parameters

The `DecryptArgs` structure supports the following options:

- **`EncryptionArgs`**: Container for decryption metadata and configuration
  - **`ProtocolID`**: Protocol identification for the decryption scheme
  - **`KeyID`**: Specific key identifier for the decryption operation
  - **`Counterparty`**: Information about the counterparty in the decryption context
- **`Ciphertext`**: The encrypted message data to be decrypted (as byte array)

### Decryption Process

The decryption follows this pattern:

1. **Input Validation**: Verify that ciphertext is not empty
2. **Wallet Setup**: Create and initialize Alice's wallet instance
3. **Arguments Creation**: Configure decryption parameters with protocol and key information
4. **Ciphertext Input**: Use predefined encrypted message data for decryption
5. **Decryption Execution**: Call wallet's `Decrypt` method with configured arguments
6. **Result Processing**: Extract and display the decrypted plaintext message

### Response Analysis

The decryption response contains:

- **`Plaintext`**: The decrypted message data as a byte array that can be converted to string
- **Additional metadata**: Information about the decryption operation and success status

## Running the Example

To run this example:

```bash
go run ./examples/wallet_examples/decrypt/decrypt.go
```

## Expected Output

```text
🚀 STARTING: Decrypt
============================================================
CreateWallet: 0200d66e0a2139239c13fdbb99b60185884214670ac5531aadaff8c9e9272e3b57

=== STEP ===
Alice is performing: Decrypting
--------------------------------------------------
DecryptArgs: {EncryptionArgs:{ProtocolID:{SecurityLevel:0 Protocol:encryption} KeyID:key-id Counterparty:{Type:0 Counterparty:<nil>} Privileged:false PrivilegedReason: SeekPermission:false} Ciphertext:[220 119 136 203 17 165 76 206 75 228 144 225 235 47 193 218 155 164 179 233 45 112 160 238 33 21 110 175 176 161 88 157 37 181 228 183 194 110 216 84 109 233 220 130 43 252 193 241 151 47 58 62 246 139 62 117 44 213 191 45 130]}
============================================================
Decrypted: Hello, world!
============================================================
🎉 COMPLETED: Decrypt
```

## Integration Steps

To integrate message decryption into your application:

1. **Configure decryption parameters** including protocol ID, key ID, and originator information.
2. **Set up wallet connection** with appropriate storage and authentication settings.
3. **Prepare encrypted data** by obtaining the ciphertext from secure storage or transmission.
4. **Create decryption arguments** with proper protocol identification and key specifications.
5. **Execute decryption operation** using the wallet's `Decrypt` method.
6. **Handle decrypted output** by processing the returned plaintext message data.
7. **Implement error handling** for decryption failures, invalid ciphertext, or key mismatches.

## Additional Resources

- [Decrypt Example](./decrypt.go) - Complete code example for message decryption
- [Encrypt Documentation](../encrypt/encrypt.md) - Encrypt messages using wallet encryption
- [Create Action Documentation](../create_action/create_action.md) - Create wallet transactions
- [List Outputs Documentation](../list_outputs/list_outputs.md) - View wallet transaction outputs
