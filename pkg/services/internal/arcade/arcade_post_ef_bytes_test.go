package arcade_test

import (
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arcade"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// PostEFBytes must answer exactly as PostEF does for the hex of the same bytes, including the
// broadcast note of a failed result, which carries the EF as hex.
func TestPostEFBytes_MatchesPostEF(t *testing.T) {
	efBytes, err := hex.DecodeString(testEFHex)
	require.NoError(t, err)

	responses := map[string]func(t *testing.T, w http.ResponseWriter){
		"accepted": func(t *testing.T, w http.ResponseWriter) {
			writeJSON(t, w, http.StatusAccepted, arcade.TXInfo{TxID: testTxID, TxStatus: arcade.StatusSeenOnNetwork})
		},
		"rejected as a double spend": func(t *testing.T, w http.ResponseWriter) {
			writeJSON(t, w, http.StatusAccepted, arcade.TXInfo{
				TxID:         testTxID,
				TxStatus:     arcade.StatusRejected,
				CompetingTxs: []string{"27a53423aa3e5d5c46bf30be53a9998dd247daf758847f244f82d430be71de6e"},
			})
		},
		"refused with a 400": func(t *testing.T, w http.ResponseWriter) {
			writeJSON(t, w, http.StatusBadRequest, map[string]any{"error": "bad tx"})
		},
	}

	for name, respond := range responses {
		t.Run(name, func(t *testing.T) {
			// given:
			var bodies [][]byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, readErr := io.ReadAll(r.Body)
				assert.NoError(t, readErr)
				bodies = append(bodies, body)
				respond(t, w)
			}))
			defer server.Close()
			service := newService(t, defaultConfig(server.URL))

			// when:
			want, wantErr := service.PostEF(t.Context(), testEFHex, testTxID)
			got, gotErr := service.PostEFBytes(t.Context(), efBytes, testTxID)

			// then: the same request body...
			require.Len(t, bodies, 2)
			assert.Equal(t, bodies[0], bodies[1])
			assert.Equal(t, efBytes, bodies[1])

			// ...and the same answer
			require.Equal(t, wantErr, gotErr)
			require.NotNil(t, got)
			assertSamePosted(t, want, got)
		})
	}

	t.Run("empty bytes are refused like an empty hex, without calling Arcade", func(t *testing.T) {
		// given:
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			assert.Fail(t, "arcade should not be called for an empty transaction")
		}))
		defer server.Close()
		service := newService(t, defaultConfig(server.URL))

		// when:
		want, wantErr := service.PostEF(t.Context(), "", testTxID)
		got, gotErr := service.PostEFBytes(t.Context(), nil, testTxID)

		// then:
		require.NoError(t, wantErr)
		require.NoError(t, gotErr)
		assert.Equal(t, wdk.PostedTxIDResultError, got.Result)
		assertSamePosted(t, want, got)
	})
}

func assertSamePosted(t *testing.T, want, got *wdk.PostedTxID) {
	t.Helper()
	assert.Equal(t, want.TxID, got.TxID)
	assert.Equal(t, want.Result, got.Result)
	assert.Equal(t, want.DoubleSpend, got.DoubleSpend)
	assert.Equal(t, want.CompetingTxs, got.CompetingTxs)
	assert.Equal(t, want.Data, got.Data)
	if want.Error == nil {
		require.NoError(t, got.Error)
	} else {
		require.EqualError(t, got.Error, want.Error.Error())
	}
	require.Len(t, got.Notes, len(want.Notes))
	for i := range want.Notes {
		assert.Equal(t, want.Notes[i].What, got.Notes[i].What)
		assert.Equal(t, want.Notes[i].Attributes, got.Notes[i].Attributes)
	}
}
