package fixtures

import (
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func DefaultValidListCertificatesArgs() *wdk.ListCertificatesArgs {
	return &wdk.ListCertificatesArgs{
		ListCertificatesArgsPartial: wdk.ListCertificatesArgsPartial{
			SerialNumber:       to.Ptr(primitives.Base64String(SerialNumber)),
			Subject:            to.Ptr(primitives.PubKeyHex(SubjectPubKey)),
			RevocationOutpoint: to.Ptr(primitives.OutpointString(RevocationOutpoint)),
			Signature:          to.Ptr(primitives.HexString(Signature)),
		},
		Certifiers: []primitives.PubKeyHex{Certifier},
		Types:      []primitives.Base64String{TypeField},
		Limit:      primitives.PositiveIntegerDefault10Max10000(4),
		Offset:     primitives.PositiveInteger(5),
	}
}
