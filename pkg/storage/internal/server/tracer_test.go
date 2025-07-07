package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/server"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracer(t *testing.T) {
	// given:
	testWriter := logging.TestWriter{}
	logger := logging.New().WithLevel(defs.LogLevelDebug).WithHandler(defs.TextHandler, &testWriter).Logger()

	// given server:
	handler := &mockHandler{}
	rpcServer := server.NewRPCHandler(logger, "MockHandler", handler)

	mux := http.NewServeMux()
	rpcServer.Register(mux)

	testSrv := httptest.NewServer(mux)
	defer testSrv.Close()

	// and client:
	var client mockClient
	closer, err := jsonrpc.NewMergeClient(
		context.Background(),
		testSrv.URL,
		"MockHandler",
		[]any{&client},
		nil,
		jsonrpc.WithMethodNameFormatter(jsonrpc.NewMethodNameFormatter(false, jsonrpc.LowerFirstCharCase)),
	)
	require.NoError(t, err)
	defer closer()

	// when:
	val := client.Get()

	// then:
	require.Equal(t, 10, val)

	msg := testWriter.String()
	assert.Contains(t, msg, "time=")
	assert.Contains(t, msg, "level=INFO")
	assert.Contains(t, msg, `msg="Handling RPC call"`)
	assert.Contains(t, msg, `method=get`)
}

type mockHandler struct{}

func (h *mockHandler) Get() int {
	return 10
}

// mockClient matches the mockHandler (but on the client side)
type mockClient struct {
	Get func() int
}
