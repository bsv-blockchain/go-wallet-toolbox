package whatsonchain_test

import (
	"net/http"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
)

func TestWhatsOnChain_IsUtxo_Success(t *testing.T) {
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
	isUtxo, err := woc.IsUtxo(t.Context(), scriptHash, &transaction.Outpoint{
		Txid:  *testabilities.MustHashFromHex(txid),
		Index: index,
	})

	// then:
	require.NoError(t, err)
	assert.True(t, isUtxo)
}

func TestWhatsOnChain_IsUtxo_NoMatch(t *testing.T) {
	// given:
	scriptHash := testabilities.TestScriptHash

	given := testabilities.Given(t)
	fixture := given.WhatsOnChain()
	fixture.WillRespondWithUtxoStatus(http.StatusOK, scriptHash,
		`{"result":[{"tx_hash":"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", "tx_pos":2, "height":700000, "value":1000}]}`)

	woc := given.NewWoCService()

	// when:
	isUtxo, err := woc.IsUtxo(t.Context(), scriptHash, &transaction.Outpoint{
		Txid:  *testabilities.MustHashFromHex(testabilities.TestTxIDHex),
		Index: testabilities.TestTxIndex,
	})

	// then:
	require.NoError(t, err)
	assert.False(t, isUtxo)
}

func TestWhatsOnChain_IsUtxo_APIError(t *testing.T) {
	// given:
	scriptHash := testabilities.TestScriptHash

	given := testabilities.Given(t)
	fixture := given.WhatsOnChain()
	fixture.WillRespondWithUtxoStatus(http.StatusOK, scriptHash,
		testabilities.UtxoAPIErrorJSON("some api error"))

	woc := given.NewWoCService()

	// when:
	isUtxo, err := woc.IsUtxo(t.Context(), scriptHash, &transaction.Outpoint{
		Txid:  *testabilities.MustHashFromHex(testabilities.TestTxIDHex),
		Index: testabilities.TestTxIndex,
	})

	// then:
	require.Error(t, err)
	assert.False(t, isUtxo)
	assert.Contains(t, err.Error(), "failed to determine UTXO status")
}

func TestWhatsOnChain_IsUtxo_HTTPError(t *testing.T) {
	// given:
	scriptHash := testabilities.TestScriptHash

	given := testabilities.Given(t)
	fixture := given.WhatsOnChain()
	fixture.WillRespondWithUtxoStatus(http.StatusInternalServerError, scriptHash, "fail")

	woc := given.NewWoCService()

	// when:
	isUtxo, err := woc.IsUtxo(t.Context(), scriptHash, &transaction.Outpoint{
		Txid:  *testabilities.MustHashFromHex(testabilities.TestTxIDHex),
		Index: testabilities.TestTxIndex,
	})

	// then:
	require.Error(t, err)
	assert.False(t, isUtxo)
	assert.Contains(t, err.Error(), "failed to determine UTXO status")
}

func TestWhatsOnChain_IsUtxo_InvalidScriptHash(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	woc := given.NewWoCService()

	// when:
	isUtxo, err := woc.IsUtxo(t.Context(), "short", &transaction.Outpoint{
		Txid:  *testabilities.MustHashFromHex(testabilities.TestTxIDHex),
		Index: testabilities.TestTxIndex,
	})

	// then:
	require.Error(t, err)
	assert.False(t, isUtxo)
	assert.Contains(t, err.Error(), "invalid scripthash length")
}

func TestWhatsOnChain_IsUtxo_NilOutpoint(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	woc := given.NewWoCService()

	// when:
	isUtxo, err := woc.IsUtxo(t.Context(), testabilities.TestScriptHash, nil)

	// then:
	require.Error(t, err)
	assert.False(t, isUtxo)
	assert.Contains(t, err.Error(), "outpoint is required")
}
