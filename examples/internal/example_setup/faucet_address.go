package example_setup

import (
	"encoding/base64"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/utils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
)

// FaucetAddress generates a BRC29 address for the given wallet
func FaucetAddress(wallet *Setup) {
	parts := utils.DerivationParts()

	keyID := brc29.KeyID{
		DerivationPrefix: base64.StdEncoding.EncodeToString(parts.DerivationPrefix),
		DerivationSuffix: base64.StdEncoding.EncodeToString(parts.DerivationSuffix),
	}

	address, err := brc29.AddressForSelf(
		parts.SenderIdentityKey,
		keyID,
		wallet.PrivateKey,
		brc29.WithTestNet(),
	)
	if err != nil {
		panic(fmt.Errorf("failed to generate BRC29 address: %w", err))
	}

	show.FaucetInstructions(address.AddressString)
}
