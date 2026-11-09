package certifier

import (
	"context"
	"errors"
	"time"
)

// ErrIssuedCertificateNotFound is returned by CertificateStash lookups when no record matches.
var ErrIssuedCertificateNotFound = errors.New("issued certificate not found")

// IssuedCertificate is the certifier's own record of a certificate it signed.
type IssuedCertificate struct {
	ID                 uint
	Type               string
	SerialNumber       string
	Subject            string
	Certifier          string
	RevocationOutpoint string
	Signature          string
	Fields             []IssuedCertificateField
	// RevocationTx is the transaction (as returned by the wallet's CreateAction) that created
	// the revocation output. It is kept so the certifier can spend the output to revoke the certificate later.
	RevocationTx []byte
	CreatedAt    time.Time
}

// IssuedCertificateField is a single certificate field as it was signed.
type IssuedCertificateField struct {
	Name string
	// Value is the encrypted field value (base64) included in the signed certificate.
	Value string
	// MasterKey is the field's master keyring entry (base64) sent by the subject, encrypted for the certifier.
	MasterKey string
}

// CertificateStash persists certificates issued by the certifier server.
// It is kept separate from the certifier wallet's storage.
type CertificateStash interface {
	// Save stores the certificate together with its fields. Either everything is stored or nothing is.
	Save(ctx context.Context, cert *IssuedCertificate) error
	// FindBySerialNumber returns the certificate with the given serial number or ErrIssuedCertificateNotFound.
	FindBySerialNumber(ctx context.Context, serialNumber string) (*IssuedCertificate, error)
	// FindByRevocationOutpoint returns the certificate with the given revocation outpoint or ErrIssuedCertificateNotFound.
	FindByRevocationOutpoint(ctx context.Context, outpoint string) (*IssuedCertificate, error)
	// FindBySubject returns all certificates issued to the given subject identity key.
	FindBySubject(ctx context.Context, subject string) ([]*IssuedCertificate, error)
}
