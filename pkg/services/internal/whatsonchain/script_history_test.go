package whatsonchain_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetScriptHistory_UsingQueryFixture(t *testing.T) {
	// given
	given := testabilities.Given(t)
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"
	// Use the builder pattern instead of plain response objects
	given.WhatsOnChain().
		WithValidScriptHistoryData().
		WithScriptHash(scriptHash).
		WithConfirmedTransactions(1, 800000).
		WithUnconfirmedTransactions(1).
		WillBeReturned()

	woc := given.NewWoCService()

	// when
	result, err := woc.GetScriptHistory(t.Context(), scriptHash)

	// then
	require.NoError(t, err)
	assert.Len(t, result.History, 2)
	assert.Equal(t, "c0000000000e1b81dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9", result.History[0].TxHash)
	assert.Equal(t, "u0000000000e1b81dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9", result.History[1].TxHash)
}

func TestWhatsOnChain_GetScriptHistory_ValidResponse(t *testing.T) {
	// given
	given := testabilities.Given(t)
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	confirmedJSON := `{
		"result": [
			{"tx_hash": "tx1", "height": 800000},
			{"tx_hash": "tx2", "height": 800001}
		],
		"error": ""
	}`

	unconfirmedJSON := `{
		"result": [
			{"tx_hash": "tx3", "height": null}
		],
		"error": ""
	}`

	given.WhatsOnChain().WillRespondWithConfirmedScriptHistory(http.StatusOK, scriptHash, confirmedJSON)
	given.WhatsOnChain().WillRespondWithUnconfirmedScriptHistory(http.StatusOK, scriptHash, unconfirmedJSON)

	woc := given.NewWoCService()

	// when
	result, err := woc.GetScriptHistory(t.Context(), scriptHash)

	// then
	require.NoError(t, err)
	assert.Len(t, result.History, 3)
	assert.Equal(t, "tx1", result.History[0].TxHash)
	assert.Equal(t, 800000, *result.History[0].Height)
	assert.Equal(t, "tx3", result.History[2].TxHash)
	assert.Nil(t, result.History[2].Height)
}

func TestServices_GetScriptHistory_EmptyHistory(t *testing.T) {
	// given
	given := testabilities.Given(t)

	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	given.WhatsOnChain().
		WithValidScriptHistoryData().
		WithScriptHash(scriptHash).
		WithEmptyHistory().
		WillBeReturned()

	woc := given.NewWoCService()

	// when
	result, err := woc.GetScriptHistory(t.Context(), scriptHash)

	// then
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.History)
}

func TestWhatsOnChain_GetScriptHistory_ConfirmedAPIError(t *testing.T) {
	// given
	given := testabilities.Given(t)
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	errorJSON := `{
		"result": [],
		"error": "Script not found"
	}`

	given.WhatsOnChain().WillRespondWithConfirmedScriptHistory(http.StatusOK, scriptHash, errorJSON)

	woc := given.NewWoCService()

	// when
	result, err := woc.GetScriptHistory(t.Context(), scriptHash)

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error: Script not found")
	assert.Nil(t, result)
}

func TestWhatsOnChain_GetScriptHistory_UnconfirmedAPIError(t *testing.T) {
	// given
	given := testabilities.Given(t)
	scriptHash := "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	confirmedJSON := `{
		"result": [],
		"error": ""
	}`

	errorJSON := `{
		"result": [],
		"error": "Script not found"
	}`

	given.WhatsOnChain().WillRespondWithConfirmedScriptHistory(http.StatusOK, scriptHash, confirmedJSON)
	given.WhatsOnChain().WillRespondWithUnconfirmedScriptHistory(http.StatusOK, scriptHash, errorJSON)

	woc := given.NewWoCService()

	// when
	result, err := woc.GetScriptHistory(t.Context(), scriptHash)

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error: Script not found")
	assert.Nil(t, result)
}

func TestGetScriptHistory_LargeHistory(t *testing.T) {
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

			given.WhatsOnChain().
				WithValidScriptHistoryData().
				WithScriptHash(scriptHash).
				WithConfirmedTransactions(tc.confirmedCount, tc.startHeight).
				WithUnconfirmedTransactions(tc.unconfirmedCount).
				WillBeReturned()

			woc := given.NewWoCService()

			// when
			result, err := woc.GetScriptHistory(t.Context(), scriptHash)

			// then
			require.NoError(t, err)
			assert.Len(t, result.History, tc.expectedTotalCount)

			for i := 0; i < tc.confirmedCount; i++ {
				assert.NotNil(t, result.History[i].Height, "Confirmed transaction %d should have height", i)
				assert.Equal(t, tc.startHeight+i, *result.History[i].Height)
				assert.Equal(t, fmt.Sprintf("c%010de1b81dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9", i), result.History[i].TxHash)
			}

			for i := tc.confirmedCount; i < tc.confirmedCount+tc.unconfirmedCount; i++ {
				assert.Nil(t, result.History[i].Height, "Unconfirmed transaction %d should have nil height", i)
				assert.Equal(t, fmt.Sprintf("u%010de1b81dd2c9c0c6cd67f9bdf832e9c2bb12a1d57f30cb6ebbe78d9", i-tc.confirmedCount), result.History[i].TxHash)
			}
		})
	}
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

	errorJSON := `{
		"result": [],
		"error": "Script not found"
	}`

	given.WhatsOnChain().WillRespondWithConfirmedScriptHistory(http.StatusOK, scriptHash, errorJSON)

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

	given.WhatsOnChain().WillRespondWithConfirmedScriptHistory(http.StatusNotFound, scriptHash, "")

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
		WithValidScriptHistoryData().
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
		WithValidScriptHistoryData().
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
