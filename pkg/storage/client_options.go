package storage

import (
	"log/slog"
	"net/http"

	"github.com/filecoin-project/go-jsonrpc"
)

// NOTE: ClientOptions and rpcOptions are retained for API compatibility.
// The V1 HTTP client (now default in NewClient) ignores the legacy jsonrpc options.
// The old JSON-RPC transport (go-jsonrpc) has been deprecated in favor of the
// BRC-100 /storage/v1/* contract implemented by v1adapter.

// ClientOptions is a function that can be used to override internal dependencies.
// This is meant to be used for testing purposes.
type ClientOptions = func(*clientOptions)

type clientOptions struct {
	rpcOptions []jsonrpc.Option
	httpClient *http.Client
	logger     *slog.Logger
}

func defaultClientOptions() clientOptions {
	return clientOptions{
		rpcOptions: []jsonrpc.Option{
			jsonrpc.WithMethodNameFormatter(jsonrpc.NewMethodNameFormatter(false, jsonrpc.LowerFirstCharCase)),
		},
	}
}

// WithClientLogger is a function that can be used to set the logger for a client.
func WithClientLogger(logger *slog.Logger) ClientOptions {
	return func(o *clientOptions) {
		o.logger = logger
	}
}

// WithHttpClient is a function that can be used to override the http.Client used by the client.
// This is meant to be used for testing purposes.
func WithHttpClient(httpClient *http.Client) ClientOptions {
	return func(o *clientOptions) {
		o.httpClient = httpClient
	}
}
