package models

type ProvenTx struct {
	Timestamps

	ProvenTxID uint    `gorm:"column:provenTxId;primaryKey;autoIncrement"`
	TxID       string  `gorm:"column:txid;type:varchar(64);not null"`
	Height     *uint32 `gorm:"column:height"`
	Index      *uint64 `gorm:"column:index"`
	MerklePath []byte  `gorm:"column:merklePath"`
	RawTx      []byte  `gorm:"column:rawTx"`
	BlockHash  *string `gorm:"column:blockHash;type:varchar(64)"`
	MerkleRoot *string `gorm:"column:merkleRoot;type:varchar(64)"`
}

// HasMerklePath returns true if the MerklePath field contains data, indicating the presence of a Merkle proof.
func (p *ProvenTx) HasMerklePath() bool {
	return len(p.MerklePath) > 0
}

func (ProvenTx) TableName() string {
	return "bsv_proven_txs"
}
