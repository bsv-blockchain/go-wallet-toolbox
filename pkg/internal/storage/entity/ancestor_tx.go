package entity

// AncestorTx is one transaction carried inside a caller-supplied inputBEEF,
// normalised into the shape a known tx row stores.
//
// The BEEF a caller submits duplicates its whole ancestry into every descendant
// that spends it. Recording each ancestor once, as a row, makes that duplication
// unnecessary: the ancestry walk reads those rows directly.
//
// MerklePath is set only when the BEEF proved this ancestor. An unproven
// ancestor still gets a row - the raw tx alone is what the walk needs to reach
// its own parents.
type AncestorTx struct {
	TxID       string
	RawTx      []byte
	MerklePath []byte
}

// IsProven reports whether the submitted BEEF carried a proof for this ancestor.
func (a AncestorTx) IsProven() bool {
	return len(a.MerklePath) > 0
}
