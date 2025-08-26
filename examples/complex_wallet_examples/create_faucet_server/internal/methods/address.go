package methods

import (
	"encoding/base64"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/constants"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/utils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
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
