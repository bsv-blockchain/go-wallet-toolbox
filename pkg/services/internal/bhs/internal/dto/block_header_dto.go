package dto

import "github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"

type BlockHeader struct {
	Height        uint   `json:"height"`
	Hash          string `json:"hash"`
	Version       uint32 `json:"version"`
	MerkleRoot    string `json:"merkleRoot"`
	Timestamp     uint64 `json:"creationTimestamp"`
	Bits          uint64 `json:"bits"`
	Nonce         uint32 `json:"nonce"`
	PreviousBlock string `json:"prevBlockHash"`
}

func (b *BlockHeader) ConvertToChainBlockHeader() *wdk.ChainBlockHeader {
	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
			Version:      b.Version,
			PreviousHash: b.PreviousBlock,
			MerkleRoot:   b.MerkleRoot,
			Time:         b.Timestamp,
			Bits:         b.Bits,
			Nonce:        b.Nonce,
		},
		Height: b.Height,
		Hash:   b.Hash,
	}
}
