package rpcserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/rpcserver"
)

func TestTracer(t *testing.T) {
	const expectedTimeMessage = "time="
	const expectedLevelInfoMessage = "level=INFO"
	const expectedRequestMessage = `msg="RPC request"`
	const expectedResultMessage = `msg="RPC result"`
	const expectedHandlerArgMessage = `handler=`

	// given:
	testWriter := logging.TestWriter{}
	logger := logging.New().WithLevel(defs.LogLevelDebug).WithHandler(defs.TextHandler, &testWriter).Logger()

	// given server:
	handler := &mockHandler{}
	rpcServer := rpcserver.NewRPCHandler(logger, "MockHandler", handler)

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
		lines := testWriter.Lines()

		msg := lines[0]
		assert.Contains(t, msg, expectedTimeMessage)
		assert.Contains(t, msg, expectedLevelInfoMessage)
		assert.Contains(t, msg, expectedRequestMessage)
		assert.Contains(t, msg, `method=get`)
		assert.Contains(t, msg, expectedHandlerArgMessage)

		msg2 := lines[1]
		assert.Contains(t, msg2, expectedTimeMessage)
		assert.Contains(t, msg2, expectedLevelInfoMessage)
		assert.Contains(t, msg2, expectedResultMessage)
		assert.Contains(t, msg2, `method=get`)
		assert.Contains(t, msg2, expectedHandlerArgMessage)
		assert.Contains(t, msg2, `result_0=10`)
	})

	t.Run("method with arguments and no result", func(t *testing.T) {
		defer testWriter.Clear()

		// when:
		client.Set(t.Context(), 10)

		// then:
		lines := testWriter.Lines()

		msg := lines[0]
		assert.Contains(t, msg, expectedTimeMessage)
		assert.Contains(t, msg, expectedLevelInfoMessage)
		assert.Contains(t, msg, expectedRequestMessage)
		assert.Contains(t, msg, `method=set`)
		assert.Contains(t, msg, expectedHandlerArgMessage)
		assert.Contains(t, msg, `param_0="<context: `)
		assert.Contains(t, msg, `param_1=10`)

		msg2 := lines[1]
		assert.Contains(t, msg2, expectedTimeMessage)
		assert.Contains(t, msg2, expectedLevelInfoMessage)
		assert.Contains(t, msg2, expectedResultMessage)
		assert.Contains(t, msg2, `method=set`)
		assert.Contains(t, msg2, expectedHandlerArgMessage)
	})
}

type mockHandler struct{}

func (h *mockHandler) Get() int {
	return 10
}

func (h *mockHandler) Set(context.Context, int) {
	// nothing to do, it's just for test case purposes
}

// mockClient matches the mockHandler (but on the client side)
type mockClient struct {
	Get func() int
	Set func(context.Context, int)
}
