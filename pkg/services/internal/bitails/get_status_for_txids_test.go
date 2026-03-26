package bitails_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bt "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/servicequeue"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestBitails_GetStatusForTxIDs_Mixed(t *testing.T) {
	// given:
	given := bt.Given(t)
	svc := given.NewBitailsService()

	tip := uint32(105)
	given.Bitails().WillReturnNetworkInfo(http.StatusOK, tip)

	minedTx := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	unconfTx := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	notFoundTx := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

	given.Bitails().WillReturnTxStatusMined(minedTx, 100)
	given.Bitails().WillReturnTxStatusUnconfirmed(unconfTx)
	given.Bitails().WillReturnTxStatusNotFound(notFoundTx)

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), []string{minedTx, unconfTx, notFoundTx})

	// then:
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "Bitails", res.Name)
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

func TestBitails_GetStatusForTxIDs_AllNotFound_ReturnsEmptyResult(t *testing.T) {
	// given:
	given := bt.Given(t)
	svc := given.NewBitailsService()
	given.Bitails().WillReturnNetworkInfo(http.StatusOK, 500_000)

	tx1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tx2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	given.Bitails().WillReturnTxStatusNotFound(tx1)
	given.Bitails().WillReturnTxStatusNotFound(tx2)

	// when:
	_, err := svc.GetStatusForTxIDs(t.Context(), []string{tx1, tx2})

	// then:
	require.ErrorIs(t, err, servicequeue.ErrEmptyResult)
}

func TestBitails_GetStatusForTxIDs_NoInput(t *testing.T) {
	// given:
	given := bt.Given(t)
	svc := given.NewBitailsService()

	// when:
	_, err := svc.GetStatusForTxIDs(t.Context(), nil)

	// then:
	require.Error(t, err)
	require.Contains(t, err.Error(), "no txIDs provided")
}

func TestBitails_GetStatusForTxIDs_TipHeightFailure(t *testing.T) {
	// given:
	given := bt.Given(t)
	svc := given.NewBitailsService()

	given.Bitails().WillReturnNetworkInfo(http.StatusBadGateway, 0)

	tx := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	// when:
	_, err := svc.GetStatusForTxIDs(t.Context(), []string{tx})

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get current height")
}

func TestBitails_GetStatusForTxIDs_PerItemHTTPError(t *testing.T) {
	// given:
	given := bt.Given(t)
	svc := given.NewBitailsService()
	given.Bitails().WillReturnNetworkInfo(http.StatusOK, 600_000)

	tx := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	given.Bitails().WillReturnTxStatusHttpError(tx, http.StatusInternalServerError)

	// when:
	_, err := svc.GetStatusForTxIDs(t.Context(), []string{tx})

	// then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get status for")
}

func TestBitails_GetStatusForTxIDs_NegativeConfirmationsClampedToZero(t *testing.T) {
	// given:
	given := bt.Given(t)
	svc := given.NewBitailsService()

	given.Bitails().WillReturnNetworkInfo(http.StatusOK, 100)
	tx := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	given.Bitails().WillReturnTxInfo(tx, "blockhash", 120)

	// when:
	res, err := svc.GetStatusForTxIDs(t.Context(), []string{tx})

	// then:
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Results, 1)

	got := res.Results[0]
	assert.Equal(t, tx, got.TxID)
	assert.Equal(t, "known", got.Status)
	require.NotNil(t, got.Depth)
	assert.Equal(t, 0, *got.Depth)
}
