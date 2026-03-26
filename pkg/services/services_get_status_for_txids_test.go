package services_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestWalletServices_GetStatusForTxIDs_Success_Single(t *testing.T) {
	// given:
	fix := testservices.GivenServices(t)
	txid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	fix.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{
		ExpectBlockHash:   "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		ExpectBlockHeight: 777777,
	})

	svc := fix.Services().New()

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), []string{txid})

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

func TestWalletServices_GetStatusForTxIDs_Success_Multiple(t *testing.T) {
	// given:
	fix := testservices.GivenServices(t)
	txIDs := []string{
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	}

	fix.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{
		ExpectBlockHash:   "feedfacefeedfacefeedfacefeedfacefeedfacefeedfacefeedfacefeedface",
		ExpectBlockHeight: 123456,
	})

	svc := fix.Services().New()

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), txIDs)

	// then:
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, whatsonchain.ServiceName, res.Name)
	assert.Equal(t, wdk.GetStatusSuccess, res.Status)
	require.Len(t, res.Results, len(txIDs))

	for i, it := range res.Results {
		assert.Equal(t, txIDs[i], it.TxID)
		require.NotNil(t, it.Depth)
		assert.Equal(t, 10, *it.Depth)
		assert.Equal(t, "mined", it.Status)
	}
}

func TestWalletServices_GetStatusForTxIDs_Error_NoTxids(t *testing.T) {
	// given:
	fix := testservices.GivenServices(t)
	svc := fix.Services().New()

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), nil)

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "no txIDs provided")
}

func TestWalletServices_GetStatusForTxIDs_Error_HTTP500(t *testing.T) {
	// given:
	fix := testservices.GivenServices(t)
	fix.WhatsOnChain().WillRespondOnTxStatus(http.StatusInternalServerError, testservices.TxStatusExpectation{})
	svc := fix.Services().New()

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "failed to get status for txIDs")
}

func TestWalletServices_GetStatusForTxIDs_Error_Unreachable(t *testing.T) {
	// given:
	fix := testservices.GivenServices(t)
	_ = fix.WhatsOnChain().WillBeUnreachable()
	svc := fix.Services().New()

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "failed to get status for txIDs")
}

func TestWalletServices_GetStatusForTxIDs_ContextCancelled(t *testing.T) {
	// given:
	fix := testservices.GivenServices(t)
	ctx, cancel := context.WithCancelCause(t.Context())

	// Cancel the context when WoC endpoint is hit
	pat := `=~.*/txs/status$`
	fix.WhatsOnChain().Transport().RegisterResponder(http.MethodPost, pat,
		func(_ *http.Request) (*http.Response, error) {
			cancel(context.Canceled)
			return nil, context.Canceled
		})

	svc := fix.Services().New()

	// when:
	res, err := svc.GetStatusForTxIDs(ctx, []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
	require.ErrorIs(t, err, context.Canceled)
}

func TestWalletServices_GetStatusForTxIDs_Success_Single_Bitails(t *testing.T) {
	// given:
	fix := testservices.GivenServices(t)
	txid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	// Make WoC fail so Bitails is used
	_ = fix.WhatsOnChain().WillBeUnreachable()

	// Bitails chain tip and mined tx at height -> depth = tip - height + 1 = 10
	fix.Bitails().WillReturnNetworkInfo(http.StatusOK, 1000)
	fix.Bitails().WillReturnTxStatusMined(txid, 991)

	svc := fix.Services().Config(testservices.WithEnabledBitails(true)).New()

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), []string{txid})

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

func TestWalletServices_GetStatusForTxIDs_Success_Multiple_Bitails(t *testing.T) {
	// given:
	fix := testservices.GivenServices(t)
	txIDs := []string{
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	}

	// Force Bitails path
	_ = fix.WhatsOnChain().WillBeUnreachable()

	// tip 105; mined at 100 -> depth 6
	fix.Bitails().WillReturnNetworkInfo(http.StatusOK, 105)
	for _, txid := range txIDs {
		fix.Bitails().WillReturnTxStatusMined(txid, 100)
	}

	svc := fix.Services().Config(testservices.WithEnabledBitails(true)).New()

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), txIDs)

	// then:
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, bitails.ServiceName, res.Name)
	assert.Equal(t, wdk.GetStatusSuccess, res.Status)
	require.Len(t, res.Results, len(txIDs))

	for i, it := range res.Results {
		assert.Equal(t, txIDs[i], it.TxID)
		require.NotNil(t, it.Depth)
		assert.Equal(t, 6, *it.Depth)
		assert.Equal(t, "mined", it.Status)
	}
}

func TestWalletServices_GetStatusForTxIDs_Bitails_Mixed(t *testing.T) {
	// given:
	fix := testservices.GivenServices(t)
	_ = fix.WhatsOnChain().WillBeUnreachable()

	minedTx := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	unconfTx := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	notFoundTx := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

	fix.Bitails().WillReturnNetworkInfo(http.StatusOK, 105)
	fix.Bitails().WillReturnTxStatusMined(minedTx, 100)
	fix.Bitails().WillReturnTxStatusUnconfirmed(unconfTx)
	fix.Bitails().WillReturnTxStatusNotFound(notFoundTx)

	svc := fix.Services().Config(testservices.WithEnabledBitails(true)).New()

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), []string{minedTx, unconfTx, notFoundTx})

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

func TestWalletServices_GetStatusForTxIDs_Bitails_NoStatusFound(t *testing.T) {
	// given:
	fix := testservices.GivenServices(t)
	txIDs := []string{
		"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	}

	err := fix.WhatsOnChain().WillBeUnreachable()
	require.Error(t, err)

	fix.Bitails().WillReturnNetworkInfo(http.StatusOK, 200)
	for _, tx := range txIDs {
		fix.Bitails().WillReturnTxStatusNotFound(tx)
	}

	svc := fix.Services().Config(testservices.WithEnabledBitails(true)).New()

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), txIDs)

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "no status found for provided txIDs")
}

func TestWalletServices_GetStatusForTxIDs_Bitails_AllProvidersFail(t *testing.T) {
	// given:
	fix := testservices.GivenServices(t)

	err := fix.WhatsOnChain().WillBeUnreachable()
	require.Error(t, err)
	fix.Bitails().WillReturnNetworkInfo(http.StatusInternalServerError, 0)

	svc := fix.Services().Config(testservices.WithEnabledBitails(true)).New()

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "failed to get status for txIDs")
}

func TestWalletServices_GetStatusForTxIDs_Bitails_ContextCancelled(t *testing.T) {
	// given:
	fix := testservices.GivenServices(t)
	err := fix.WhatsOnChain().WillBeUnreachable()
	require.Error(t, err)

	ctx, cancel := context.WithCancelCause(t.Context())
	pat := `=~.*?/network/info$`
	fix.Bitails().Transport().RegisterResponder(http.MethodGet, pat,
		func(_ *http.Request) (*http.Response, error) {
			cancel(context.Canceled)
			return nil, context.Canceled
		})

	svc := fix.Services().Config(testservices.WithEnabledBitails(true)).New()

	// when:
	res, err := svc.GetStatusForTxIDs(ctx, []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})

	// then:
	require.Error(t, err)
	assert.Nil(t, res)
	require.ErrorIs(t, err, context.Canceled)
}
