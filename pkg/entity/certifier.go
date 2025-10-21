package entity

type Certifier struct {
	Certifier          string
	UserID             int
	Type               string
	Subject            string
	Verifier           string
	RevocationOutpoint string
	Signature          string
}

type CertifierReadSpecification struct {
	Certifier          *Comparable[string]
	UserID             *Comparable[int]
	Type               *Comparable[string]
	Subject            *Comparable[string]
	Verifier           *Comparable[string]
	RevocationOutpoint *Comparable[string]
	Signature          *Comparable[string]
}
