package wdk

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"

// WalletCertificate is a wallet certificate object
type WalletCertificate struct {
	Type               primitives.Base64String                  `json:"type"`
	Subject            primitives.PubKeyHex                     `json:"subject"`
	SerialNumber       primitives.Base64String                  `json:"serialNumber"`
	Certifier          primitives.PubKeyHex                     `json:"certifier"`
	RevocationOutpoint primitives.OutpointString                `json:"revocationOutpoint"`
	Signature          primitives.HexString                     `json:"signature"`
	Fields             map[primitives.StringUnder50Bytes]string `json:"fields"`
}

// ListCertificatesResult is a response for ListCertificates action
type ListCertificatesResult struct {
	TotalCertificates primitives.PositiveInteger `json:"totalCertificates"`
	Certificates      []*CertificateResult       `json:"certificates"`
}

// CertificateResult is a response with WalletCertificate
// extended with keyring and verifier
type CertificateResult struct {
	WalletCertificate
	Keyring  map[primitives.StringUnder50Bytes]primitives.Base64String `json:"keyring"`
	Verifier string                                                    `json:"verifier"`
}
