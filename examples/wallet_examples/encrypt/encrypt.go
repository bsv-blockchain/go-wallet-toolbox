package main

import (
	"context"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

var (
	// keyID is the key ID for the encryption key.
	keyID = "key-id"

	// originator specifies the originator domain or FQDN used to identify the source of the encryption request.
	// NOTE: Replace "example.com" with the actual originator domain or FQDN in real usage.
	originator = "example.com"

	// protocolID is the default protocol ID for the encryption.
	protocolID = "encryption"

	// plaintext is the text that will be encrypted.
	plaintext = "Hello, world!"
)

// This example shows how to encrypt a message using the go-sdk wallet.
// It creates a new wallet for Alice, encrypts a message, and prints the encrypted message.
func main() {
	show.ProcessStart("Encrypt")
	ctx := context.Background()

	if plaintext == "" {
		panic("plaintext cannot be empty")
	}

	alice := example_setup.CreateAlice()

	aliceWallet, cleanup := alice.CreateWallet(ctx)
	defer cleanup()
	show.Step("Alice", "Encrypting")

	args := sdk.EncryptArgs{
		EncryptionArgs: sdk.EncryptionArgs{
			ProtocolID:   sdk.Protocol{Protocol: protocolID},
			KeyID:        keyID,
			Counterparty: sdk.Counterparty{},
		},
		Plaintext: []byte(plaintext),
	}
	show.Info("EncryptArgs", args)
	show.Separator()

	encrypted, err := aliceWallet.Encrypt(ctx, args, originator)
	if err != nil {
		panic(fmt.Errorf("failed to encrypt: %w", err))
	}

	show.Info("Encrypted", encrypted)
	show.ProcessComplete("Encrypt")
}
