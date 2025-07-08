package example_setup

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/utils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
)

// FaucetAddress generates a BRC29 address for the given wallet
func FaucetAddress(wallet *Setup) {
	parts := utils.DerivationParts()

	keyID := brc29.KeyID{
		DerivationPrefix: parts.PaymentRemittance.DerivationPrefix,
		DerivationSuffix: parts.PaymentRemittance.DerivationSuffix,
	}

	address, err := brc29.Address(
		&wallet.PrivateKey,
		keyID,
		&wallet.IdentityKey,
		brc29.WithTestNet(),
	)
	if err != nil {
		panic(fmt.Errorf("failed to generate BRC29 address: %w", err))
	}

	show.FaucetInstructions(address.AddressString)

}
