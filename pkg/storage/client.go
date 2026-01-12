package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	clients "github.com/bsv-blockchain/go-sdk/auth/clients/authhttp"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/go-softwarelab/common/pkg/slogx"
	"github.com/go-softwarelab/common/pkg/to"
)

// NewClient returns WalletStorageWriterClient that allows connection to rpc server.
func NewClient(addr string, wallet sdk.Interface, opts ...ClientOptions) (*WalletStorageProviderClient, func(), error) {
	options := to.OptionsWithDefault(defaultClientOptions(), opts...)
	options.logger = logging.Child(options.logger, "StorageClient")

	client := &WalletStorageProviderClient{
		client: &rpcWalletStorageProvider{},
	}

	httpClient := newRPCAuthriteRequester(addr, wallet, options)

	cleanup, err := jsonrpc.NewCustomClient(
		"remote_storage",
		[]any{client.client},
		httpClient.DoHTTPRequest,
		options.rpcOptions...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize new RPC client: %w", err)
	}

	return client, cleanup, nil
}

func newRPCAuthriteRequester(addr string, wallet sdk.Interface, options clientOptions) *rpcAuthriteRequester {
	var opts []func(*clients.AuthFetchOptions)
	if options.httpClient != nil {
		opts = append(opts, clients.WithHttpClient(options.httpClient))
	}
	opts = append(opts, clients.WithLogger(options.logger))

	authFetch := clients.New(wallet, opts...)

	return &rpcAuthriteRequester{
		addr:       addr,
		log:        options.logger,
		httpClient: authFetch,
	}
}

type rpcAuthriteRequester struct {
	log        *slog.Logger
	httpClient *clients.AuthFetch
	addr       string
}

func (r *rpcAuthriteRequester) DoHTTPRequest(ctx context.Context, body []byte) (io.ReadCloser, error) {
	log := r.log.With(slog.Group("req",
		slog.String("method", "POST"),
		slog.String("url", r.addr),
		slog.String("body", string(body)),
	))

	resp, err := r.httpClient.Fetch(ctx, r.addr, &clients.SimplifiedFetchRequestOptions{
		Method: http.MethodPost,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: body,
	})
	if err != nil {
		log.DebugContext(ctx, "Request to storage server failed",
			slogx.Error(err),
		)
		return nil, fmt.Errorf("storage client request failed: %w", err)
	}
	log.DebugContext(ctx, "Successfully sent request to storage server",
		slog.Any("resp", (*loggableResponse)(resp)),
	)

	return resp.Body, nil
}

const maxLogBodySize = 32 * 1024 // 32KB

type loggableResponse http.Response

func (r *loggableResponse) LogValue() slog.Value {
	attrs := make([]slog.Attr, 0, 4)
	attrs = append(attrs,
		slog.Int("statusCode", r.StatusCode),
		slog.String("status", r.Status),
	)
	attrs = append(attrs, r.bodyLogAttributes()...)

	return slog.GroupValue(attrs...)
}

func (r *loggableResponse) bodyLogAttributes() []slog.Attr {
	var attrs []slog.Attr

	if r.ContentLength == 0 {
		return attrs
	}

	if r.ContentLength < 0 {
		attrs = append(attrs, slog.String("body", "<TRUNCATED: Unknown size>"))
		return attrs
	}

	if r.ContentLength > maxLogBodySize {
		attrs = append(attrs, slog.String("body", "<TRUNCATED: Too large>"))
		return attrs
	}

	bodyReader := r.Body
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		attrs = append(attrs, slog.String("body", "<ERROR: failed to read response body>"))
		attrs = append(attrs, slogx.Error(fmt.Errorf("failed to read response body during logging: %w", err)))
		return attrs
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	attrs = append(attrs, slog.String("body", string(body)))

	err = bodyReader.Close()
	if err != nil {
		attrs = append(attrs, slogx.Error(fmt.Errorf("failed to close response body during logging: %w", err)))
	}

	return attrs
}
