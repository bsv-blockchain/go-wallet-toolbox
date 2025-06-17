package brc29

import (
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

func toIdentityKey[KeySource CounterpartyPublicKey](keySource KeySource) (*ec.PublicKey, error) {
	switch k := any(keySource).(type) {
	case string:
		pubKey, err := ec.PublicKeyFromString(k)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key from string: %w", err)
		}
		return pubKey, nil
	case *sdk.KeyDeriver:
		if k == nil {
			return nil, fmt.Errorf("key deriver cannot be nil")
		}
		return k.IdentityKey(), nil
	case *ec.PublicKey:
		if k == nil {
			return nil, fmt.Errorf("public key cannot be nil")
		}
		return k, nil
	default:
		return nil, fmt.Errorf("unexpected key source type: %T, ensure that all subtypes of key source are handled", k)
	}
}

func toKeyDeriver[KeySource CounterpartyPrivateKey](keySource KeySource) (*sdk.KeyDeriver, error) {
	switch k := any(keySource).(type) {
	case string:
		priv, err := ec.PrivateKeyFromHex(k)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key from hex: %w", err)
		}
		return sdk.NewKeyDeriver(priv), nil
	case WIF:
		priv, err := k.PrivateKey()
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key from WIF: %w", err)
		}
		return sdk.NewKeyDeriver(priv), nil
	case *ec.PrivateKey:
		if k == nil {
			return nil, fmt.Errorf("private key cannot be nil")
		}
		return sdk.NewKeyDeriver(k), nil
	case *sdk.KeyDeriver:
		if k == nil {
			return nil, fmt.Errorf("key deriver cannot be nil")
		}
		return k, nil
	default:
		return nil, fmt.Errorf("unexpected key source type: %T, ensure that all subtypes of key source are handled", k)
	}
}
