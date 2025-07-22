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

type OutPointSlice []OutPoint

func (oo OutPointSlice) AreUnique() bool {
	m := make(map[string]bool)
	for _, o := range oo {
		k := fmt.Sprintf("%s_%d", o.TxID, o.Vout)
		if ok := m[k]; ok {
			return false
		}
		m[k] = true
	}
	return true
}

func (oo OutPointSlice) Vouts() []uint32 {
	out := make([]uint32, 0, len(oo))
	for _, o := range oo {
		out = append(out, o.Vout)
	}
	return out
}

func (oo OutPointSlice) TxIDs() []string {
	out := make([]string, 0, len(oo))
	for _, o := range oo {
		out = append(out, o.TxID)
	}
	return out
}
