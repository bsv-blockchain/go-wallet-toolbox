package bitails_test

import (
	"net/http"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	"github.com/stretchr/testify/require"
)

func TestBitails_IsValidRootForHeight(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	svc := given.NewBitailsService()
	tr := given.Bitails().Transport()

	validRoot := testabilities.HashFromHex(t, testabilities.TestMerkleRootHex)
	given.Bitails().WillRespondWithBlockHeaderByHeight(http.StatusOK, testabilities.TestBlockHeight, testabilities.FakeHeaderHexWithMerkleRoot(t, testabilities.TestMerkleRootHex))

	// when:
	ok, err := svc.IsValidRootForHeight(t.Context(), validRoot, testabilities.TestBlockHeight)

	// then:
	require.NoError(t, err)
	require.True(t, ok)

	// when:
	tr.Reset()
	ok, err = svc.IsValidRootForHeight(t.Context(), validRoot, testabilities.TestBlockHeight)

	// then:
	require.NoError(t, err)
	require.True(t, ok)
}
