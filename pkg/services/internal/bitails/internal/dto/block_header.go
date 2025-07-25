package dto

// BlockHeader represents a block header in the BSV blockchain.
type BlockHeader struct {
	Hash              string `json:"hash"`
	Height            uint   `json:"height"`
	Version           uint32 `json:"version"`
	MerkleRoot        string `json:"merkleroot"`
	Time              uint64 `json:"time"`
	Nonce             uint32 `json:"nonce"`
	Bits              string `json:"bits"`
	PreviousBlockHash string `json:"previousblockhash"`
}
