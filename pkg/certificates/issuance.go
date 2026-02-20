package certificates

import (
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

// ProtocolIssuanceRequest represents the certificate signing request sent to the certifier
// as part of the issuance protocol.
type ProtocolIssuanceRequest struct {
	Type          string            `json:"type"`
	Nonce         string            `json:"clientNonce"`
	Fields        map[string]string `json:"fields"`
	MasterKeyring map[string]string `json:"masterKeyring"`
}

// ProtocolIssuanceResponse represents the response from the certifier containing the signed certificate
// and server nonce for verification.
type ProtocolIssuanceResponse struct {
	Protocol    string       `json:"protocol"`
	Certificate *Certificate `json:"certificate"`
	ServerNonce string       `json:"serverNonce"`
	Timestamp   string       `json:"timestamp"`
	Version     string       `json:"version"`
}

// Certificate represents a certificate as returned by the certifier in the issuance protocol response.
type Certificate struct {
	Type               string            `json:"type"`
	SerialNumber       string            `json:"serialNumber"`
	Subject            string            `json:"subject"`
	Certifier          string            `json:"certifier"`
	RevocationOutpoint string            `json:"revocationOutpoint"`
	Fields             map[string]string `json:"fields"`
	Signature          string            `json:"signature"`
}

// MapToCertificateFields converts a map of string fields to SDK certificate fields.
// It validates that each field name is between 1 and 50 bytes as required by the SDK.
func MapToCertificateFields(fields map[string]string) (map[sdk.CertificateFieldNameUnder50Bytes]sdk.StringBase64, error) {
	const (
		minLength = 1
		maxLength = 50
	)

	stringFields := make(map[sdk.CertificateFieldNameUnder50Bytes]sdk.StringBase64, len(fields))
	for k, v := range fields {
		if len(k) < minLength || len(k) > maxLength {
			return nil, fmt.Errorf("invalid field name %q: must be between 1 and 50 bytes", k)
		}
		stringFields[sdk.CertificateFieldNameUnder50Bytes(k)] = sdk.StringBase64(v)
	}

	return stringFields, nil
}
