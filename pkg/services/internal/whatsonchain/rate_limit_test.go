package whatsonchain_test

import (
	"net/http"
	"testing"
	"time"

	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
)

// All WoC requests must pass through the client-side rate limiter, so the WoC
// service-side limit (3 rps without an API key) is never exceeded - exceeding it
// yields 429 responses which under load turn into broadcast failures.
func TestWhatsOnChain_RateLimiterThrottlesRequests(t *testing.T) {
	// given:
	txSpec := testvectors.GivenTX().
		WithInput(100).
		WithP2PKHOutput(90)
	givenTxID := txSpec.TX().TxID().String()
	rawTx := txSpec.TX().Bytes()

	given := testabilities.Given(t)
	woc := given.NewWoCService(testabilities.WithRequestsPerSecond(4))

	given.WhatsOnChain().WillRespondWithBroadcast(http.StatusOK, `{"txid":"`+givenTxID+`"}`)
	given.WhatsOnChain().WillRespondOnTxStatus(http.StatusOK, testservices.TxStatusExpectation{})

	// when: 3 posts = 6 requests (each post makes a broadcast call and a tx-info call):
	start := time.Now()
	for range 3 {
		result, err := woc.PostTX(t.Context(), rawTx)
		require.NoError(t, err)
		require.NoError(t, result.Error)
	}
	elapsed := time.Since(start)

	// then: at 4 rps (burst 4), requests 5 and 6 wait for new tokens at 250ms intervals:
	assert.GreaterOrEqual(t, elapsed, 400*time.Millisecond,
		"requests must be throttled by the client-side rate limiter")
}
