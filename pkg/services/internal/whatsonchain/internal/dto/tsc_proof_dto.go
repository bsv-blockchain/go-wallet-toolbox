package dto

type TscProof struct {
	Index  int      `json:"index"`
	Nodes  []string `json:"nodes"`
	Target string   `json:"target"` // block hash
	TxOrID string   `json:"txOrId"` // txid
}

type BlockHeaderResponse struct {
	Height     int    `json:"height"`
	MerkleRoot string `json:"merkleRoot"`
}
