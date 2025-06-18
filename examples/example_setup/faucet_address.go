package example_setup

import (
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/examples/show"
	"github.com/4chain-ag/go-wallet-toolbox/examples/utils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/brc29"
)

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
