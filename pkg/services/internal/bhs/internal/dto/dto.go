package dto

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bitcoin-sv/block-headers-service/transports/http/endpoints/api/tips"
)

type ExtendedTipStateResponse tips.TipStateResponse

func (e *ExtendedTipStateResponse) IsZero() bool {
	return *e == ExtendedTipStateResponse{}
}

func (e *ExtendedTipStateResponse) ConvertToChainBlockHeader() *wdk.ChainBlockHeader {
	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
			Version:      uint32(e.Header.Version), //nolint:gosec
			PreviousHash: e.Header.PreviousBlock,
			MerkleRoot:   e.Header.MerkleRoot,
			Time:         uint64(e.Header.Timestamp),
			Nonce:        e.Header.Nonce,
		},
		Height: uint(e.Height), //nolint:gosec
		Hash:   e.Header.Hash,
	}
}
