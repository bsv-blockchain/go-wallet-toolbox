package wdk

import (
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
)

// TableCertificateField represents a field related to a certificate
type TableCertificateField struct {
	CreatedAt     time.Time               `json:"created_at"`
	UpdatedAt     time.Time               `json:"updated_at"`
	UserID        int                     `json:"userId"`
	CertificateID uint                    `json:"certificateId"`
	FieldName     string                  `json:"fieldName"`
	FieldValue    string                  `json:"fieldValue"`
	MasterKey     primitives.Base64String `json:"masterKey"`
}
