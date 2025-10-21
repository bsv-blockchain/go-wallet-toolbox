package entity

import "time"

// Certificate represents a digital wallet certificate including certifier, subject, serial number, and related fields.
type Certificate struct {
	ID                 uint
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Certifier          string
	SerialNumber       string
	UserID             int
	Type               string
	Subject            string
	Verifier           string
	RevocationOutpoint string
	Signature          string
	CertificateFields  []CertificateField
}

// CertificateReadSpecification defines filter criteria for querying certificates based on various optional comparable fields.
type CertificateReadSpecification struct {
	ID                 *uint
	Certifier          *Comparable[string]
	SerialNumber       *Comparable[string]
	UserID             *Comparable[int]
	Type               *Comparable[string]
	Subject            *Comparable[string]
	Verifier           *Comparable[string]
	RevocationOutpoint *Comparable[string]
	Signature          *Comparable[string]
}

// CertificateField represents a single named field associated with a certificate and its corresponding value and master key.
type CertificateField struct {
	CreatedAt time.Time
	UpdatedAt time.Time

	FieldName  string
	FieldValue string
	MasterKey  string
}
