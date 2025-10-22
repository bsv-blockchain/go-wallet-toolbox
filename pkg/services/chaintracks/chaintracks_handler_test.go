package chaintracks_test

import (
	"net/http/httptest"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/require"
)

func TestChaintracksHandler_BasicEndpoints(t *testing.T) {
	// given:
	config := defs.DefaultChaintracksServiceConfig()
	handler, err := chaintracks.NewHandler(logging.NewTestLogger(t), config)
	require.NoError(t, err)

	testsrv := httptest.NewServer(handler.Handler())
	defer testsrv.Close()

	client := resty.NewWithClient(testsrv.Client())

	t.Run("GET /robots.txt", func(t *testing.T) {
		// when:
		resp, err := client.R().Get(testsrv.URL + "/robots.txt")

		// then:
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode())
		require.Equal(t, "text/plain", resp.Header().Get("Content-Type"))
		require.Equal(t, "User-agent: *\nDisallow: /", resp.String())
	})

	t.Run("GET /", func(t *testing.T) {
		// when:
		resp, err := client.R().Get(testsrv.URL + "/")

		// then:
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode())
		require.Equal(t, "text/plain", resp.Header().Get("Content-Type"))
		require.Equal(t, "Chaintracks mainNet Block Header Service", resp.String())
	})

	t.Run("GET /getChain", func(t *testing.T) {
		// when:
		resp, err := client.R().Get(testsrv.URL + "/getChain")

		// then:
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode())
		require.Equal(t, "application/json", resp.Header().Get("Content-Type"))
		require.Contains(t, resp.String(), `"value":"main"`)
	})
}
