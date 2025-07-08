package fixtures

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func DefaultValidRelinquishCertificateArgs() *wdk.RelinquishCertificateArgs {
	return &wdk.RelinquishCertificateArgs{
		Type:         TypeField,
		SerialNumber: SerialNumber,
		Certifier:    Certifier,
	}
}
