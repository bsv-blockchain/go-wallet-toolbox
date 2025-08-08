# Encrypt Message

This example demonstrates how to encrypt a message using a BSV wallet with the Go Wallet Toolbox SDK. It showcases the complete encryption process from wallet setup to encrypted message generation using protocol-based encryption.

## Overview

The process involves several steps:
1. Setting up wallet configuration and establishing connection to storage.
2. Configuring encryption parameters including protocol ID, key ID, and counterparty information.
3. Creating encryption arguments with plaintext message and encryption metadata.
4. Executing the encryption using the wallet's `Encrypt` method.
5. Processing and displaying the encrypted message result.

This approach ensures secure message encryption using wallet-based cryptographic operations with proper protocol identification.

## Code Walkthrough

### Configuration Parameters

The example uses the following configurable constants:

- **`keyID`**: The key identifier for the encryption key (default: `"key-id"`)
- **`originator`**: The originator domain or FQDN used to identify the source of the encryption request (default: `"example.com"`)
- **`protocolID`**: The protocol identifier for the encryption operation (default: `"encryption"`)
- **`plaintext`**: The message text to be encrypted (default: `"Hello, world!"`)

### Encryption Parameters

The `EncryptArgs` structure supports the following options:

- **`EncryptionArgs`**: Container for encryption metadata and configuration
  - **`ProtocolID`**: Protocol identification for the encryption scheme
  - **`KeyID`**: Specific key identifier for the encryption operation
  - **`Counterparty`**: Information about the counterparty in the encryption context
- **`Plaintext`**: The raw message data to be encrypted (as byte array)

### Encryption Process

The encryption follows this pattern:

1. **Input Validation**: Verify that plaintext is not empty
2. **Wallet Setup**: Create and initialize Alice's wallet instance
3. **Arguments Creation**: Configure encryption parameters with protocol and key information
4. **Message Conversion**: Convert plaintext string to byte array for encryption
5. **Encryption Execution**: Call wallet's `Encrypt` method with configured arguments
6. **Result Processing**: Handle and display the encrypted message output

### Response Analysis

The encryption response contains the encrypted message data that can be:

- **Stored securely**: Save encrypted data for later decryption
- **Transmitted safely**: Send encrypted message over insecure channels
- **Processed further**: Use as input for additional cryptographic operations

## Running the Example

To run this example:

```bash
go run ./examples/wallet_examples/encrypt/encrypt.go
```

## Expected Output

```text
🚀 STARTING: Encrypt
============================================================
CreateWallet: 0200d66e0a2139239c13fdbb99b60185884214670ac5531aadaff8c9e9272e3b57

=== STEP ===
Alice is performing: Encrypting
--------------------------------------------------
EncryptArgs: {EncryptionArgs:{ProtocolID:{SecurityLevel:0 Protocol:encryption} KeyID:key-id Counterparty:{Type:0 Counterparty:<nil>} Privileged:false PrivilegedReason: SeekPermission:false} Plaintext:[72 101 108 108 111 44 32 119 111 114 108 100 33]}
============================================================
Encrypted: &{Ciphertext:[220 119 136 203 17 165 76 206 75 228 144 225 235 47 193 218 155 164 179 233 45 112 160 238 33 21 110 175 176 161 88 157 37 181 228 183 194 110 216 84 109 233 220 130 43 252 193 241 151 47 58 62 246 139 62 117 44 213 191 45 130]}
============================================================
🎉 COMPLETED: Encrypt
```

## Integration Steps

To integrate message encryption into your application:

1. **Configure encryption parameters** including protocol ID, key ID, and originator information.
2. **Set up wallet connection** with appropriate storage and authentication settings.
3. **Prepare message data** by converting plaintext to byte array format.
4. **Create encryption arguments** with proper protocol identification and key specifications.
5. **Execute encryption operation** using the wallet's `Encrypt` method.
6. **Handle encrypted output** by processing the returned encrypted message data.
7. **Implement error handling** for encryption failures, invalid parameters, or key issues.

## Additional Resources

- [Encrypt Example](./encrypt.go) - Complete code example for message encryption
- [Create Action Documentation](../create_action/create_action.md) - Create wallet transactions
- [List Outputs Documentation](../list_outputs/list_outputs.md) - View wallet transaction outputs
