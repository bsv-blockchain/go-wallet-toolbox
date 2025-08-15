package arc

import "time"

// TXInfo is the struct that represents the transaction information from ARC
type TXInfo struct {
	BlockHash    string    `json:"blockHash"`
	BlockHeight  uint32    `json:"blockHeight"`
	CompetingTxs []string  `json:"competingTxs"`
	ExtraInfo    string    `json:"extraInfo"`
	MerklePath   string    `json:"merklePath"`
	Timestamp    time.Time `json:"timestamp"`
	TXStatus     TXStatus  `json:"txStatus"`
	TxID         string    `json:"txid"`
}

// Found presents a convention to indicate that the transaction is known by ARC
func (t *TXInfo) Found() bool {
	return t != nil
}
