package fixtures

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

func DefaultRequestSyncChunkArgs(userIdentityKey, fromIdentityKey string) wdk.RequestSyncChunkArgs {
	return wdk.RequestSyncChunkArgs{
		FromStorageIdentityKey: fromIdentityKey,
		ToStorageIdentityKey:   "to_storage",
		IdentityKey:            userIdentityKey,
		MaxItems:               10,
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
			// TODO: Add more offsets for other entities when implemented
		},
	}
}
