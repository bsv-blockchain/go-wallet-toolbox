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
	ciphertext = []byte{} //example []byte{220, 119, 136, 203, 17, 165, 76, 206, 75, 228, 144, 225, 235, 47, 193, 218, 155, 164, 179, 233, 45, 112, 160, 238, 33, 21, 110, 175, 176, 161, 88, 157, 37, 181, 228, 183, 194, 110, 216, 84, 109, 233, 220, 130, 43, 252, 193, 241, 151, 47, 58, 62, 246, 139, 62, 117, 44, 213, 191, 45, 130} 
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
	show.Info("DecryptArgs", args)
	show.Separator()

	decrypted, err := aliceWallet.Decrypt(ctx, args, DefaultOriginator)
	if err != nil {
		panic(fmt.Errorf("failed to decrypt: %w", err))
	}

	show.Info("Decrypted", string(decrypted.Plaintext))
	show.ProcessComplete("Decrypt")
}
