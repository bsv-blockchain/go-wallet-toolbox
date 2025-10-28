package ingest

// WOCBlockHeaderDTO represents a block header as retrieved from the Whatsonchain API for a given block hash.
type WOCBlockHeaderDTO struct {
	Hash          string  `json:"hash"`
	Size          int     `json:"size"`
	Height        uint    `json:"height"`
	Version       uint32  `json:"version"`
	VersionHex    string  `json:"versionHex"`
	MerkleRoot    string  `json:"merkleroot"`
	Time          uint32  `json:"time"`
	MedianTime    uint32  `json:"mediantime"`
	Nonce         uint32  `json:"nonce"`
	Bits          string  `json:"bits"`
	Difficulty    float64 `json:"difficulty"`
	Chainwork     string  `json:"chainwork"`
	PrevBlock     string  `json:"previousblockhash,omitempty"`
	NextBlock     string  `json:"nextblockhash,omitempty"`
	Confirmations int     `json:"confirmations"`
}
