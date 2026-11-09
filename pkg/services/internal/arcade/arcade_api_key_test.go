package arcade_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arcade"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// testAPIKey has the shape of an Arcade key (bsvb_ + 43 base62 + _ + 6 base62)
// but is not a real one.
const testAPIKey = "bsvb_0000000000000000000000000000000000000000000_000000"

// authRecorder is an Arcade stand-in that records the Authorization header of
// every request per path and answers with a canned response.
type authRecorder struct {
	mu      sync.Mutex
	headers map[string][]string
}

func newAuthRecorder() *authRecorder {
	return &authRecorder{headers: map[string][]string{}}
}

func (a *authRecorder) record(r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.headers[r.URL.Path] = append(a.headers[r.URL.Path], r.Header.Get("Authorization"))
}

func (a *authRecorder) seen(path string) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.headers[path]...)
}

// server answers every Arcade endpoint the client uses: POST /tx, GET /tx/{id},
// GET /health and a single-event SSE /events stream.
func (a *authRecorder) server(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.record(r)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/tx":
			writeJSON(t, w, http.StatusAccepted, arcade.TXInfo{TxID: testTxID, TxStatus: arcade.StatusSeenOnNetwork})
		case r.Method == http.MethodGet && r.URL.Path == "/tx/"+testTxID:
			writeJSON(t, w, http.StatusOK, arcade.TXInfo{TxID: testTxID, TxStatus: arcade.StatusSeenOnNetwork})
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/events":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("id: 1\nevent: status\ndata: {\"txid\":\"" + testTxID + "\",\"txStatus\":\"MINED\"}\n\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// exerciseEveryEndpoint drives each request path of the client once.
func exerciseEveryEndpoint(t *testing.T, service *arcade.Service) {
	t.Helper()

	posted, err := service.PostEF(t.Context(), testEFHex, testTxID)
	require.NoError(t, err)
	require.Equal(t, wdk.PostedTxIDResultSuccess, posted.Result)

	_, err = service.QueryTx(t.Context(), testTxID)
	require.NoError(t, err)

	require.True(t, service.Healthy(t.Context()))

	// the stream yields one event, then the test server closes it; cancel after
	// the first event so StreamEvents does not reconnect forever.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	got := make(chan arcade.StatusEvent, 1)
	go func() {
		_ = service.StreamEvents(ctx, "", func(ev arcade.StatusEvent) error {
			select {
			case got <- ev:
			default:
			}
			cancel()
			return nil
		})
	}()
	select {
	case ev := <-got:
		require.Equal(t, testTxID, ev.TxID)
	case <-ctx.Done():
		require.FailNow(t, "no event received from the SSE stream")
	}
}

func TestAPIKey(t *testing.T) {
	t.Run("configured key is sent as a bearer token on every endpoint", func(t *testing.T) {
		// given:
		recorder := newAuthRecorder()
		server := recorder.server(t)

		config := defaultConfig(server.URL)
		config.APIKey = testAPIKey
		service := newService(t, config)

		// when:
		exerciseEveryEndpoint(t, service)

		// then:
		for _, path := range []string{"/tx", "/tx/" + testTxID, "/health", "/events"} {
			seen := recorder.seen(path)
			require.NotEmpty(t, seen, "no request reached %s", path)
			for _, header := range seen {
				assert.Equal(t, "Bearer "+testAPIKey, header, "Authorization on %s", path)
			}
		}
	})

	t.Run("no Authorization header is sent when no key is configured", func(t *testing.T) {
		// given:
		recorder := newAuthRecorder()
		server := recorder.server(t)
		service := newService(t, defaultConfig(server.URL))

		// when:
		exerciseEveryEndpoint(t, service)

		// then:
		for _, path := range []string{"/tx", "/tx/" + testTxID, "/health", "/events"} {
			seen := recorder.seen(path)
			require.NotEmpty(t, seen, "no request reached %s", path)
			for _, header := range seen {
				assert.Empty(t, header, "Authorization on %s", path)
			}
		}
	})

	t.Run("key is never written to resty debug logs", func(t *testing.T) {
		// given:
		recorder := newAuthRecorder()
		server := recorder.server(t)

		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
		config := defaultConfig(server.URL)
		config.APIKey = testAPIKey
		service := arcade.New(logger, resty.New(), config)

		// when:
		_, err := service.PostEF(t.Context(), testEFHex, testTxID)
		require.NoError(t, err)

		// then: the request was logged (debug is on) but the key was not, and the
		// redaction did not strip the header from the request actually sent
		require.Contains(t, logs.String(), "Authorization")
		assert.NotContains(t, logs.String(), testAPIKey)
		assert.Equal(t, []string{"Bearer " + testAPIKey}, recorder.seen("/tx"))
	})
}

func TestAPIKeyRefused(t *testing.T) {
	// arcade answers a missing-but-required, malformed, unknown, revoked or
	// expired key with 401 and this body (services/apikey/middleware.go).
	refuse := func(t *testing.T, status int) *httptest.Server {
		t.Helper()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="api", error="invalid_token"`)
			writeJSON(t, w, status, map[string]string{"error": "invalid api key"})
		}))
		t.Cleanup(server.Close)
		return server
	}

	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status)+" on broadcast is a service failure, not a tx rejection", func(t *testing.T) {
			// given:
			server := refuse(t, status)
			config := defaultConfig(server.URL)
			config.APIKey = testAPIKey
			service := newService(t, config)

			// when:
			posted, err := service.PostEF(t.Context(), testEFHex, testTxID)

			// then: no per-tx result (that would be a verdict on the tx), an AuthError instead
			assert.Nil(t, posted)
			var authErr *arcade.AuthError
			require.ErrorAs(t, err, &authErr)
			assert.Equal(t, status, authErr.StatusCode)
			assert.True(t, authErr.APIKeyConfigured)
			assert.Contains(t, err.Error(), "check the configured arcade api_key")
			assert.Contains(t, err.Error(), "invalid api key")
		})
	}

	t.Run("401 without a configured key hints that the instance requires one", func(t *testing.T) {
		// given:
		server := refuse(t, http.StatusUnauthorized)
		service := newService(t, defaultConfig(server.URL))

		// when:
		_, err := service.PostEF(t.Context(), testEFHex, testTxID)

		// then:
		var authErr *arcade.AuthError
		require.ErrorAs(t, err, &authErr)
		assert.False(t, authErr.APIKeyConfigured)
		assert.Contains(t, err.Error(), "may require an api_key")
	})

	t.Run("401 on QueryTx is an AuthError, not 'not found'", func(t *testing.T) {
		// given:
		server := refuse(t, http.StatusUnauthorized)
		config := defaultConfig(server.URL)
		config.APIKey = testAPIKey
		service := newService(t, config)

		// when:
		info, err := service.QueryTx(t.Context(), testTxID)

		// then:
		assert.Nil(t, info)
		var authErr *arcade.AuthError
		require.ErrorAs(t, err, &authErr)
		assert.NotErrorIs(t, err, wdk.ErrNotFoundError)
	})
}

func TestRateLimited(t *testing.T) {
	t.Run("429 on broadcast is backpressure honoring Retry-After, not a tx rejection", func(t *testing.T) {
		// given: arcade's quota response (services/ratelimit)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "17")
			w.Header().Set("X-RateLimit-Reason", "quota")
			writeJSON(t, w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		}))
		t.Cleanup(server.Close)
		config := defaultConfig(server.URL)
		config.APIKey = testAPIKey
		service := newService(t, config)

		// when:
		posted, err := service.PostEF(t.Context(), testEFHex, testTxID)

		// then:
		assert.Nil(t, posted)
		var backpressure *arcade.BackpressureError
		require.ErrorAs(t, err, &backpressure)
		assert.Equal(t, 17*time.Second, backpressure.RetryAfter)
	})

	t.Run("4xx other than auth and rate limiting is still a tx rejection", func(t *testing.T) {
		// given:
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, http.StatusBadRequest, map[string]string{"error": "transaction failed validation"})
		}))
		t.Cleanup(server.Close)
		config := defaultConfig(server.URL)
		config.APIKey = testAPIKey
		service := newService(t, config)

		// when:
		posted, err := service.PostEF(t.Context(), testEFHex, testTxID)

		// then:
		require.NoError(t, err)
		require.NotNil(t, posted)
		assert.Equal(t, wdk.PostedTxIDResultError, posted.Result)
		require.Error(t, posted.Error)
		assert.Contains(t, posted.Error.Error(), "transaction failed validation")
	})
}
