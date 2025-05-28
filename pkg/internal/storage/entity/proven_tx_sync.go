package entity

type ProvenTxToSync struct {
	TxID     string
	Attempts uint64
}

type ProvenTxAsMined struct {
	TxID        string
	BlockHeight uint32
	MerklePath  []byte
	MerkleRoot  string
	BlockHash   string
}
