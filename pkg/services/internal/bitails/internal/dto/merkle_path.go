package dto

// ProofResponse defines the structure of the response from Bitails for TSC proofs.
type ProofResponse struct {
	Index  int      `json:"index"`
	TxOrId string   `json:"txOrId"`
	Target string   `json:"target"`
	Nodes  []string `json:"nodes"`
}

// FetchInfoResponse is the structure for the response from Bitails when fetching transaction info.
type FetchInfoResponse struct {
	TxID        string `json:"txid"`
	BlockHash   string `json:"blockhash"`
	BlockHeight int64  `json:"blockheight"`
}
