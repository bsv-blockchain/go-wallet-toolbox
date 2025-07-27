package whatsonchain_test

import (
	"net/http"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhatsOnChain_GetUtxoStatus_Success(t *testing.T) {
	// given:
	scriptHash := testabilities.TestScriptHash
	txid := testabilities.TestTxIDHex
	index := testabilities.TestTxIndex
	height := testabilities.TestUtxoHeight
	value := testabilities.TestUtxoSatoshis

	given := testabilities.Given(t)
	fixture := given.WhatsOnChain()

	fixture.WillRespondWithUtxoStatus(http.StatusOK, scriptHash,
		testabilities.UtxoSuccessJSON(scriptHash, txid, index, height, value))

	woc := given.NewWoCService()

	// when:
	result, err := woc.GetUtxoStatus(t.Context(), scriptHash, &transaction.Outpoint{
		Txid:  *testabilities.MustHashFromHex(txid),
		Index: index,
	})

	// then:
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.IsUtxo)
	assert.Equal(t, whatsonchain.ServiceName, result.Name)
	require.Len(t, result.Details, 1)
	assert.Equal(t, txid, result.Details[0].TxID)
	assert.Equal(t, index, result.Details[0].Index)
	assert.Equal(t, height, result.Details[0].Height)
	assert.Equal(t, value, result.Details[0].Satoshis)
}

func TestWhatsOnChain_GetUtxoStatus_APIError(t *testing.T) {
	scriptHash := testabilities.TestScriptHash
	given := testabilities.Given(t)
	fixture := given.WhatsOnChain()

	fixture.WillRespondWithUtxoStatus(http.StatusOK, scriptHash, testabilities.UtxoAPIErrorJSON("scripthash not found"))

	woc := given.NewWoCService()

	result, err := woc.GetUtxoStatus(t.Context(), scriptHash, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "WoC API error: scripthash not found")
	assert.Nil(t, result)
}

func TestWhatsOnChain_GetUtxoStatus_HTTPError(t *testing.T) {
	scriptHash := testabilities.TestScriptHash
	given := testabilities.Given(t)
	fixture := given.WhatsOnChain()

	fixture.WillRespondWithUtxoStatus(http.StatusInternalServerError, scriptHash, "fail")

	woc := given.NewWoCService()

	result, err := woc.GetUtxoStatus(t.Context(), scriptHash, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status code 500")
	assert.Nil(t, result)
}

func TestWhatsOnChain_GetUtxoStatus_ValidationError(t *testing.T) {
	invalidScriptHash := "short"

	given := testabilities.Given(t)
	woc := given.NewWoCService()

	result, err := woc.GetUtxoStatus(t.Context(), invalidScriptHash, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid scripthash length")
	assert.Nil(t, result)
}
