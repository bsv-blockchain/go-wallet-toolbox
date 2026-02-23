package fixtures

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

const (
	// TypeField is base64-encoded string (original: "exampleType")
	TypeField = "ZXhhbXBsZVR5cGU="

	// SerialNumber is base64-encoded string (original: "serial123")
	SerialNumber = "c2VyaWFsMTIz"

	// Certifier is pubKeyHex (33-byte compressed public key)
	Certifier = "02c123eabcdeff1234567890abcdef1234567890abcdef1234567890abcdef1234"

	// SubjectPubKey is pubKeyHex (33-byte compressed public key)
	SubjectPubKey = "02c123eabcdeff1234567890abcdef1234567890abcdef1234567890abcdef5678"

	// RevocationOutpoint is outpointString (format: txid:vout)
	RevocationOutpoint = "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890.0"

	// Signature is hexString (64-byte signature)
	Signature = "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
)

func DefaultInsertCertAuth(userID int, subject primitives.PubKeyHex) *wdk.TableCertificateX {
	return &wdk.TableCertificateX{
		TableCertificate: wdk.TableCertificate{
			UserID:             userID,
			Type:               TypeField,
			SerialNumber:       SerialNumber,
			Certifier:          Certifier,
			Subject:            subject,
			RevocationOutpoint: RevocationOutpoint,
			Signature:          Signature,
		},
		Fields: []*wdk.TableCertificateField{
			{
				UserID:     userID,
				FieldName:  "exampleField",
				FieldValue: "exampleValue",
				MasterKey:  "exampleMasterKey",
			},
		},
	}
}
