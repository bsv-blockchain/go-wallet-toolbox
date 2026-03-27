package whatsonchain_test

import (
	"net/http"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
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
	details := result.Details[0]
	assert.Equal(t, txid, details.TxID)
	assert.Equal(t, index, details.Index)
	assert.Equal(t, height, details.Height)
	assert.Equal(t, value, details.Satoshis)
}

func TestWhatsOnChain_GetUtxoStatus_APIError(t *testing.T) {
	// given:
	scriptHash := testabilities.TestScriptHash
	given := testabilities.Given(t)
	fixture := given.WhatsOnChain()

	fixture.WillRespondWithUtxoStatus(http.StatusOK, scriptHash, testabilities.UtxoAPIErrorJSON("scripthash not found"))

	woc := given.NewWoCService()

	// when:
	result, err := woc.GetUtxoStatus(t.Context(), scriptHash, nil)

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WoC API error: scripthash not found")
	assert.Nil(t, result)
}

func TestWhatsOnChain_GetUtxoStatus_HTTPError(t *testing.T) {
	// given:
	scriptHash := testabilities.TestScriptHash
	given := testabilities.Given(t)
	fixture := given.WhatsOnChain()

	fixture.WillRespondWithUtxoStatus(http.StatusInternalServerError, scriptHash, "fail")

	woc := given.NewWoCService()

	// when:
	result, err := woc.GetUtxoStatus(t.Context(), scriptHash, nil)

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status code 500")
	assert.Nil(t, result)
}

func TestWhatsOnChain_GetUtxoStatus_ValidationError(t *testing.T) {
	// given:
	invalidScriptHash := "short"

	given := testabilities.Given(t)
	woc := given.NewWoCService()

	// when:
	result, err := woc.GetUtxoStatus(t.Context(), invalidScriptHash, nil)

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid scripthash length")
	assert.Nil(t, result)
}
