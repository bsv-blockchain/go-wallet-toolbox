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
		t.Context(),
		testSrv.URL,
		"MockHandler",
		[]any{&client},
		nil,
		jsonrpc.WithMethodNameFormatter(jsonrpc.NewMethodNameFormatter(false, jsonrpc.LowerFirstCharCase)),
	)
	require.NoError(t, err)
	defer closer()

	t.Run("method with no arguments and single result", func(t *testing.T) {
		defer testWriter.Clear()

		// when:
		client.Get()

		// then:
		msg := testWriter.String()
		assert.Contains(t, msg, "time=")
		assert.Contains(t, msg, "level=INFO")
		assert.Contains(t, msg, `msg="Handling RPC call"`)
		assert.Contains(t, msg, `method=get`)
		assert.Contains(t, msg, `handler=`)
		assert.Contains(t, msg, `result_0=10`)
	})

	t.Run("method with arguments and no result", func(t *testing.T) {
		defer testWriter.Clear()

		// when:
		client.Set(t.Context(), 10)

		// then:
		msg := testWriter.String()
		assert.Contains(t, msg, "time=")
		assert.Contains(t, msg, "level=INFO")
		assert.Contains(t, msg, `msg="Handling RPC call"`)
		assert.Contains(t, msg, `method=set`)
		assert.Contains(t, msg, `handler=`)
		assert.Contains(t, msg, `param_0="<context: `)
		assert.Contains(t, msg, `param_1=10`)
	})
}

type mockHandler struct{}

func (h *mockHandler) Get() int {
	return 10
}

func (h *mockHandler) Set(context.Context, int) {
}

// mockClient matches the mockHandler (but on the client side)
type mockClient struct {
	Get func() int
	Set func(context.Context, int)
}
