package wdk

import (
	"encoding/base64"
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/to"
)

// WalletCertificateFieldMap represents a mapping of wallet certificate fields.
// The keys are constrained to primitives.StringUnder50Bytes (strings up to 50 bytes),
// and the values are regular strings.
type WalletCertificateFieldMap map[primitives.StringUnder50Bytes]string

// ToMap converts the WalletCertificateFieldMap to a standard map[string]string.
// This is useful for interoperability with functions or APIs that expect regular string maps.
// Returns a new map where each key and value is converted to a plain string.
func (m WalletCertificateFieldMap) ToMap() map[string]string {
	out := make(map[string]string, len(m))
	for key, val := range m {
		out[to.String(key)] = to.String(val)
	}
	return out
}

// ToFieldsForEncryption converts the WalletCertificateFieldMap into a map suitable for encryption,
// where each key is cast to sdk.CertificateFieldNameUnder50Bytes.
//
// It validates that all field names are between 1 and 50 characters long before conversion.
// If any key is invalid, the function returns an error and no fields are returned.
func (m WalletCertificateFieldMap) ToFieldsForEncryption() (map[sdk.CertificateFieldNameUnder50Bytes]string, error) {
	const (
		minLength = 1
		maxLength = 50
	)

	out := make(map[sdk.CertificateFieldNameUnder50Bytes]string, len(m))
	for key, val := range m {
		if len(key) < minLength || len(key) > maxLength {
			return nil, fmt.Errorf("invalid field name %q: must be between 1 and 50 characters", key)
		}
		out[sdk.CertificateFieldNameUnder50Bytes(key)] = val
	}

	return out, nil
}

// WalletCertificate is a wallet certificate object
type WalletCertificate struct {
	Type               primitives.Base64String   `json:"type"`
	Subject            primitives.PubKeyHex      `json:"subject"`
	SerialNumber       primitives.Base64String   `json:"serialNumber"`
	Certifier          primitives.PubKeyHex      `json:"certifier"`
	RevocationOutpoint primitives.OutpointString `json:"revocationOutpoint"`
	Signature          primitives.HexString      `json:"signature"`
	Fields             WalletCertificateFieldMap `json:"fields"`
}

// CertifierCounterparty converts the WalletCertificate's Certifier field into a wallet.Counterparty.
// It interprets the stored Certifier value (expected to be a hex-encoded private key)
// and derives its corresponding public key to construct a Counterparty object.
func (w *WalletCertificate) CertifierCounterparty() (sdk.Counterparty, error) {
	key, err := ec.PrivateKeyFromHex(to.String(w.Certifier))
	if err != nil {
		return sdk.Counterparty{}, fmt.Errorf("invalid certifier private key hex: %w", err)
	}

	return sdk.Counterparty{
		Type:         sdk.CounterpartyTypeOther,
		Counterparty: key.PubKey(),
	}, nil
}

// ToSDKCertificate converts a WalletCertificate to an sdk.Certificate.
// This involves parsing and converting the stored string fields into their respective SDK types,
// including public keys, serial numbers, certificate type, and revocation outpoint.
//
// Returns the constructed sdk.Certificate on success, or an error if any of the conversions fail.
func (w *WalletCertificate) ToSDKCertificate() (sdk.Certificate, error) {
	// Convert the Subject string to an EC public key
	subject, err := ec.PublicKeyFromString(string(w.Subject))
	if err != nil {
		return sdk.Certificate{}, fmt.Errorf("failed to create public key from subject string: %w", err)
	}

	// Convert the RevocationOutpoint string to a transaction.Outpoint
	revocationOutpoint, err := transaction.OutpointFromString(string(w.RevocationOutpoint))
	if err != nil {
		return sdk.Certificate{}, fmt.Errorf("failed to get revocation outpoint from string: %w", err)
	}

	// Parse the serial number from base64 to sdk.SerialNumber
	serial, err := parseSerialNumber(string(w.SerialNumber))
	if err != nil {
		return sdk.Certificate{}, fmt.Errorf("failed to convert stored serial number to sdk type: %w", err)
	}

	// Parse the certificate type from base64 to sdk.CertificateType
	certType, err := parseCertificationType(string(w.Type))
	if err != nil {
		return sdk.Certificate{}, fmt.Errorf("failed to convert stored certification type to sdk type: %w", err)
	}

	// Convert the Certifier string to an EC public key
	certifier, err := ec.PublicKeyFromString(string(w.Certifier))
	if err != nil {
		return sdk.Certificate{}, fmt.Errorf("failed to create certifier public key: %w", err)
	}

	// Parse the signature string to an EC Signature
	sig, err := parseSignature(w.Signature)
	if err != nil {
		return sdk.Certificate{}, fmt.Errorf("failed to convert signature to sdk type: %w", err)
	}

	// Construct and return the SDK certificate
	return sdk.Certificate{
		Type:               certType,
		SerialNumber:       serial,
		Subject:            subject,
		Certifier:          certifier,
		RevocationOutpoint: revocationOutpoint,
		Fields:             w.Fields.ToMap(),
		Signature:          sig,
	}, nil
}

// ListCertificatesResult is a response for ListCertificates action
type ListCertificatesResult struct {
	TotalCertificates primitives.PositiveInteger `json:"totalCertificates"`
	Certificates      []*CertificateResult       `json:"certificates"`
}

// First returns the first certificate in the list, or nil if the list is empty.
func (l *ListCertificatesResult) First() *CertificateResult {
	if len(l.Certificates) == 0 {
		return nil
	}
	return l.Certificates[0]
}

// HasNoCertificates returns true if the list contains no certificates.
func (l *ListCertificatesResult) HasNoCertificates() bool {
	return len(l.Certificates) == 0
}

// KeyringMap represents a mapping of keys in a wallet keyring.
// The keys are constrained to primitives.StringUnder50Bytes (strings up to 50 bytes),
// and the values are base64-encoded strings (primitives.Base64String).
type KeyringMap map[primitives.StringUnder50Bytes]primitives.Base64String

// IsEmpty checks whether the KeyringMap contains any entries.
// Returns true if the map has no elements, otherwise false.
func (m KeyringMap) IsEmpty() bool {
	return len(m) == 0
}

// ToMap converts the KeyringMap to a standard map[string]string.
// This is useful for interoperability with functions or APIs that expect regular string maps.
// Returns a new map where both keys and values are converted to plain strings.
func (m KeyringMap) ToMap() map[string]string {
	out := make(map[string]string, len(m))
	for key, val := range m {
		out[to.String(key)] = to.String(val)
	}
	return out
}

// CertificateResult is a response with WalletCertificate
// extended with keyring and verifier
type CertificateResult struct {
	WalletCertificate
	Keyring  KeyringMap     `json:"keyring"`
	Verifier VerifierString `json:"verifier"`
}

// VerifierString represents a string used as a verifier identifier or value.
type VerifierString string

// IsEmpty checks whether the VerifierString is empty.
// Returns true if the string has length 0, otherwise false.
func (s VerifierString) IsEmpty() bool {
	return len(s) == 0
}

// parseSignature converts a HexString into an EC signature.
// The input `s` is expected to be a concatenation of the R and S values in hex format (64 characters each).
// Returns a pointer to an ec.Signature containing the parsed R and S values.
// Otherwise error that indicates the given input string length is not correct.
func parseSignature(s primitives.HexString) (*ec.Signature, error) {
	if len(s) < 128 {
		return nil, fmt.Errorf("input is too short to contain both R and S values (64 hex chars each)")
	}

	rHex := s[:64]
	sHex := s[64:]

	return &ec.Signature{
		R: ec.FromHex(rHex.String()),
		S: ec.FromHex(sHex.String()),
	}, nil
}

// parseSerialNumber decodes a base64-encoded string into an sdk.SerialNumber.
// Returns the decoded SerialNumber, or an error if decoding fails.
func parseSerialNumber(s string) (sdk.SerialNumber, error) {
	serialBytes, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return sdk.SerialNumber{}, fmt.Errorf("failed to decode certificate serial number: %w", err)
	}

	var serial sdk.SerialNumber
	if len(serialBytes) != len(serial) {
		return sdk.SerialNumber{}, fmt.Errorf("serial bytes length: %d is not equal to sdk.SerialNumber bytes length: %d", len(serialBytes), len(serial))
	}

	copy(serial[:], serialBytes)

	return serial, nil
}

// parseCertificationType decodes a base64-encoded string into an sdk.CertificateType.
// Returns the decoded CertificateType, or an error if decoding fails.
func parseCertificationType(s string) (sdk.CertificateType, error) {
	certBytes, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return sdk.CertificateType{}, fmt.Errorf("failed to decode certificate type: %w", err)
	}

	var certType sdk.CertificateType
	if len(certType) != len(certBytes) {
		return sdk.CertificateType{}, fmt.Errorf("certificate type bytes length: %d is not equal to sdk.CertificateType bytes length: %d", len(certBytes), len(certType))
	}

	copy(certType[:], certBytes)

	return certType, nil
}
