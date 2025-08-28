package methods

import (
	"context"
	"encoding/base64"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/constants"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/utils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/go-softwarelab/common/pkg/to"
)

const (
	balanceListLimit = uint32(100)
)

// DeriveAddress returns a faucet BRC-29 address
func DeriveAddress(faucetKeyHex string, network defs.BSVNetwork) (string, error) {
	priv, err := ec.PrivateKeyFromHex(faucetKeyHex)
	if err != nil {
		return "", err
	}

	identityKey := priv.PubKey()

	derivationPrefixBytes, err := utils.BytesFromBase64(constants.DefaultBase64Prefix)
	if err != nil {
		return "", err
	}
	derivationSuffixBytes, err := utils.BytesFromBase64(constants.DefaultBase64Suffix)
	if err != nil {
		return "", err
	}

	keyID := brc29.KeyID{
		DerivationPrefix: base64.StdEncoding.EncodeToString(derivationPrefixBytes),
		DerivationSuffix: base64.StdEncoding.EncodeToString(derivationSuffixBytes),
	}

	var addr *script.Address
	if network == defs.NetworkMainnet {
		addr, err = brc29.Address(priv, keyID, identityKey, brc29.WithMainNet())
	} else {
		addr, err = brc29.Address(priv, keyID, identityKey, brc29.WithTestNet())
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

		offset += uint32(len(outputs.Outputs))
		if len(outputs.Outputs) < int(balanceListLimit) {
			break
		}
	}

	return balance, nil
}
