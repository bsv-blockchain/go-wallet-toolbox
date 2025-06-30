package dto

import (
	"math/big"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

type TipResponse struct {
	Hash             string   `json:"hash"`
	Version          uint32   `json:"version"`
	PreviousBlock    string   `json:"prevBlockHash"`
	MerkleRoot       string   `json:"merkleRoot"`
	Timestamp        uint64   `json:"creationTimestamp"`
	DifficultyTarget uint32   `json:"difficultyTarget"`
	Nonce            uint32   `json:"nonce"`
	Work             *big.Int `json:"work"`
}

type TipStateResponse struct {
	Header    TipResponse `json:"header"`
	State     string      `json:"state"`
	ChainWork *big.Int    `json:"chainWork"`
	Height    uint        `json:"height"`
}

func (t *TipStateResponse) IsZero() bool { return *t == TipStateResponse{} }

func (t *TipStateResponse) ConvertToChainBlockHeader() *wdk.ChainBlockHeader {
	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
			Version:      t.Header.Version,
			PreviousHash: t.Header.PreviousBlock,
			MerkleRoot:   t.Header.MerkleRoot,
			Time:         t.Header.Timestamp,
			Nonce:        t.Header.Nonce,
		},
		Height: t.Height,
		Hash:   t.Header.Hash,
	}
}
