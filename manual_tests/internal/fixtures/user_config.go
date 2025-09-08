package fixtures

import (
	"fmt"

	primitives "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
)

type UserConfig struct {
	Name    string `mapstructure:"name"`
	PrivKey string `mapstructure:"private_key"`
}

func (u UserConfig) Validate() error {
	if u.PrivKey == "" {
		return fmt.Errorf("private_key is required")
	}

	return nil
}

func (u UserConfig) IdentityKey() string {
	return u.PubKey()
}

func (u UserConfig) PubKey() string {
	return u.PublicKey().ToDERHex()
}

func (u UserConfig) PrivateKey() *primitives.PrivateKey {
	priv, err := primitives.PrivateKeyFromHex(u.PrivKey)
	if err != nil {
		panic(fmt.Errorf("failed to create private key for user %s: %w", u.Name, err))
	}
	return priv
}

func (u UserConfig) PublicKey() *primitives.PublicKey {
	priv, err := primitives.PrivateKeyFromHex(u.PrivKey)
	if err != nil {
		panic(fmt.Errorf("failed to create public key for user %s: %w", u.Name, err))
	}

	return priv.PubKey()
}

func (u UserConfig) Address(mainnet bool) *script.Address {
	address, err := script.NewAddressFromPublicKey(u.PublicKey(), mainnet)
	if err != nil {
		panic(fmt.Errorf("failed to create address for user %s: %w", u.Name, err))
	}
	return address
}
