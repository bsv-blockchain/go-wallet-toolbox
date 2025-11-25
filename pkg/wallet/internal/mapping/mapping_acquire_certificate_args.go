package mapping

import (
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/to"
)

func MapToCertifierCounterparty(s primitives.PubKeyHex) (sdk.Counterparty, error) {
	pubKey, err := ec.PublicKeyFromString(to.String(s))
	if err != nil {
		return sdk.Counterparty{}, fmt.Errorf("invalid certifier public key hex: %w", err)
	}

	return sdk.Counterparty{
		Type:         sdk.CounterpartyTypeOther,
		Counterparty: pubKey,
	}, nil
}

func MapToFieldsForEncryption(m map[string]string) (map[sdk.CertificateFieldNameUnder50Bytes]string, error) {
	const (
		minLength = 1
		maxLength = 50
	)

	out := make(map[sdk.CertificateFieldNameUnder50Bytes]string, len(m))
	for key, val := range m {
		if len(key) < minLength || len(key) > maxLength {
			return nil, fmt.Errorf("invalid field name %q: must be between 1 and 50 bytes", key)
		}
		out[sdk.CertificateFieldNameUnder50Bytes(key)] = val
	}

	return out, nil
}
