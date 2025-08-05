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

- **`DefaultKeyID`**: The key identifier for the encryption key (default: `"key-id"`)
- **`DefaultOriginator`**: The originator domain or FQDN used to identify the source of the encryption request (default: `"example.com"`)
- **`DefaultProtocolID`**: The protocol identifier for the encryption operation (default: `"encryption"`)
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

1. **Wallet Setup**: Create and initialize Alice's wallet instance
2. **Arguments Creation**: Configure encryption parameters with protocol and key information
3. **Message Conversion**: Convert plaintext string to byte array for encryption
4. **Encryption Execution**: Call wallet's `Encrypt` method with configured arguments
5. **Result Processing**: Handle and display the encrypted message output

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
Config file not found, generating new configuration: examples/examples-config.yaml
Generated new configuration file: examples/examples-config.yaml
CreateWallet: 02c840e94d9547d2371fa503c60f8f6d8356b35f4e9ab33d79950fdb4cc9e2b014

=== STEP ===
Alice is performing: Encrypting
--------------------------------------------------
Encrypted: &{Ciphertext:[112 59 65 61 125 51 231 179 217 78 200 193 84 191 10 66 32 106 183 73 35 227 32 124 118 10 150 215 86 95 173 216 155 52 202 173 106 199 49 124 176 150 22 143 49 195 32 222 146 224 164 107 84 131 79 35 119 68 13 224 85]}
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
