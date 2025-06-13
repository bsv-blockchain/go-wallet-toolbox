package methods

import (
	"context"
	"log"

	"github.com/4chain-ag/go-wallet-toolbox/examples/src/core"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

func Decrypt(ctx context.Context, network defs.BSVNetwork, decryptArgs sdk.DecryptArgs, identityKey string) (sdk.DecryptResult, error) {
	wallet, err := core.MatchUser(ctx, network, identityKey)
	if err != nil {
		return sdk.DecryptResult{}, err
	}

	decrypted, err := wallet.Decrypt(ctx, decryptArgs, "originator")
	if err != nil {
		return sdk.DecryptResult{}, err
	}

	return *decrypted, nil
}

func DecryptHandler(ciphertext []byte, identityKey string) sdk.DecryptResult {
	ctx := context.Background()
	network := defs.NetworkTestnet
	decryptArgs := sdk.DecryptArgs{
		EncryptionArgs: sdk.EncryptionArgs{
			ProtocolID:   sdk.Protocol{Protocol: "encryption"},
			KeyID:        "test-key-1",
			Counterparty: sdk.Counterparty{},
		},
		Ciphertext: ciphertext,
	}

	decryptResult, err := Decrypt(ctx, network, decryptArgs, identityKey)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	return decryptResult
}
