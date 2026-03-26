package bitails_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
)

func TestBitails_GetScriptHistory_WithTransactionsOneConfirmedOneUnconfirmed(t *testing.T) {
	given := testabilities.Given(t)
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	given.Bitails().
		ScriptHistoryData().
		WithScriptHash(scriptHash).
		WithConfirmedTransactions(1, 800000).
		WithUnconfirmedTransactions(1).
		WillBeReturned()

	svc := given.NewBitailsService()

	result, err := svc.GetScriptHashHistory(t.Context(), scriptHash)

	require.NoError(t, err)
	require.Len(t, result.History, 2)

	assert.Equal(t, "00000000000e1b71dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9", result.History[0].TxHash)
	assert.NotNil(t, result.History[0].Height)

	assert.Equal(t, "00000000000e1b81dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9", result.History[1].TxHash)
	assert.Nil(t, result.History[1].Height)
}

func TestBitails_GetScriptHistory_ValidationErrors(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	b := given.NewBitailsService()

	invalid := map[string]string{
		"empty":              "",
		"too short":          "abc123",
		"too long":           "abc1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		"non-hex characters": "invalid!!@@",
	}

	for name, scriptHash := range invalid {
		t.Run(name, func(t *testing.T) {
			// when:
			result, err := b.GetScriptHashHistory(t.Context(), scriptHash)

			// then:
			require.Error(t, err)
			assert.Nil(t, result)
		})
	}
}

func TestBitails_GetScriptHistory_HTTPError(t *testing.T) {
	// given
	given := testabilities.Given(t)
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	given.Bitails().
		ScriptHistoryData().
		WithScriptHash(scriptHash).
		WithConfirmedTransactionsNotFound().
		WillBeReturned()

	b := given.NewBitailsService()

	// when
	result, err := b.GetScriptHashHistory(t.Context(), scriptHash)

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status code 404")
	assert.Nil(t, result)
}

func TestBitails_GetScriptHistory_OnlyConfirmed(t *testing.T) {
	// given
	given := testabilities.Given(t)
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	given.Bitails().
		ScriptHistoryData().
		WithScriptHash(scriptHash).
		WithConfirmedTransactions(1, 800000).
		WithUnconfirmedTransactions(0).
		WillBeReturned()

	b := given.NewBitailsService()

	// when
	result, err := b.GetScriptHashHistory(t.Context(), scriptHash)

	// then
	require.NoError(t, err)
	assert.Len(t, result.History, 1)
	assert.Equal(t, "00000000000e1b71dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9", result.History[0].TxHash)
	assert.NotNil(t, result.History[0].Height)
	assert.Equal(t, 800000, *result.History[0].Height)
}

func TestBitails_GetScriptHistory_OnlyUnconfirmed(t *testing.T) {
	// given
	given := testabilities.Given(t)
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	given.Bitails().
		ScriptHistoryData().
		WithScriptHash(scriptHash).
		WithConfirmedTransactions(0, 0).
		WithUnconfirmedTransactions(1).
		WillBeReturned()

	b := given.NewBitailsService()

	// when
	result, err := b.GetScriptHashHistory(t.Context(), scriptHash)

	// then
	require.NoError(t, err)
	assert.Len(t, result.History, 1)
	assert.Equal(t, "00000000000e1b81dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9", result.History[0].TxHash)
	assert.Nil(t, result.History[0].Height)
}

func TestBitails_GetScriptHistory_ManyItems_NoPagination(t *testing.T) {
	testCases := []struct {
		name               string
		confirmedCount     int
		unconfirmedCount   int
		startHeight        int
		expectedTotalCount int
	}{
		{
			name:               "standard_large_history",
			confirmedCount:     100,
			unconfirmedCount:   10,
			startHeight:        800000,
			expectedTotalCount: 110,
		},
		{
			name:               "very_large_confirmed_only",
			confirmedCount:     1000,
			unconfirmedCount:   0,
			startHeight:        900000,
			expectedTotalCount: 1000,
		},
		{
			name:               "large_unconfirmed_only",
			confirmedCount:     0,
			unconfirmedCount:   500,
			startHeight:        0,
			expectedTotalCount: 500,
		},
		{
			name:               "mixed_large_history",
			confirmedCount:     250,
			unconfirmedCount:   250,
			startHeight:        750000,
			expectedTotalCount: 500,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			given := testabilities.Given(t)
			scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

			given.Bitails().
				ScriptHistoryData().
				WithScriptHash(scriptHash).
				WithConfirmedTransactions(tc.confirmedCount, tc.startHeight).
				WithUnconfirmedTransactions(tc.unconfirmedCount).
				WillBeReturned()

			b := given.NewBitailsService()

			// when
			result, err := b.GetScriptHashHistory(t.Context(), scriptHash)

			// then
			require.NoError(t, err)
			assert.Len(t, result.History, tc.expectedTotalCount)

			for i := 0; i < tc.confirmedCount; i++ {
				expectedTxID := fmt.Sprintf("%02x%062s", i, "e1b71dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9")
				assert.NotNil(t, result.History[i].Height, "Confirmed transaction %d should have height", i)
				assert.Equal(t, tc.startHeight+i, *result.History[i].Height)
				assert.Equal(t, expectedTxID, result.History[i].TxHash)
			}

			for i := tc.confirmedCount; i < tc.confirmedCount+tc.unconfirmedCount; i++ {
				expectedTxID := fmt.Sprintf("%02x%062s", i-tc.confirmedCount, "e1b81dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9")
				assert.Nil(t, result.History[i].Height, "Unconfirmed transaction %d should have nil height", i)
				assert.Equal(t, expectedTxID, result.History[i].TxHash)
			}
		})
	}
}
