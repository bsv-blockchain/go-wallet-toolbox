package gormstorage

import (
	dbmodels "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func mapLiveHeader(oneBack *dbmodels.ChaintracksLiveHeader) *models.LiveBlockHeader {
	return &models.LiveBlockHeader{
		ChainBlockHeader: wdk.ChainBlockHeader{
			ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
				Version:      oneBack.Version,
				PreviousHash: oneBack.PreviousHash,
				MerkleRoot:   oneBack.MerkleRoot,
				Time:         oneBack.Time,
				Bits:         oneBack.Bits,
				Nonce:        oneBack.Nonce,
			},
			Height: oneBack.Height,
			Hash:   oneBack.Hash,
		},
		ChainWork:        oneBack.ChainWork,
		IsChainTip:       oneBack.IsChainTip,
		IsActive:         oneBack.IsActive,
		HeaderID:         oneBack.HeaderID,
		PreviousHeaderID: oneBack.PreviousHeaderID,
	}
}
