package wdk_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	testPartyAlice = "alice"
	testPartyBob   = "bob"
	// 32-byte hex txids
	testTxID1 = "1111111111111111111111111111111111111111111111111111111111111111"
	testTxID2 = "2222222222222222222222222222222222222222222222222222222222222222"
	testTxID3 = "3333333333333333333333333333333333333333333333333333333333333333"
)

func TestNewBeefParty(t *testing.T) {
	t.Run("creates party with initial parties", func(t *testing.T) {
		bp := wdk.NewBeefParty([]string{testPartyAlice, testPartyBob})

		assert.True(t, bp.IsParty(testPartyAlice))
		assert.True(t, bp.IsParty(testPartyBob))
		assert.False(t, bp.IsParty("unknown"))
	})

	t.Run("creates party with no initial parties", func(t *testing.T) {
		bp := wdk.NewBeefParty(nil)

		assert.False(t, bp.IsParty(testPartyAlice))
	})
}

func TestBeefPartyAddParty(t *testing.T) {
	bp := wdk.NewBeefParty(nil)
	bp.AddParty(testPartyAlice)

	assert.True(t, bp.IsParty(testPartyAlice))

	// Re-adding should not wipe known txids
	require.NoError(t, bp.AddKnownTxIDsForParty(t.Context(), testPartyAlice, testTxID1))
	bp.AddParty(testPartyAlice)

	ids, err := bp.GetKnownTxIDsForParty(t.Context(), testPartyAlice)
	require.NoError(t, err)
	assert.Equal(t, []string{testTxID1}, ids)
}

func TestBeefPartyGetKnownTxIDsForParty(t *testing.T) {
	t.Run("returns error for unknown party", func(t *testing.T) {
		bp := wdk.NewBeefParty(nil)

		ids, err := bp.GetKnownTxIDsForParty(t.Context(), testPartyAlice)
		require.Error(t, err)
		assert.Nil(t, ids)
		assert.Contains(t, err.Error(), "unknown party")
	})

	t.Run("returns empty slice for new party", func(t *testing.T) {
		bp := wdk.NewBeefParty([]string{testPartyAlice})

		ids, err := bp.GetKnownTxIDsForParty(t.Context(), testPartyAlice)
		require.NoError(t, err)
		assert.Empty(t, ids)
	})

	t.Run("returns copy of known txids", func(t *testing.T) {
		bp := wdk.NewBeefParty([]string{testPartyAlice})
		require.NoError(t, bp.AddKnownTxIDsForParty(t.Context(), testPartyAlice, testTxID1))

		ids, err := bp.GetKnownTxIDsForParty(t.Context(), testPartyAlice)
		require.NoError(t, err)
		require.Len(t, ids, 1)

		// Mutating returned slice must not affect internal state
		ids[0] = "mutated"
		ids2, err := bp.GetKnownTxIDsForParty(t.Context(), testPartyAlice)
		require.NoError(t, err)
		assert.Equal(t, testTxID1, ids2[0])
	})
}

func TestBeefPartyAddKnownTxIDsForParty(t *testing.T) {
	t.Run("auto-creates party if missing", func(t *testing.T) {
		bp := wdk.NewBeefParty(nil)

		require.NoError(t, bp.AddKnownTxIDsForParty(t.Context(), testPartyAlice, testTxID1, testTxID2))

		assert.True(t, bp.IsParty(testPartyAlice))
		ids, err := bp.GetKnownTxIDsForParty(t.Context(), testPartyAlice)
		require.NoError(t, err)
		assert.Equal(t, []string{testTxID1, testTxID2}, ids)
	})

	t.Run("deduplicates txids", func(t *testing.T) {
		bp := wdk.NewBeefParty([]string{testPartyAlice})

		require.NoError(t, bp.AddKnownTxIDsForParty(t.Context(), testPartyAlice, testTxID1, testTxID1, testTxID2))
		require.NoError(t, bp.AddKnownTxIDsForParty(t.Context(), testPartyAlice, testTxID2, testTxID3))

		ids, err := bp.GetKnownTxIDsForParty(t.Context(), testPartyAlice)
		require.NoError(t, err)
		assert.Equal(t, []string{testTxID1, testTxID2, testTxID3}, ids)
	})

	t.Run("merges txid-only into embedded beef", func(t *testing.T) {
		bp := wdk.NewBeefParty([]string{testPartyAlice})

		require.NoError(t, bp.AddKnownTxIDsForParty(t.Context(), testPartyAlice, testTxID1))

		hash, err := chainhash.NewHashFromHex(testTxID1)
		require.NoError(t, err)
		assert.Contains(t, bp.Transactions, *hash)
	})

	t.Run("returns error for invalid hex txid", func(t *testing.T) {
		bp := wdk.NewBeefParty([]string{testPartyAlice})

		err := bp.AddKnownTxIDsForParty(t.Context(), testPartyAlice, "not-a-txid")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse")
	})
}

func TestBeefPartyGetTrimmedBeefForParty(t *testing.T) {
	t.Run("returns error for unknown party", func(t *testing.T) {
		bp := wdk.NewBeefParty(nil)

		trimmed, err := bp.GetTrimmedBeefForParty(t.Context(), testPartyAlice)
		require.Error(t, err)
		assert.Nil(t, trimmed)
	})

	t.Run("trims known txids from cloned beef", func(t *testing.T) {
		bp := wdk.NewBeefParty([]string{testPartyAlice})
		require.NoError(t, bp.AddKnownTxIDsForParty(t.Context(), testPartyAlice, testTxID1, testTxID2))

		// Add another txid not known to the party
		hash3, err := chainhash.NewHashFromHex(testTxID3)
		require.NoError(t, err)
		bp.MergeTxidOnly(t.Context(), hash3)

		trimmed, err := bp.GetTrimmedBeefForParty(t.Context(), testPartyAlice)
		require.NoError(t, err)
		require.NotNil(t, trimmed)

		hash1, err := chainhash.NewHashFromHex(testTxID1)
		require.NoError(t, err)
		hash2, err := chainhash.NewHashFromHex(testTxID2)
		require.NoError(t, err)

		assert.NotContains(t, trimmed.Transactions, *hash1)
		assert.NotContains(t, trimmed.Transactions, *hash2)
		assert.Contains(t, trimmed.Transactions, *hash3)
	})
}
