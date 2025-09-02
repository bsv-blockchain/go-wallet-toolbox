package testutils

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

func MockValidMerklePath(t testing.TB, txID string, blockHeight uint32) sdk.MerklePath {
	t.Helper()

	hash, err := chainhash.NewHashFromHex(txID)
	require.NoError(t, err)

	someSecondHash, errHash := chainhash.NewHashFromHex("27a53423aa3e5d5c46bf30be53a9998dd247daf758847f244f82d430be71de6e")
	require.NoError(t, errHash)

	return sdk.MerklePath{
		BlockHeight: blockHeight,
		Path: [][]*sdk.PathElement{
			{
				{
					Offset: 0,
					Hash:   hash,
					Txid:   to.Ptr(true),
				},
				{
					Offset: 1,
					Hash:   someSecondHash,
				},
			},
		},
	}
}
