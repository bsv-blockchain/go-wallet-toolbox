package fixtures

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func DefaultRequestSyncChunkArgs(userIdentityKey, fromIdentityKey, toIdentityKey string) wdk.RequestSyncChunkArgs {
	return wdk.RequestSyncChunkArgs{
		FromStorageIdentityKey: fromIdentityKey,
		ToStorageIdentityKey:   toIdentityKey,
		IdentityKey:            userIdentityKey,
		MaxItems:               1000,
		MaxRoughSize:           100_000,

		Offsets: []wdk.SyncOffsets{
			{
				Name:   wdk.OutputBasketEntityName,
				Offset: 0,
			},
			{
				Name:   wdk.ProvenTxReqEntityName,
				Offset: 0,
			},
			{
				Name:   wdk.ProvenTxEntityName,
				Offset: 0,
			},
			{
				Name:   wdk.TransactionEntityName,
				Offset: 0,
			},
			{
				Name:   wdk.OutputEntityName,
				Offset: 0,
			},
			{
				Name:   wdk.TxLabelEntityName,
				Offset: 0,
			},
			{
				Name:   wdk.TxLabelMapEntityName,
				Offset: 0,
			},
			{
				Name:   wdk.OutputTagEntityName,
				Offset: 0,
			},
			{
				Name:   wdk.OutputTagMapEntityName,
				Offset: 0,
			},
			// TODO: Add more offsets for other entities when implemented
		},
	}
}
