package bitails_test

import (
	"context"
	"testing"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/bitails"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBitails_MerklePath(t *testing.T) {
	// given:
	fixture := testabilities.Given(t)
	service := fixture.NewBitailsService()

	txID := testabilities.TestTxID
	blockHash := testabilities.TestTargetHash
	siblingHash := testabilities.TestSiblingHash
	height := testabilities.TestBlockHeight

	fixture.Bitails().WillReturnTscProof(txID, blockHash, 1, []string{siblingHash})
	fixture.Bitails().WillReturnBlockHeader(blockHash, testabilities.TestFakeHeaderBinary)
	fixture.Bitails().WillReturnTxInfo(txID, blockHash, int64(height))

	// when:
	ctx := context.Background()
	result, err := service.MerklePath(ctx, txID)

	// then:
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, bitails.ServiceName, result.Name)
	assert.NotNil(t, result.MerklePath)
	assert.NotNil(t, result.BlockHeader)

	require.Len(t, result.Notes, 1)
	assert.Contains(t, result.Notes[0].What, "getMerklePath")
	assert.WithinDuration(t, time.Now(), *result.Notes[0].When, 2*time.Second)
}
