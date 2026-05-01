package certifier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bsv-blockchain/go-sdk/auth/brc104"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

func TestHandleSignCertificateRejectsOversizedRequestBody(t *testing.T) {
	clientKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	server := &Server{
		config: &ServerConfig{
			Logger:              logging.NewTestLogger(t),
			MaxRequestBodyBytes: 8,
		},
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/signCertificate", strings.NewReader(`{"type":"`+strings.Repeat("A", 32)+`"}`))
	req.Header.Set(brc104.HeaderIdentityKey, clientKey.PubKey().ToDERHex())
	rec := httptest.NewRecorder()

	server.handleSignCertificate(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}
