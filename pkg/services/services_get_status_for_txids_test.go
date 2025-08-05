package services_test

import (
	"context"
	"net/http"
	"testing"

	ts "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalletServices_GetStatusForTxids_Success_Single(t *testing.T) {
	// given:
	fix := ts.GivenServices(t)
	txid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	fix.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, ts.TxStatusExpectation{
		ExpectBlockHash:   "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		ExpectBlockHeight: 777777,
	})

	svc := fix.Services().WithDefaultConfig()

	// when:
	res, err := svc.GetStatusForTxids(t.Context(), []string{txid})

	// then:
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, whatsonchain.ServiceName, res.Name)
	assert.Equal(t, wdk.GetStatusSuccess, res.Status)
	require.Len(t, res.Results, 1)

	item := res.Results[0]
	assert.Equal(t, txid, item.TxID)
	require.NotNil(t, item.Depth)
	assert.Equal(t, 10, *item.Depth)
	assert.Equal(t, "mined", item.Status)
}

func TestWalletServices_GetStatusForTxids_Success_Multiple(t *testing.T) {
	// given:
	fix := ts.GivenServices(t)
	txids := []string{
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	}

	fix.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, ts.TxStatusExpectation{
		ExpectBlockHash:   "feedfacefeedfacefeedfacefeedfacefeedfacefeedfacefeedfacefeedface",
		ExpectBlockHeight: 123456,
	})

	svc := fix.Services().WithDefaultConfig()

	// when:
	res, err := svc.GetStatusForTxids(t.Context(), txids)

	// then:
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, whatsonchain.ServiceName, res.Name)
	assert.Equal(t, wdk.GetStatusSuccess, res.Status)
	require.Len(t, res.Results, len(txids))

	for i, it := range res.Results {
		assert.Equal(t, txids[i], it.TxID)
		require.NotNil(t, it.Depth)
		assert.Equal(t, 10, *it.Depth)
		assert.Equal(t, "mined", it.Status)
	}
}

func TestWalletServices_GetStatusForTxids_Error_NoTxids(t *testing.T) {
	// given:
	fix := ts.GivenServices(t)
	svc := fix.Services().WithDefaultConfig()

	// when:
	res, err := svc.GetStatusForTxids(t.Context(), nil)

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "no txids provided")
}

func TestWalletServices_GetStatusForTxids_Error_HTTP500(t *testing.T) {
	// given:
	fix := ts.GivenServices(t)
	fix.WhatsOnChain().WillRespondOnTxStatus(http.StatusInternalServerError, ts.TxStatusExpectation{})
	svc := fix.Services().WithDefaultConfig()

	// when:
	res, err := svc.GetStatusForTxids(t.Context(), []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "failed to get status for txids")
}

func TestWalletServices_GetStatusForTxids_Error_Unreachable(t *testing.T) {
	// given:
	fix := ts.GivenServices(t)
	_ = fix.WhatsOnChain().WillBeUnreachable()
	svc := fix.Services().WithDefaultConfig()

	// when:
	res, err := svc.GetStatusForTxids(t.Context(), []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "failed to get status for txids")
}

func TestWalletServices_GetStatusForTxids_ContextCancelled(t *testing.T) {
	// given:
	fix := ts.GivenServices(t)
	ctx, cancel := context.WithCancelCause(t.Context())

	// Cancel the context when WoC endpoint is hit
	pat := `=~.*/txs/status$`
	fix.WhatsOnChain().Transport().RegisterResponder(http.MethodPost, pat,
		func(_ *http.Request) (*http.Response, error) {
			cancel(context.Canceled)
			return nil, context.Canceled
		})

	svc := fix.Services().WithDefaultConfig()

	// when:
	res, err := svc.GetStatusForTxids(ctx, []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
	require.ErrorIs(t, err, context.Canceled)
}

func TestWalletServices_GetStatusForTxids_Success_Single_Bitails(t *testing.T) {
	// given:
	fix := ts.GivenServices(t)
	txid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	// Make WoC fail so Bitails is used
	_ = fix.WhatsOnChain().WillBeUnreachable()

	// Bitails chain tip and mined tx at height -> depth = tip - height + 1 = 10
	fix.Bitails().WillReturnNetworkInfo(http.StatusOK, 1000)
	fix.Bitails().WillReturnTxStatusMined(txid, 991)

	svc := fix.Services().WithDefaultConfig()

	// when:
	res, err := svc.GetStatusForTxids(t.Context(), []string{txid})

	// then:
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, bitails.ServiceName, res.Name)
	assert.Equal(t, wdk.GetStatusSuccess, res.Status)
	require.Len(t, res.Results, 1)

	item := res.Results[0]
	assert.Equal(t, txid, item.TxID)
	require.NotNil(t, item.Depth)
	assert.Equal(t, 10, *item.Depth)
	assert.Equal(t, "mined", item.Status)
}

func TestWalletServices_GetStatusForTxids_Success_Multiple_Bitails(t *testing.T) {
	// given:
	fix := ts.GivenServices(t)
	txids := []string{
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	}

	// Force Bitails path
	_ = fix.WhatsOnChain().WillBeUnreachable()

	// tip 105; mined at 100 -> depth 6
	fix.Bitails().WillReturnNetworkInfo(http.StatusOK, 105)
	for _, txid := range txids {
		fix.Bitails().WillReturnTxStatusMined(txid, 100)
	}

	svc := fix.Services().WithDefaultConfig()

	// when:
	res, err := svc.GetStatusForTxids(t.Context(), txids)

	// then:
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, bitails.ServiceName, res.Name)
	assert.Equal(t, wdk.GetStatusSuccess, res.Status)
	require.Len(t, res.Results, len(txids))

	for i, it := range res.Results {
		assert.Equal(t, txids[i], it.TxID)
		require.NotNil(t, it.Depth)
		assert.Equal(t, 6, *it.Depth)
		assert.Equal(t, "mined", it.Status)
	}
}

func TestWalletServices_GetStatusForTxids_Bitails_Mixed(t *testing.T) {
	// given:
	fix := ts.GivenServices(t)
	_ = fix.WhatsOnChain().WillBeUnreachable()

	minedTx := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	unconfTx := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	notFoundTx := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

	fix.Bitails().WillReturnNetworkInfo(http.StatusOK, 105)
	fix.Bitails().WillReturnTxStatusMined(minedTx, 100)
	fix.Bitails().WillReturnTxStatusUnconfirmed(unconfTx)
	fix.Bitails().WillReturnTxStatusNotFound(notFoundTx)

	svc := fix.Services().WithDefaultConfig()

	// when:
	res, err := svc.GetStatusForTxids(t.Context(), []string{minedTx, unconfTx, notFoundTx})

	// then:
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, bitails.ServiceName, res.Name)
	assert.Equal(t, wdk.GetStatusSuccess, res.Status)
	require.Len(t, res.Results, 3)

	got := res.Results[0]
	assert.Equal(t, minedTx, got.TxID)
	assert.Equal(t, "mined", got.Status)
	require.NotNil(t, got.Depth)
	assert.Equal(t, 6, *got.Depth)

	got = res.Results[1]
	assert.Equal(t, unconfTx, got.TxID)
	assert.Equal(t, "known", got.Status)
	require.NotNil(t, got.Depth)
	assert.Equal(t, 0, *got.Depth)

	got = res.Results[2]
	assert.Equal(t, notFoundTx, got.TxID)
	assert.Equal(t, "unknown", got.Status)
	assert.Nil(t, got.Depth)
}

func TestWalletServices_GetStatusForTxids_Bitails_NoStatusFound(t *testing.T) {
	// given:
	fix := ts.GivenServices(t)
	txids := []string{
		"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	}

	err := fix.WhatsOnChain().WillBeUnreachable()
	require.Error(t, err)

	fix.Bitails().WillReturnNetworkInfo(http.StatusOK, 200)
	for _, tx := range txids {
		fix.Bitails().WillReturnTxStatusNotFound(tx)
	}

	svc := fix.Services().WithDefaultConfig()

	// when:
	res, err := svc.GetStatusForTxids(t.Context(), txids)

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "no status found for provided txids")
}

func TestWalletServices_GetStatusForTxids_Bitails_AllProvidersFail(t *testing.T) {
	// given:
	fix := ts.GivenServices(t)

	err := fix.WhatsOnChain().WillBeUnreachable()
	require.Error(t, err)
	fix.Bitails().WillReturnNetworkInfo(http.StatusInternalServerError, 0)

	svc := fix.Services().WithDefaultConfig()

	// when:
	res, err := svc.GetStatusForTxids(t.Context(), []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "failed to get status for txids")
}

func TestWalletServices_GetStatusForTxids_Bitails_ContextCancelled(t *testing.T) {
	// given:
	fix := ts.GivenServices(t)
	err := fix.WhatsOnChain().WillBeUnreachable()
	require.Error(t, err)

	ctx, cancel := context.WithCancelCause(t.Context())
	pat := `=~.*?/network/info$`
	fix.Bitails().Transport().RegisterResponder(http.MethodGet, pat,
		func(_ *http.Request) (*http.Response, error) {
			cancel(context.Canceled)
			return nil, context.Canceled
		})

	svc := fix.Services().WithDefaultConfig()

	// when:
	res, err := svc.GetStatusForTxids(ctx, []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
	require.ErrorIs(t, err, context.Canceled)
}
