package wdk

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"

// RelinquishCertificateArgs represents the arguments for relinquishing certificate
type RelinquishCertificateArgs struct {
	Type         primitives.Base64String `json:"type"`
	SerialNumber primitives.Base64String `json:"serialNumber"`
	Certifier    primitives.PubKeyHex    `json:"certifier"`
}

// ListCertificatesArgsPartial is
type ListCertificatesArgsPartial struct {
	SerialNumber       *primitives.Base64String   `json:"serialNumber"`
	Subject            *primitives.PubKeyHex      `json:"subject"`
	RevocationOutpoint *primitives.OutpointString `json:"revocationOutpoint"`
	Signature          *primitives.HexString      `json:"signature"`
}

// ListCertificatesArgs represents the arguments for listing certificates
type ListCertificatesArgs struct {
	ListCertificatesArgsPartial `json:"partial"`

	Certifiers []primitives.PubKeyHex                      `json:"certifiers"`
	Types      []primitives.Base64String                   `json:"types"`
	Limit      primitives.PositiveIntegerDefault10Max10000 `json:"limit"`
	Offset     primitives.PositiveInteger                  `json:"offset"`
}
