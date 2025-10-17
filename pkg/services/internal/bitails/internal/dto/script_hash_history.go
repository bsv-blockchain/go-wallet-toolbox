package dto

import (
	"fmt"
	"strconv"

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

type BlockHeaderByHeight struct {
	PreviousBlockHash string `json:"previousBlockHash"`
	Version           uint32 `json:"version"`
	MerkleRoot        string `json:"merkleroot"`
	Time              uint32 `json:"time"`
	Bits              string `json:"bits"`
	Hash              string `json:"hash"`
	Nonce             uint32 `json:"nonce"`
}

func (b *BlockHeaderByHeight) IsZero() bool { return *b == BlockHeaderByHeight{} }

func (b *BlockHeaderByHeight) ConvertToChainBlockHeader() (*wdk.ChainBlockHeader, error) {
	bits, err := strconv.ParseUint(b.Bits, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid bits value %q: expected hex string convertible to uint32: %w", b.Bits, err)
	}

	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
			Version:      b.Version,
			PreviousHash: b.PreviousBlockHash,
			MerkleRoot:   b.MerkleRoot,
			Time:         b.Time,
			Bits:         uint32(bits),
			Nonce:        b.Nonce,
		},
		Hash: b.Hash,
	}, nil
}
