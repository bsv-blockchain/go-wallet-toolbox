package testservices

type headerByHeightDTO struct {
	Hash              string  `json:"hash"`
	Confirmations     int     `json:"confirmations"`
	Size              int     `json:"size"`
	Height            uint32  `json:"height"`
	Version           uint32  `json:"version"`
	VersionHex        string  `json:"versionHex"`
	MerkleRoot        string  `json:"merkleroot"`
	Time              uint64  `json:"time"`
	MedianTime        uint64  `json:"mediantime"`
	Nonce             uint32  `json:"nonce"`
	Bits              string  `json:"bits"`
	Difficulty        float64 `json:"difficulty"`
	ChainWork         string  `json:"chainwork"`
	PreviousBlockHash string  `json:"previousblockhash"`
	NextBlockHash     string  `json:"nextblockhash"`
	NTx               int     `json:"nTx"`
	NumTx             int     `json:"num_tx"`
}

type blockHeaderDTO struct {
	Hash              string `json:"hash"`
	Height            uint   `json:"height"`
	Version           uint32 `json:"version"`
	MerkleRoot        string `json:"merkleroot"`
	Time              uint32 `json:"time"`
	Nonce             uint32 `json:"nonce"`
	Bits              string `json:"bits"`
	PreviousBlockHash string `json:"previousblockhash"`
}
