package whatsonchain_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	tst "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestWhatsOnChain_GetStatusForTxIDs_Error_NoTxids(t *testing.T) {
	// given:
	given := tst.Given(t)
	svc := given.NewWoCService()

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), nil)

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
}

func TestWhatsOnChain_GetStatusForTxIDs_HTTPError(t *testing.T) {
	// given:
	given := tst.Given(t)
	svc := given.NewWoCService()

	txIDs := []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	// force non-200 from the WoC mock
	given.WhatsOnChain().WillRespondOnTxStatus(http.StatusInternalServerError, testservices.TxStatusExpectation{})

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), txIDs)

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "failed to get status for txIDs")
}

func TestWhatsOnChain_GetStatusForTxIDs_Success_Single(t *testing.T) {
	// given:
	given := tst.Given(t)
	svc := given.NewWoCService()

	txid := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	given.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{
		ExpectBlockHash:   "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		ExpectBlockHeight: 777777,
	})

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), []string{txid})

	// then:
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, whatsonchain.ServiceName, res.Name)
	assert.Equal(t, wdk.GetStatusSuccess, res.Status)
	require.Len(t, res.Results, 1)

	got := res.Results[0]
	assert.Equal(t, txid, got.TxID)
	require.NotNil(t, got.Depth)
	assert.Equal(t, 10, *got.Depth)
	assert.Equal(t, "mined", got.Status)
}

func TestWhatsOnChain_GetStatusForTxIDs_Success_Multiple(t *testing.T) {
	// given:
	given := tst.Given(t)
	svc := given.NewWoCService()

	txIDs := []string{
		"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
	}

	given.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{
		ExpectBlockHash:   "feedfacefeedfacefeedfacefeedfacefeedfacefeedfacefeedfacefeedface",
		ExpectBlockHeight: 123456,
	})

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), txIDs)

	// then:
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, whatsonchain.ServiceName, res.Name)
	assert.Equal(t, wdk.GetStatusSuccess, res.Status)
	require.Len(t, res.Results, len(txIDs))

	for i, got := range res.Results {
		assert.Equal(t, txIDs[i], got.TxID)
		require.NotNil(t, got.Depth)
		assert.Equal(t, 10, *got.Depth)
		assert.Equal(t, "mined", got.Status)
	}
}
