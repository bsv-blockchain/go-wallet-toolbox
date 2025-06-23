package wallet

import (
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

// WIF represents a string holding private key in WIF format.
// To pass a string as WIF simply wrap it with WIF type.
type WIF string

// PrivateKey returns the private key from the WIF string.
func (w WIF) PrivateKey() (*ec.PrivateKey, error) {
	return ec.PrivateKeyFromWif(string(w)) //nolint:wrapcheck
}

// PrivateKeySource represents a source of wallet owner private key.
// Can be used with different types of sources:
//   - string: a private key in DER HEX format
//   - WIF: a private key in WIF format string
//   - *sdk.KeyDeriver: a key deriver that can be used to derive the private key
//   - *ec.PrivateKey: a private key object
type PrivateKeySource interface {
	string | WIF | *ec.PrivateKey | *sdk.KeyDeriver
}

func toKeyDeriver[KeySource PrivateKeySource](keySource KeySource) (*sdk.KeyDeriver, error) {
	switch k := any(keySource).(type) {
	case string:
		priv, err := ec.PrivateKeyFromHex(k)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key from string hex %q: %w", k, err)
		}
		return sdk.NewKeyDeriver(priv), nil
	case WIF:
		priv, err := k.PrivateKey()
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key from string containing WIF %q: %w", k, err)
		}
		return sdk.NewKeyDeriver(priv), nil
	case *ec.PrivateKey:
		if k == nil {
			return nil, fmt.Errorf("private key (%T) cannot be nil", k)
		}
		return sdk.NewKeyDeriver(k), nil
	case *sdk.KeyDeriver:
		if k == nil {
			return nil, fmt.Errorf("key deriver (%T) cannot be nil", k)
		}
		return k, nil
	default:
		return nil, fmt.Errorf("unexpected key source type: %T, ensure that all subtypes of key source are handled", k)
	}
}
