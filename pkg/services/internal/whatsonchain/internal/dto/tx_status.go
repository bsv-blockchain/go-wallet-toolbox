package dto

type WocStatusRequest struct {
	Txids []string `json:"txids"`
}

type WocStatusItem struct {
	TxID          string  `json:"txid"`
	BlockHash     string  `json:"blockhash"`
	BlockHeight   int64   `json:"blockheight"`
	BlockTime     int64   `json:"blocktime"`
	Confirmations *int    `json:"confirmations,omitempty"`
	Error         *string `json:"error,omitempty"`
}

type WocStatusResponse []WocStatusItem
