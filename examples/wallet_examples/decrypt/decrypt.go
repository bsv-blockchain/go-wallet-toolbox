package main

import (
	"context"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
)

var (
	// keyID is the key ID for the decryption key.
	keyID = "key-id"

	// DefaultOriginator specifies the originator domain or FQDN used to identify the source of the decryption request.
	// NOTE: Replace "example.com" with the actual originator domain or FQDN in real usage.
	DefaultOriginator = "example.com"

	// DefaultProtocolID is the default protocol ID for the decryption.
	DefaultProtocolID = "encryption"

	// ciphertext is the encrypted version of the plaintext
	ciphertext = []byte{} // replace with your ciphertext byte array
) 

// This example shows how to decrypt a message using the go-sdk wallet.
// It creates a new wallet for Alice, decrypts a message, and prints the decrypted message.
func main() {
	show.ProcessStart("Decrypt")
	ctx := context.Background()

	alice := example_setup.CreateAlice()

	aliceWallet, cleanup := alice.CreateWallet(ctx)
	defer cleanup()

	show.Step("Alice", "Decrypting")

	args := sdk.DecryptArgs{
		EncryptionArgs: sdk.EncryptionArgs{
			ProtocolID:   sdk.Protocol{Protocol: DefaultProtocolID},
			KeyID:        keyID,
			Counterparty: sdk.Counterparty{},
		},
		Ciphertext: ciphertext,
	}

	decrypted, err := aliceWallet.Decrypt(ctx, args, DefaultOriginator)
	if err != nil {
		panic(fmt.Errorf("failed to decrypt: %w", err))
	}

	show.Info("Decrypted", string(decrypted.Plaintext))
	show.ProcessComplete("Decrypt")
}
