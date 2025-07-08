package wdk

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// TableCertificate represents a certificate with JSON tags
type TableCertificate struct {
	CreatedAt          time.Time                 `json:"created_at"`
	UpdatedAt          time.Time                 `json:"updated_at"`
	CertificateID      uint                      `json:"certificateId"`
	UserID             int                       `json:"userId"`
	Type               primitives.Base64String   `json:"type"`
	SerialNumber       primitives.Base64String   `json:"serialNumber"`
	Certifier          primitives.PubKeyHex      `json:"certifier"`
	Subject            primitives.PubKeyHex      `json:"subject"`
	Verifier           *primitives.PubKeyHex     `json:"verifier,omitempty"`
	RevocationOutpoint primitives.OutpointString `json:"revocationOutpoint"`
	Signature          primitives.HexString      `json:"signature"`
	IsDeleted          bool                      `json:"isDeleted"`
}

// TableCertificateX extends TableCertificate with optional fields
type TableCertificateX struct {
	TableCertificate
	Fields []*TableCertificateField `json:"fields,omitempty"`
}
