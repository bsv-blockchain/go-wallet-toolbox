package wdk

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
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

// TableCertificateFieldSlice represents a slice of TableCertificateField items.
type TableCertificateFieldSlice []TableCertificateField

// ParseToTableCertificateFieldSlice converts a map of certificate fields into a slice of TableCertificateField pointers.
// Each entry in the `fields` map becomes a TableCertificateField, populated with the user ID, field name, field value,
// and the corresponding master key from `keyringForSubject`.
func ParseToTableCertificateFieldSlice(userID int, fields map[string]string, keyringForSubject map[string]string) []*TableCertificateField {
	tableCertificateFields := make([]*TableCertificateField, 0, len(fields))
	for k, v := range fields {
		tableCertificateFields = append(tableCertificateFields, &TableCertificateField{
			CreatedAt:  time.Now(),
			UserID:     userID,
			FieldName:  k,
			FieldValue: v,
			MasterKey:  primitives.Base64String(keyringForSubject[k]),
		})
	}
	return tableCertificateFields
}
