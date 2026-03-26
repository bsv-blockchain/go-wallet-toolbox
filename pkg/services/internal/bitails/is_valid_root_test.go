package bitails_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
)

func TestBitails_IsValidRootForHeight(t *testing.T) {
	t.Run("returns true for valid root", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		svc := given.NewBitailsService()
		validRoot := testabilities.HashFromHex(t, testabilities.TestMerkleRootHex)

		given.Bitails().
			WillRespondWithBlockHeaderByHeight(http.StatusOK, testabilities.TestBlockHeight,
				testabilities.FakeHeaderHexWithMerkleRoot(t, testabilities.TestMerkleRootHex))

		// when:
		ok, err := svc.IsValidRootForHeight(t.Context(), validRoot, testabilities.TestBlockHeight)

		// then:
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("returns false for invalid root", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		svc := given.NewBitailsService()

		wrongRoot := testabilities.HashFromHex(t, "0000000000000000000000000000000000000000000000000000000000000000")
		given.Bitails().
			WillRespondWithBlockHeaderByHeight(http.StatusOK, testabilities.TestBlockHeight,
				testabilities.FakeHeaderHexWithMerkleRoot(t, testabilities.TestMerkleRootHex))

		// when:
		ok, err := svc.IsValidRootForHeight(t.Context(), wrongRoot, testabilities.TestBlockHeight)

		// then:
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("returns error when bitails call fails", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		svc := given.NewBitailsService()

		root := testabilities.HashFromHex(t, testabilities.TestMerkleRootHex)
		given.Bitails().
			WillRespondWithBlockHeaderByHeight(http.StatusInternalServerError, testabilities.TestBlockHeight, "")

		// when:
		ok, err := svc.IsValidRootForHeight(t.Context(), root, testabilities.TestBlockHeight)

		// then:
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("returns error on malformed block header", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		svc := given.NewBitailsService()
		root := testabilities.HashFromHex(t, testabilities.TestMerkleRootHex)

		given.Bitails().
			WillRespondWithBlockHeaderByHeight(http.StatusOK, testabilities.TestBlockHeight, "zzznothex")

		// when:
		ok, err := svc.IsValidRootForHeight(t.Context(), root, testabilities.TestBlockHeight)

		// then:
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("context canceled returns error", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		svc := given.NewBitailsService()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		root := testabilities.HashFromHex(t, testabilities.TestMerkleRootHex)

		// when:
		ok, err := svc.IsValidRootForHeight(ctx, root, testabilities.TestBlockHeight)

		// then:
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("zero height returns error", func(t *testing.T) {
		// given:
		given := testabilities.Given(t)
		svc := given.NewBitailsService()

		root := testabilities.HashFromHex(t, testabilities.TestMerkleRootHex)

		// when:
		ok, err := svc.IsValidRootForHeight(t.Context(), root, 0)

		// then:
		require.Error(t, err)
		require.False(t, ok)
	})
}

func TestBitails_IsValidRootForHeight_Caching(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	svc := given.NewBitailsService()
	tr := given.Bitails().Transport()

	validRoot := testabilities.HashFromHex(t, testabilities.TestMerkleRootHex)
	given.Bitails().
		WillRespondWithBlockHeaderByHeight(http.StatusOK, testabilities.TestBlockHeight,
			testabilities.FakeHeaderHexWithMerkleRoot(t, testabilities.TestMerkleRootHex))

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
