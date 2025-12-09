package chaintracks

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"

func liveBlockHeaderToBlockHeaderDTO(header *models.LiveBlockHeader) *models.BlockHeader {
	if header == nil {
		return nil
	}

	return &models.BlockHeader{
		BaseBlockHeader: models.BaseBlockHeader{
			Version:      header.Version,
			PreviousHash: header.PreviousHash,
			MerkleRoot:   header.MerkleRoot,
			Time:         header.Time,
			Bits:         header.Bits,
			Nonce:        header.Nonce,
		},
		BlockHeaderInfo: models.BlockHeaderInfo{
			HeaderID:         header.HeaderID,
			PreviousHeaderID: header.PreviousHeaderID,
			Chainwork:        header.ChainWork,
			IsChainTip:       header.IsChainTip,
			IsActive:         header.IsActive,
		},
		Height: header.Height,
		Hash:   header.Hash,
	}
}
