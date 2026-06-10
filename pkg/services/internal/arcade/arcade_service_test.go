package arcade_test

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arcade"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	testTxID = "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b"

	// testEFHex is an opaque (but valid) hex fixture - the arcade client treats EF bytes opaquely.
	testEFHex = "0100000001abcdef0123456789abcdef0123456789abcdef0123456789abcdef012345678900000000025100ffffffff0100e1f505000000001976a914eb0bd5edba389198e73f8efabddfc61666969ff788ac00000000"

	testCallbackToken = "test-callback-token"
)

func defaultConfig(url string) defs.Arcade {
	return defs.Arcade{
		Enabled:           true,
		URL:               url,
		EventsURL:         url,
		CallbackToken:     testCallbackToken,
		FullStatusUpdates: true,
	}
}

func newService(t testing.TB, config defs.Arcade) *arcade.Service {
	t.Helper()
	return arcade.New(logging.NewTestLogger(t), resty.New(), config)
}

func writeJSON(t testing.TB, w http.ResponseWriter, statusCode int, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	// assert (not require): this runs in the HTTP handler goroutine where FailNow is illegal
	assert.NoError(t, json.NewEncoder(w).Encode(payload))
}

func TestPostEF(t *testing.T) {
	t.Run("broadcast single transaction", func(t *testing.T) {
		// given:
		expectedBody, err := hex.DecodeString(testEFHex)
		require.NoError(t, err)

		txInfo := arcade.TXInfo{
			TxID:      testTxID,
			TxStatus:  arcade.StatusSeenOnNetwork,
			Timestamp: "2026-06-10T12:00:00Z",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/tx", r.URL.Path)
			assert.Equal(t, "application/octet-stream", r.Header.Get("Content-Type"))
			assert.Equal(t, testCallbackToken, r.Header.Get("X-CallbackToken"))
			assert.Equal(t, "true", r.Header.Get("X-FullStatusUpdates"))

			body, readErr := io.ReadAll(r.Body)
			assert.NoError(t, readErr)
			assert.Equal(t, expectedBody, body)

			writeJSON(t, w, http.StatusAccepted, txInfo)
		}))
		defer server.Close()

		// and:
		service := newService(t, defaultConfig(server.URL))

		// when:
		res, err := service.PostEF(t.Context(), testEFHex, testTxID)

		// then:
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, wdk.PostedTxIDResultSuccess, res.Result)
		assert.Equal(t, testTxID, res.TxID)
		assert.False(t, res.DoubleSpend)
		require.NoError(t, res.Error)
		assert.Len(t, res.Notes, 1)

		var data arcade.TXInfo
		require.NoError(t, json.Unmarshal([]byte(res.Data), &data))
		assert.Equal(t, txInfo, data)
	})

	t.Run("rejected with competing txs results in double spend", func(t *testing.T) {
		// given:
		competingTxID := "27a53423aa3e5d5c46bf30be53a9998dd247daf758847f244f82d430be71de6e"

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, http.StatusAccepted, arcade.TXInfo{
				TxID:         testTxID,
				TxStatus:     arcade.StatusRejected,
				CompetingTxs: []string{competingTxID},
			})
		}))
		defer server.Close()

		// and:
		service := newService(t, defaultConfig(server.URL))

		// when:
		res, err := service.PostEF(t.Context(), testEFHex, testTxID)

		// then:
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, wdk.PostedTxIDResultError, res.Result)
		assert.True(t, res.DoubleSpend)
		assert.Equal(t, []string{competingTxID}, res.CompetingTxs)
		require.Error(t, res.Error)
		assert.Len(t, res.Notes, 1)
	})

	t.Run("rejected without competing txs is an error but NOT a double spend", func(t *testing.T) {
		// given: a bare REJECTED (e.g. policy/fee rejection) without any conflict
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, http.StatusAccepted, arcade.TXInfo{
				TxID:     testTxID,
				TxStatus: arcade.StatusRejected,
			})
		}))
		defer server.Close()

		// and:
		service := newService(t, defaultConfig(server.URL))

		// when:
		res, err := service.PostEF(t.Context(), testEFHex, testTxID)

		// then: rejection is reported as an error result...
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, wdk.PostedTxIDResultError, res.Result)
		require.Error(t, res.Error)
		assert.Contains(t, res.Error.Error(), string(arcade.StatusRejected))
		assert.Contains(t, res.Data, string(arcade.StatusRejected))

		// ...but without double-spend evidence (no competing txs)
		assert.False(t, res.DoubleSpend)
		assert.Empty(t, res.CompetingTxs)
		assert.Len(t, res.Notes, 1)
	})

	t.Run("validation error 400 results in error result without transport error", func(t *testing.T) {
		// given:
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, http.StatusBadRequest, map[string]string{"error": "empty transaction"})
		}))
		defer server.Close()

		// and:
		service := newService(t, defaultConfig(server.URL))

		// when:
		res, err := service.PostEF(t.Context(), testEFHex, testTxID)

		// then:
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, wdk.PostedTxIDResultError, res.Result)
		require.Error(t, res.Error)
		assert.Contains(t, res.Error.Error(), "empty transaction")
		assert.Len(t, res.Notes, 1)
	})

	t.Run("503 with Retry-After header returns BackpressureError", func(t *testing.T) {
		// given:
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		// and:
		service := newService(t, defaultConfig(server.URL))

		// when:
		res, err := service.PostEF(t.Context(), testEFHex, testTxID)

		// then:
		require.Error(t, err)
		assert.Nil(t, res)

		var backpressureErr *arcade.BackpressureError
		require.ErrorAs(t, err, &backpressureErr)
		assert.Equal(t, 7*time.Second, backpressureErr.RetryAfter)
	})

	t.Run("503 without Retry-After header defaults to 5 seconds", func(t *testing.T) {
		// given:
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		// and:
		service := newService(t, defaultConfig(server.URL))

		// when:
		res, err := service.PostEF(t.Context(), testEFHex, testTxID)

		// then:
		require.Error(t, err)
		assert.Nil(t, res)

		var backpressureErr *arcade.BackpressureError
		require.ErrorAs(t, err, &backpressureErr)
		assert.Equal(t, 5*time.Second, backpressureErr.RetryAfter)
	})

	t.Run("503 with Retry-After HTTP date in the past defaults to 5 seconds", func(t *testing.T) {
		// given: a Retry-After date already in the past (clock skew)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		// and:
		service := newService(t, defaultConfig(server.URL))

		// when:
		res, err := service.PostEF(t.Context(), testEFHex, testTxID)

		// then:
		require.Error(t, err)
		assert.Nil(t, res)

		var backpressureErr *arcade.BackpressureError
		require.ErrorAs(t, err, &backpressureErr)
		assert.Equal(t, 5*time.Second, backpressureErr.RetryAfter)
	})

	t.Run("connection refused returns transport error", func(t *testing.T) {
		// given:
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		url := server.URL
		server.Close()

		// and:
		service := newService(t, defaultConfig(url))

		// when:
		res, err := service.PostEF(t.Context(), testEFHex, testTxID)

		// then:
		require.Error(t, err)
		assert.Nil(t, res)
	})

	invalidEFTestCases := map[string]string{
		"return error result on empty ef hex":              "",
		"return error result on invalid ef hex characters": "abc-not-hex",
	}
	for name, efHex := range invalidEFTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Fail(t, "arcade should not be called for invalid ef hex")
			}))
			defer server.Close()

			// and:
			service := newService(t, defaultConfig(server.URL))

			// when:
			res, err := service.PostEF(t.Context(), efHex, testTxID)

			// then:
			require.NoError(t, err)
			require.NotNil(t, res)

			assert.Equal(t, wdk.PostedTxIDResultError, res.Result)
			require.Error(t, res.Error)
			assert.Len(t, res.Notes, 1)
		})
	}
}

func TestQueryTx(t *testing.T) {
	t.Run("query known transaction", func(t *testing.T) {
		// given:
		txInfo := arcade.TXInfo{
			TxID:        testTxID,
			TxStatus:    arcade.StatusMined,
			Timestamp:   "2026-06-10T12:00:00Z",
			BlockHash:   "000000000000000001a7aa3999410ca53fb645851531ec0a7a5cb9ce2d4ae313",
			BlockHeight: 800_000,
			ExtraInfo:   "some extra info",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/tx/"+testTxID, r.URL.Path)

			writeJSON(t, w, http.StatusOK, txInfo)
		}))
		defer server.Close()

		// and:
		service := newService(t, defaultConfig(server.URL))

		// when:
		res, err := service.QueryTx(t.Context(), testTxID)

		// then:
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, txInfo, *res)
	})

	t.Run("unknown transaction returns not found error", func(t *testing.T) {
		// given:
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, http.StatusNotFound, map[string]string{"error": "transaction not found"})
		}))
		defer server.Close()

		// and:
		service := newService(t, defaultConfig(server.URL))

		// when:
		res, err := service.QueryTx(t.Context(), testTxID)

		// then:
		require.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, wdk.ErrNotFoundError)
	})
}

func TestHealthy(t *testing.T) {
	t.Run("healthy arcade returns true", func(t *testing.T) {
		// given:
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/health", r.URL.Path)
			writeJSON(t, w, http.StatusOK, map[string]string{"status": "ok"})
		}))
		defer server.Close()

		// and:
		service := newService(t, defaultConfig(server.URL))

		// when & then:
		assert.True(t, service.Healthy(t.Context()))
	})

	t.Run("failing arcade returns false", func(t *testing.T) {
		// given:
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		// and:
		service := newService(t, defaultConfig(server.URL))

		// when & then:
		assert.False(t, service.Healthy(t.Context()))
	})

	t.Run("unreachable arcade returns false", func(t *testing.T) {
		// given:
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		url := server.URL
		server.Close()

		// and:
		service := newService(t, defaultConfig(url))

		// when & then:
		assert.False(t, service.Healthy(t.Context()))
	})
}
