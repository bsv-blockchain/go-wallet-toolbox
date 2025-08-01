package dto

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// ScriptHistoryResponse represents the response from the script history endpoints
type ScriptHistoryResponse struct {
	History []ScriptHistoryItem `json:"history"`
	PgKey   string              `json:"pgkey,omitempty"`
	Error   string              `json:"error,omitempty"`
}

// ScriptHistoryItem represents a single entry in the script hash history response.
type ScriptHistoryItem struct {
	// TxID is the transaction ID associated with the script hash history entry.
	TxID string `json:"txid"`

	// Height is the block height at which the transaction was included (optional for unconfirmed)
	Height *int `json:"blockheight,omitempty"`
}

type BlockHeaderByHeightDTO struct {
	PreviousBlockHash string `json:"previousBlockHash"`
	Version           uint32 `json:"version"`
	MerkleRoot        string `json:"merkleroot"`
	Time              uint32 `json:"time"`
	Bits              uint32 `json:"bits"`
	Nonce             uint32 `json:"nonce"`
}

func (b *BlockHeaderByHeightDTO) ConvertToChainBaseBlockHeader() *wdk.ChainBaseBlockHeader {
	return &wdk.ChainBaseBlockHeader{
		Version:      b.Version,
		PreviousHash: b.PreviousBlockHash,
		MerkleRoot:   b.MerkleRoot,
		Time:         b.Time,
		Bits:         b.Bits,
		Nonce:        b.Nonce,
	}
}
