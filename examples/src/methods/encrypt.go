package methods

import (
	"context"
	"log"

	"github.com/4chain-ag/go-wallet-toolbox/examples/src/core"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

func Encrypt(ctx context.Context, network defs.BSVNetwork, encryptArgs sdk.EncryptArgs, identityKey string) (sdk.EncryptResult, error) {
	wallet, err := core.MatchUser(ctx, network, identityKey)
	if err != nil {
		return sdk.EncryptResult{}, err
	}

	encrypted, err := wallet.Encrypt(ctx, encryptArgs, "originator")
	if err != nil {
		return sdk.EncryptResult{}, err
	}

	return *encrypted, nil
}

func EncryptHandler(text string, identityKey string) sdk.EncryptResult {
	ctx := context.Background()
	network := defs.NetworkTestnet
	encryptArgs := sdk.EncryptArgs{
		EncryptionArgs: sdk.EncryptionArgs{
			ProtocolID:   sdk.Protocol{Protocol: "encryption"},
			KeyID:        "test-key-1",
			Counterparty: sdk.Counterparty{},
		},
		Plaintext: []byte(text),
	}

	encryptResult, err := Encrypt(ctx, network, encryptArgs, identityKey)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	return encryptResult
}
