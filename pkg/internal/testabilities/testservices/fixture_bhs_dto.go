package testservices

type longestChainTipResponse struct {
	Height        uint   `json:"height"`
	Hash          string `json:"hash"`
	Version       uint32 `json:"version"`
	MerkleRoot    string `json:"merkleRoot"`
	Timestamp     uint32 `json:"creationTimestamp"`
	Bits          uint32 `json:"bits"`
	Nonce         uint32 `json:"nonce"`
	PreviousBlock string `json:"prevBlockHash"`
}

type headerByHeightResponse struct {
	Hash             string `json:"hash"`
	Version          int32  `json:"version"`
	PreviousBlock    string `json:"prevBlockHash"`
	MerkleRoot       string `json:"merkleRoot"`
	Timestamp        uint32 `json:"creationTimestamp"`
	DifficultyTarget uint32 `json:"difficultyTarget"`
	Nonce            uint32 `json:"nonce"`
	Work             string `json:"work"`
}

func defaultHeaderByHeightResponse() *headerByHeightResponse {
	return &headerByHeightResponse{
		Version:          1,
		PreviousBlock:    "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea226",
		MerkleRoot:       "10f072e631081ad6bcddeabb90bc34d787fe7d7116fe0298ff26c50c5e21bfea",
		Timestamp:        1233046715,
		DifficultyTarget: 486604799,
		Nonce:            2999858432,
		Work:             "4295032833",
		Hash:             "00000000dfd5d65c9d8561b4b8f60a63018fe3933ecb131fb37f905f87da951a",
	}
}

func defaultLongestChainTipResponse() *longestChainTipResponse {
	return &longestChainTipResponse{
		Height:        800000,
		Hash:          "0000000000000000000a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e",
		Version:       536870912,
		MerkleRoot:    "3a4b5c6d7e8f90123456789abcdef0123456789abcdef0123456789abcdef01",
		Timestamp:     1719427200,
		Bits:          386136923,
		Nonce:         2083236893,
		PreviousBlock: "00000000000000000008e7b8c6d5f4e3d2c1b0a987654321fedcba9876543210",
	}
}
