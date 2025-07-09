package services_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhatsOnChain_GetScriptHistory_ValidResponse(t *testing.T) {
	// given
	given := testabilities.Given(t)
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	given.WhatsOnChain().
		ScriptHistoryData().
		WithScriptHash(scriptHash).
		WithConfirmedTransactions(2, 800000).
		WithUnconfirmedTransactions(1).
		WillBeReturned()

	woc := given.NewWoCService()

	// when
	result, err := woc.GetScriptHistory(t.Context(), scriptHash)

	// then
	require.NoError(t, err)
	assert.Len(t, result.History, 3)
	assert.Equal(t, "c0000000000e1b81dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9", result.History[0].TxHash)
	assert.Equal(t, 800000, *result.History[0].Height)
	assert.Equal(t, "u0000000000e1b81dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9", result.History[2].TxHash)
	assert.Nil(t, result.History[2].Height)
}

func TestWhatsOnChain_GetScriptHistory_ValidationErrors(t *testing.T) {
	// given
	given := testabilities.Given(t)
	woc := given.NewWoCService()

	invalidTestCases := map[string]struct {
		scriptHash    string
		expectedError string
	}{
		"empty scripthash": {
			scriptHash:    "",
			expectedError: "scripthash cannot be empty",
		},
		"too short scripthash": {
			scriptHash:    "a914b7536c",
			expectedError: "invalid scripthash length: too short",
		},
		"too long scripthash": {
			scriptHash:    "a914b7536c788d8ca2de4d867a2b5b02acef97f35aef488aca914b7536c788d8ca2de4d867a2b5b02acef97f35aef488ac",
			expectedError: "invalid scripthash length: too long",
		},
		"invalid hex characters": {
			scriptHash:    "this is not valid hex!! this is not valid hex!!",
			expectedError: "invalid scripthash format",
		},
	}

	for name, testCase := range invalidTestCases {
		t.Run(name, func(t *testing.T) {
			// when
			result, err := woc.GetScriptHistory(t.Context(), testCase.scriptHash)

			// then
			require.Error(t, err)
			assert.Contains(t, err.Error(), testCase.expectedError)
			assert.Nil(t, result)
		})
	}
}

func TestWhatsOnChain_GetScriptHistory_APIError(t *testing.T) {
	// given
	given := testabilities.Given(t)
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	given.WhatsOnChain().
		ScriptHistoryData().
		WithScriptHash(scriptHash).
		WithConfirmedTransactionsError("Script not found").
		WillBeReturned()

	woc := given.NewWoCService()

	// when
	result, err := woc.GetScriptHistory(t.Context(), scriptHash)

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error: Script not found")
	assert.Nil(t, result)
}

func TestWhatsOnChain_GetScriptHistory_HTTPError(t *testing.T) {
	// given
	given := testabilities.Given(t)
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	given.WhatsOnChain().
		ScriptHistoryData().
		WithScriptHash(scriptHash).
		WithConfirmedTransactionsError("Not found").
		WithConfirmedStatusCode(404).
		WillBeReturned()

	woc := given.NewWoCService()

	// when
	result, err := woc.GetScriptHistory(t.Context(), scriptHash)

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status code 404")
	assert.Nil(t, result)
}

func TestWhatsOnChain_GetScriptHistory_OnlyConfirmed(t *testing.T) {
	// given
	given := testabilities.Given(t)
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	given.WhatsOnChain().
		ScriptHistoryData().
		WithScriptHash(scriptHash).
		WithConfirmedTransactions(1, 800000).
		WithUnconfirmedTransactions(0).
		WillBeReturned()

	woc := given.NewWoCService()

	// when
	result, err := woc.GetScriptHistory(t.Context(), scriptHash)

	// then
	require.NoError(t, err)
	assert.Len(t, result.History, 1)
	assert.Equal(t, "c0000000000e1b81dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9", result.History[0].TxHash)
	assert.NotNil(t, result.History[0].Height)
	assert.Equal(t, 800000, *result.History[0].Height)
}

func TestWhatsOnChain_GetScriptHistory_OnlyUnconfirmed(t *testing.T) {
	// given
	given := testabilities.Given(t)
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	given.WhatsOnChain().
		ScriptHistoryData().
		WithScriptHash(scriptHash).
		WithConfirmedTransactions(0, 0).
		WithUnconfirmedTransactions(1).
		WillBeReturned()

	woc := given.NewWoCService()

	// when
	result, err := woc.GetScriptHistory(t.Context(), scriptHash)

	// then
	require.NoError(t, err)
	assert.Len(t, result.History, 1)
	assert.Equal(t, "u0000000000e1b81dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9", result.History[0].TxHash)
	assert.Nil(t, result.History[0].Height)
}
