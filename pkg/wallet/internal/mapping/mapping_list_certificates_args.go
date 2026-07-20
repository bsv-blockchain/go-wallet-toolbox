package mapping

import (
	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func MapListCertificatesArgs(args sdk.ListCertificatesArgs) ([]primitives.PubKeyHex, []primitives.Base64String) {
	certifiers := make([]primitives.PubKeyHex, 0, len(args.Certifiers))
	for _, cert := range args.Certifiers {
		certifiers = append(certifiers, primitives.PubKeyHex(cert.ToDERHex()))
	}

	types := make([]primitives.Base64String, 0, len(args.Types))
	for _, certType := range args.Types {
		// Trim trailing zero-pad so filter values match TS-ecosystem short base64 types in storage.
		types = append(types, primitives.Base64String(primitives.EncodeBytes32Base64([32]byte(certType))))
	}

	return certifiers, types
}
