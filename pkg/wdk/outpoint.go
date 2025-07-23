package wdk

import "fmt"

// OutPoint identifies a unique transaction output by its txid and index vout
type OutPoint struct {
	// TxID Transaction double sha256 hash as big endian hex string
	TxID string
	// Vout Zero based output index within the transaction
	Vout uint32
}

func (o OutPoint) String() string {
	return fmt.Sprintf("%s.%d", o.TxID, o.Vout)
}
