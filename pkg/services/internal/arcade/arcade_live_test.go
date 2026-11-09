package arcade_test

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arcade"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Live tests against a real Arcade instance with API-key auth enabled. They are
// skipped unless ARCADE_LIVE_API_KEY is set, so they never run in CI:
//
//	ARCADE_LIVE_API_KEY=bsvb_... go test ./pkg/services/internal/arcade -run TestLiveAPIKey -v
//
// ARCADE_LIVE_URL overrides the instance (default https://broadcast.bsvblockchain.tech).
// They broadcast nothing spendable: the only POST is a transaction Arcade must
// reject, and every other request is a read. Bad-key requests are kept to one per
// run, well under Arcade's per-IP invalid-key throttle (60/min).
const defaultLiveArcadeURL = "https://broadcast.bsvblockchain.tech"

type liveArcade struct {
	url string
	key string

	mu       sync.Mutex
	policies []string
}

func givenLiveArcade(t *testing.T) *liveArcade {
	t.Helper()
	if testing.Short() {
		t.Skip("live Arcade test skipped in -short mode")
	}
	key := os.Getenv("ARCADE_LIVE_API_KEY")
	if key == "" {
		t.Skip("set ARCADE_LIVE_API_KEY to run live Arcade API-key tests")
	}
	url := os.Getenv("ARCADE_LIVE_URL")
	if url == "" {
		url = defaultLiveArcadeURL
	}
	return &liveArcade{url: strings.TrimRight(url, "/"), key: key}
}

// service builds the production client with apiKey configured. The resty client
// records each response's RateLimit-Policy header, which names the tier Arcade
// charged the request to ("anonymous" when no key was accepted).
func (l *liveArcade) service(t *testing.T, apiKey string) *arcade.Service {
	t.Helper()
	client := resty.New().OnAfterResponse(func(_ *resty.Client, r *resty.Response) error {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.policies = append(l.policies, r.Header().Get("RateLimit-Policy"))
		return nil
	})
	config := defaultConfig(l.url)
	config.CallbackToken = ""
	config.FullStatusUpdates = false
	config.APIKey = apiKey
	return arcade.New(logging.NewTestLogger(t), client, config)
}

func (l *liveArcade) lastPolicy() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.policies) == 0 {
		return ""
	}
	return l.policies[len(l.policies)-1]
}

func randomTxID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return hex.EncodeToString(b)
}

// assertKeyedTier checks Arcade charged the last request to the key's tier.
// A tier with no quota sends no RateLimit-Policy, which is also keyed.
func (l *liveArcade) assertKeyedTier(t *testing.T) {
	t.Helper()
	policy := l.lastPolicy()
	assert.False(t, strings.HasPrefix(policy, `"anonymous"`),
		"request was charged to the anonymous tier (RateLimit-Policy %q): the API key was not applied", policy)
}

func TestLiveAPIKey(t *testing.T) {
	live := givenLiveArcade(t)

	t.Run("health probe with the key succeeds", func(t *testing.T) {
		service := live.service(t, live.key)
		assert.True(t, service.Healthy(t.Context()))
	})

	t.Run("query of an unknown tx is authorized and charged to the key's tier", func(t *testing.T) {
		// given:
		service := live.service(t, live.key)

		// when:
		info, err := service.QueryTx(t.Context(), randomTxID(t))

		// then: 404 (not 401) - the key was accepted and the lookup ran
		assert.Nil(t, info)
		require.ErrorIs(t, err, wdk.ErrNotFoundError)
		var authErr *arcade.AuthError
		assert.NotErrorAs(t, err, &authErr, "key refused")
		live.assertKeyedTier(t)
	})

	t.Run("broadcast is authorized: Arcade judges the tx instead of refusing the key", func(t *testing.T) {
		// given: a tx without its inputs' source data, which Arcade must reject
		service := live.service(t, live.key)

		// when:
		posted, err := service.PostEF(t.Context(), testEFHex, testTxID)

		// then: a per-tx rejection (a verdict on the tx), not an AuthError
		require.NoError(t, err, "expected a tx-level rejection, got a service-level error")
		require.NotNil(t, posted)
		assert.Equal(t, wdk.PostedTxIDResultError, posted.Result)
		require.Error(t, posted.Error)
		assert.Contains(t, posted.Error.Error(), "arcade rejected tx")
		live.assertKeyedTier(t)
	})

	t.Run("an invalid key is refused with an AuthError", func(t *testing.T) {
		// given: a key with the right shape but a wrong checksum
		service := live.service(t, "bsvb_0000000000000000000000000000000000000000000_000000")

		// when:
		info, err := service.QueryTx(t.Context(), randomTxID(t))

		// then:
		assert.Nil(t, info)
		var authErr *arcade.AuthError
		require.ErrorAs(t, err, &authErr)
		assert.Equal(t, http.StatusUnauthorized, authErr.StatusCode)
		assert.True(t, authErr.APIKeyConfigured)
	})
}
