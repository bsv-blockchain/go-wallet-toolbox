package methods

import (
	"context"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/constants"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

const (
	balanceListLimit = uint32(100)
)

// DeriveAddress returns a faucet BRC-29 address
func DeriveAddress(faucetPrivateKey *ec.PrivateKey, network defs.BSVNetwork) (string, error) {
	_, identityKey := sdk.AnyoneKey()

	keyID := brc29.KeyID{
		DerivationPrefix: constants.FaucetAddressKeyIDPrefix,
		DerivationSuffix: constants.FaucetAddressKeyIDSuffix,
	}

	var addr *script.Address
	var err error
	if network == defs.NetworkMainnet {
		addr, err = brc29.AddressForSelf(identityKey, keyID, faucetPrivateKey, brc29.WithMainNet())
	} else {
		addr, err = brc29.AddressForSelf(identityKey, keyID, faucetPrivateKey, brc29.WithTestNet())
	}
	if err != nil {
		return "", err
	}

	return addr.AddressString, nil
}

// ComputeBalance pages through wallet outputs and returns the total satoshis.
func ComputeBalance(ctx context.Context, w sdk.Interface, basket string) (uint64, error) {
	var balance uint64
	var offset uint32

	for {
		args := sdk.ListOutputsArgs{
			Basket: basket,
			Limit:  to.Ptr(balanceListLimit),
			Offset: &offset,
		}

		outputs, err := w.ListOutputs(ctx, args, "")
		if err != nil {
			return 0, err
		}

		for _, output := range outputs.Outputs {
			balance += output.Satoshis
		}

		offset += uint32(len(outputs.Outputs)) //nolint:gosec // safe: output count fits in uint32
		if len(outputs.Outputs) < int(balanceListLimit) {
			break
		}
	}

	return balance, nil
}
