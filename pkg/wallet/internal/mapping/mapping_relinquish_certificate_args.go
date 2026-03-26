package mapping

import (
	"encoding/base64"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func MapRelinquishRelinquishCertificateArgs(args sdk.RelinquishCertificateArgs) (wdk.RelinquishCertificateArgs, error) {
	if args.Certifier == nil {
		return wdk.RelinquishCertificateArgs{}, fmt.Errorf("certifier: must be a non nil value")
	}

	if len(args.SerialNumber) == 0 {
		return wdk.RelinquishCertificateArgs{}, fmt.Errorf("serial number: must be a non empty string")
	}

	if len(args.Type) == 0 {
		return wdk.RelinquishCertificateArgs{}, fmt.Errorf("certificate type: must be a non empty string")
	}

	return wdk.RelinquishCertificateArgs{
		Type:         primitives.Base64String(base64.StdEncoding.EncodeToString(args.Type[:])),
		SerialNumber: primitives.Base64String(base64.StdEncoding.EncodeToString(args.SerialNumber[:])),
		Certifier:    primitives.PubKeyHex(args.Certifier.ToDERHex()),
	}, nil
}
