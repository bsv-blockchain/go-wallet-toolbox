package wdk

import (
	"encoding/base64"
	"fmt"
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
// and the corresponding master key from `keyringForSubject`. Returns an error if any field name is missing in `keyringForSubject`.
func ParseToTableCertificateFieldSlice(userID int, fields map[string]string, keyringForSubject map[string]string) ([]*TableCertificateField, error) {
	tableCertificateFields := make([]*TableCertificateField, 0, len(fields))

	for name, value := range fields {
		masterKey, ok := keyringForSubject[name]
		if !ok {
			return nil, fmt.Errorf("keyringForSubject map doesn't contain the key with name: %s", name)
		}

		tableCertificateFields = append(tableCertificateFields, &TableCertificateField{
			UserID:     userID,
			FieldName:  name,
			FieldValue: value,
			MasterKey:  primitives.Base64String(base64.StdEncoding.EncodeToString([]byte(masterKey))),
		})
	}

	return tableCertificateFields, nil
}
