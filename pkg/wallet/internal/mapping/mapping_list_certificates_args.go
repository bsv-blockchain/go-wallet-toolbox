package mapping

import (
	"encoding/base64"

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
		types = append(types, primitives.Base64String(base64.StdEncoding.EncodeToString(certType[:])))
	}

	return certifiers, types
}
