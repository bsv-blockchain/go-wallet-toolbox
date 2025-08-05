package main

import (
	"context"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

var (
	// DefaultKeyID is the default key ID for the encryption key.
	DefaultKeyID = "key-id"

	// DefaultOriginator specifies the originator domain or FQDN used to identify the source of the output listing request.
	// NOTE: Replace "example.com" with the actual originator domain or FQDN in real usage.
	DefaultOriginator = "example.com"

	// DefaultProtocolID is the default protocol ID for the encryption.
	DefaultProtocolID = "encryption"

	// plaintext is the text that will be encrypted
	plaintext = "Hello, world!"
)


// This example shows how to encrypt a message using the go-sdk wallet.
// It creates a new wallet for Alice, encrypts a message, and prints the encrypted message.
func main() {
	show.ProcessStart("Encrypt")
	ctx := context.Background()

	alice := example_setup.CreateAlice()

	aliceWallet, cleanup := alice.CreateWallet(ctx)
	defer cleanup()

	show.Step("Alice", "Encrypting")

	args := sdk.EncryptArgs{
		EncryptionArgs: sdk.EncryptionArgs{
			ProtocolID:   sdk.Protocol{Protocol: DefaultProtocolID},
			KeyID:        DefaultKeyID,
			Counterparty: sdk.Counterparty{},
		},
		Plaintext: []byte(plaintext),
	}

	encrypted, err := aliceWallet.Encrypt(ctx, args, DefaultOriginator)

	if err != nil {
		panic(fmt.Errorf("failed to encrypt: %w", err))
	}

	show.Info("Encrypted", encrypted)
	show.ProcessComplete("Encrypt")
}
