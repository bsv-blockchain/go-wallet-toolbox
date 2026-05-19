package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	clients "github.com/bsv-blockchain/go-sdk/auth/clients/authhttp"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/slogx"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const contentTypeJSON = "application/json"

// NewClient returns a WalletStorageProviderClient that speaks the V1 storage adapter
// HTTP contract (/storage/v1/*) using authenticated requests via the go-sdk auth client.
// This replaces the legacy JSON-RPC implementation (now deprecated).
// The returned cleanup func is a no-op (no persistent connection).
func NewClient(addr string, wallet sdk.Interface, opts ...ClientOptions) (*WalletStorageProviderClient, func(), error) {
	options := to.OptionsWithDefault(defaultClientOptions(), opts...)
	options.logger = logging.Child(options.logger, "StorageClient")

	requester := newRPCAuthriteRequester(addr, wallet, options)

	impl := newV1Impl(requester)

	c := &WalletStorageProviderClient{
		client: impl,
	}

	// No persistent connection in V1 HTTP model — each request is a one-shot
	// HTTP call so there is nothing to tear down.
	cleanup := func() {
		// intentionally empty: see comment above
	}

	return c, cleanup, nil
}

// newV1Impl wires the V1 HTTP storage adapter operations to the requester.
// Each op is extracted to a standalone builder to keep complexity per-function low.
func newV1Impl(r *rpcAuthriteRequester) *rpcWalletStorageProvider {
	return &rpcWalletStorageProvider{
		Migrate:                   v1Migrate(r),
		MakeAvailable:             v1MakeAvailable(r),
		SetActive:                 v1SetActive(),
		FindOrInsertUser:          v1FindOrInsertUser(r),
		InternalizeAction:         v1InternalizeAction(r),
		CreateAction:              v1CreateAction(r),
		ProcessAction:             v1ProcessAction(r),
		InsertCertificateAuth:     v1InsertCertificateAuth(r),
		RelinquishCertificate:     v1RelinquishCertificate(r),
		RelinquishOutput:          v1RelinquishOutput(r),
		ListCertificates:          v1ListCertificates(r),
		ListOutputs:               v1ListOutputs(r),
		ListActions:               v1ListActions(r),
		GetSyncChunk:              v1GetSyncChunk(r),
		FindOrInsertSyncStateAuth: v1FindOrInsertSyncStateAuth(r),
		ProcessSyncChunk:          v1ProcessSyncChunk(),
		AbortAction:               v1AbortAction(r),
		FindOutputBasketsAuth:     v1FindOutputBasketsAuth(),
		FindOutputsAuth:           v1FindOutputsAuth(),
		ListTransactions:          v1ListTransactions(),
	}
}

// postArgs sends `{args: <args>}` to path and decodes the response into *R.
func postArgs[A any, R any](r *rpcAuthriteRequester, ctx context.Context, path string, args A) (*R, error) {
	var res R
	payload := struct {
		Args A `json:"args"`
	}{Args: args}
	if err := r.postV1(ctx, path, payload, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// postArgsNoResult sends `{args: <args>}` to path and discards the response body shape.
func postArgsNoResult[A any](r *rpcAuthriteRequester, ctx context.Context, path string, args A) error {
	payload := struct {
		Args A `json:"args"`
	}{Args: args}
	var res map[string]string
	return r.postV1(ctx, path, payload, &res)
}

func v1Migrate(r *rpcAuthriteRequester) func(ctx context.Context, storageName, storageIdentityKey string) (string, error) {
	return func(ctx context.Context, storageName, storageIdentityKey string) (string, error) {
		var res struct {
			StorageName string `json:"storageName"`
		}
		payload := map[string]string{"storageName": storageName, "storageIdentityKey": storageIdentityKey}
		if err := r.postV1(ctx, "/storage/v1/migrate", payload, &res); err != nil {
			return "", err
		}
		return res.StorageName, nil
	}
}

func v1MakeAvailable(r *rpcAuthriteRequester) func(ctx context.Context) (*wdk.TableSettings, error) {
	return func(ctx context.Context) (*wdk.TableSettings, error) {
		var res wdk.TableSettings
		if err := r.getV1(ctx, "/storage/v1/settings", &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
}

func v1SetActive() func(ctx context.Context, auth wdk.AuthID, newActiveStorageIdentityKey string) error {
	return func(ctx context.Context, auth wdk.AuthID, newActiveStorageIdentityKey string) error {
		return fmt.Errorf("SetActive not implemented in V1 client yet")
	}
}

func v1FindOrInsertUser(r *rpcAuthriteRequester) func(ctx context.Context, identityKey string) (*wdk.FindOrInsertUserResponse, error) {
	return func(ctx context.Context, identityKey string) (*wdk.FindOrInsertUserResponse, error) {
		var res wdk.FindOrInsertUserResponse
		payload := map[string]string{"identityKey": identityKey}
		if err := r.postV1(ctx, "/storage/v1/users", payload, &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
}

func v1InternalizeAction(r *rpcAuthriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.InternalizeActionArgs) (*wdk.InternalizeActionResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.InternalizeActionArgs) (*wdk.InternalizeActionResult, error) {
		var res wdk.InternalizeActionResult
		payload := struct {
			IdentityKey string                    `json:"identityKey,omitempty"`
			Args        wdk.InternalizeActionArgs `json:"args"`
		}{IdentityKey: auth.IdentityKey, Args: args}
		if err := r.postV1(ctx, "/storage/v1/actions/internalize", payload, &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
}

func v1CreateAction(r *rpcAuthriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, error) {
		return postArgs[wdk.ValidCreateActionArgs, wdk.StorageCreateActionResult](r, ctx, "/storage/v1/actions", args)
	}
}

func v1ProcessAction(r *rpcAuthriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.ProcessActionArgs) (*wdk.ProcessActionResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.ProcessActionArgs) (*wdk.ProcessActionResult, error) {
		return postArgs[wdk.ProcessActionArgs, wdk.ProcessActionResult](r, ctx, "/storage/v1/actions/process", args)
	}
}

func v1InsertCertificateAuth(r *rpcAuthriteRequester) func(ctx context.Context, auth wdk.AuthID, certificate *wdk.TableCertificateX) (uint, error) {
	return func(ctx context.Context, auth wdk.AuthID, certificate *wdk.TableCertificateX) (uint, error) {
		var res struct {
			CertificateID uint `json:"certificateId"`
			ID            uint `json:"id"`
		}
		if err := r.postV1(ctx, "/storage/v1/certificates", certificate, &res); err != nil {
			return 0, err
		}
		if res.CertificateID != 0 {
			return res.CertificateID, nil
		}
		return res.ID, nil
	}
}

func v1RelinquishCertificate(r *rpcAuthriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.RelinquishCertificateArgs) error {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.RelinquishCertificateArgs) error {
		return postArgsNoResult[wdk.RelinquishCertificateArgs](r, ctx, "/storage/v1/certificates/relinquish", args)
	}
}

func v1RelinquishOutput(r *rpcAuthriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.RelinquishOutputArgs) error {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.RelinquishOutputArgs) error {
		return postArgsNoResult[wdk.RelinquishOutputArgs](r, ctx, "/storage/v1/outputs/relinquish", args)
	}
}

func v1ListCertificates(r *rpcAuthriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.ListCertificatesArgs) (*wdk.ListCertificatesResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.ListCertificatesArgs) (*wdk.ListCertificatesResult, error) {
		return postArgs[wdk.ListCertificatesArgs, wdk.ListCertificatesResult](r, ctx, "/storage/v1/list/certificates", args)
	}
}

func v1ListOutputs(r *rpcAuthriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.ListOutputsArgs) (*wdk.ListOutputsResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.ListOutputsArgs) (*wdk.ListOutputsResult, error) {
		return postArgs[wdk.ListOutputsArgs, wdk.ListOutputsResult](r, ctx, "/storage/v1/list/outputs", args)
	}
}

func v1ListActions(r *rpcAuthriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.ListActionsArgs) (*wdk.ListActionsResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.ListActionsArgs) (*wdk.ListActionsResult, error) {
		return postArgs[wdk.ListActionsArgs, wdk.ListActionsResult](r, ctx, "/storage/v1/list/actions", args)
	}
}

func v1GetSyncChunk(r *rpcAuthriteRequester) func(ctx context.Context, args wdk.RequestSyncChunkArgs) (*wdk.SyncChunk, error) {
	return func(ctx context.Context, args wdk.RequestSyncChunkArgs) (*wdk.SyncChunk, error) {
		return postArgs[wdk.RequestSyncChunkArgs, wdk.SyncChunk](r, ctx, "/storage/v1/sync/chunk", args)
	}
}

func v1FindOrInsertSyncStateAuth(r *rpcAuthriteRequester) func(ctx context.Context, auth wdk.AuthID, storageIdentityKey, storageName string) (*wdk.FindOrInsertSyncStateAuthResponse, error) {
	return func(ctx context.Context, auth wdk.AuthID, storageIdentityKey, storageName string) (*wdk.FindOrInsertSyncStateAuthResponse, error) {
		var res wdk.FindOrInsertSyncStateAuthResponse
		payload := map[string]string{"storageIdentityKey": storageIdentityKey, "storageName": storageName}
		if err := r.postV1(ctx, "/storage/v1/sync/state", payload, &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
}

func v1ProcessSyncChunk() func(ctx context.Context, args wdk.RequestSyncChunkArgs, chunk *wdk.SyncChunk) (*wdk.ProcessSyncChunkResult, error) {
	return func(ctx context.Context, args wdk.RequestSyncChunkArgs, chunk *wdk.SyncChunk) (*wdk.ProcessSyncChunkResult, error) {
		return nil, fmt.Errorf("ProcessSyncChunk not fully implemented in V1 client")
	}
}

func v1AbortAction(r *rpcAuthriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.AbortActionArgs) (*wdk.AbortActionResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.AbortActionArgs) (*wdk.AbortActionResult, error) {
		return postArgs[wdk.AbortActionArgs, wdk.AbortActionResult](r, ctx, "/storage/v1/actions/abort", args)
	}
}

func v1FindOutputBasketsAuth() func(ctx context.Context, auth wdk.AuthID, filters wdk.FindOutputBasketsArgs) (wdk.TableOutputBaskets, error) {
	return func(ctx context.Context, auth wdk.AuthID, filters wdk.FindOutputBasketsArgs) (wdk.TableOutputBaskets, error) {
		return wdk.TableOutputBaskets{}, nil
	}
}

func v1FindOutputsAuth() func(ctx context.Context, auth wdk.AuthID, filters wdk.FindOutputsArgs) (wdk.TableOutputs, error) {
	return func(ctx context.Context, auth wdk.AuthID, filters wdk.FindOutputsArgs) (wdk.TableOutputs, error) {
		return wdk.TableOutputs{}, nil
	}
}

func v1ListTransactions() func(ctx context.Context, auth wdk.AuthID, args wdk.ListTransactionsArgs) (*wdk.ListTransactionsResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.ListTransactionsArgs) (*wdk.ListTransactionsResult, error) {
		return &wdk.ListTransactionsResult{}, nil
	}
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
	log := r.log.With(slog.Group(
		"req",
		slog.String("method", "POST"),
		slog.String("url", r.addr),
		slog.String("body", string(body)),
	))

	resp, err := r.httpClient.Fetch(ctx, r.addr, &clients.SimplifiedFetchRequestOptions{
		Method: http.MethodPost,
		Headers: map[string]string{
			"Content-Type": contentTypeJSON,
		},
		Body: body,
	})
	if err != nil {
		log.DebugContext(
			ctx, "Request to storage server failed",
			slogx.Error(err),
		)
		return nil, fmt.Errorf("storage client request failed: %w", err)
	}
	log.DebugContext(
		ctx, "Successfully sent request to storage server",
		slog.Any("resp", (*loggableResponse)(resp)),
	)

	return resp.Body, nil
}

// --- V1 adapter HTTP helpers (used by NewClient V1 implementation) ---

func (r *rpcAuthriteRequester) postV1(ctx context.Context, path string, payload any, result any) error {
	var bodyBytes []byte
	var err error
	if payload != nil {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal V1 payload: %w", err)
		}
	}

	respBody, err := r.doV1HTTPRequest(ctx, path, bodyBytes)
	if err != nil {
		return err
	}
	defer func() { _ = respBody.Close() }()

	data, err := io.ReadAll(respBody)
	if err != nil {
		return fmt.Errorf("failed to read V1 response body: %w", err)
	}

	// Check for error shape { "error": "..." }
	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &errResp) == nil && errResp.Error != "" {
		return fmt.Errorf("storage v1 error: %s", errResp.Error)
	}

	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			// If result unmarshal fails, try to surface the raw body in error
			return fmt.Errorf("failed to unmarshal V1 response (raw: %s): %w", string(data), err)
		}
	}
	return nil
}

func (r *rpcAuthriteRequester) getV1(ctx context.Context, path string, result any) error {
	// For GET, we can reuse similar but use GET method. For simplicity, use a GET variant.
	respBody, err := r.doV1HTTPRequestGet(ctx, path)
	if err != nil {
		return err
	}
	defer func() { _ = respBody.Close() }()

	data, err := io.ReadAll(respBody)
	if err != nil {
		return fmt.Errorf("failed to read V1 GET response: %w", err)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &errResp) == nil && errResp.Error != "" {
		return fmt.Errorf("storage v1 error: %s", errResp.Error)
	}

	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("failed to unmarshal V1 GET response: %w", err)
		}
	}
	return nil
}

func (r *rpcAuthriteRequester) doV1HTTPRequest(ctx context.Context, path string, body []byte) (io.ReadCloser, error) {
	target := buildV1Target(r.addr, path)
	log := r.log.With(slog.Group(
		"req",
		slog.String("method", "POST"),
		slog.String("url", target),
		slog.String("body", string(body)),
	))

	resp, err := r.httpClient.Fetch(ctx, target, &clients.SimplifiedFetchRequestOptions{
		Method: http.MethodPost,
		Headers: map[string]string{
			"Content-Type": contentTypeJSON,
		},
		Body: body,
	})
	if err != nil {
		log.DebugContext(
			ctx, "V1 request to storage server failed",
			slogx.Error(err),
		)
		return nil, fmt.Errorf("storage v1 client request failed: %w", err)
	}
	log.DebugContext(
		ctx, "Successfully sent V1 request to storage server",
		slog.Any("resp", (*loggableResponse)(resp)),
	)

	return resp.Body, nil
}

func (r *rpcAuthriteRequester) doV1HTTPRequestGet(ctx context.Context, path string) (io.ReadCloser, error) {
	target := buildV1Target(r.addr, path)
	log := r.log.With(slog.Group(
		"req",
		slog.String("method", "GET"),
		slog.String("url", target),
	))

	resp, err := r.httpClient.Fetch(ctx, target, &clients.SimplifiedFetchRequestOptions{
		Method: http.MethodGet,
	})
	if err != nil {
		log.DebugContext(
			ctx, "V1 GET request to storage server failed",
			slogx.Error(err),
		)
		return nil, fmt.Errorf("storage v1 client GET request failed: %w", err)
	}
	log.DebugContext(
		ctx, "Successfully sent V1 GET request to storage server",
		slog.Any("resp", (*loggableResponse)(resp)),
	)

	return resp.Body, nil
}

func buildV1Target(base, path string) string {
	if path == "" {
		return base
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	base = strings.TrimSuffix(base, "/")
	return base + path
}

const maxLogBodySize = 32 * 1024 // 32KB

type loggableResponse http.Response

func (r *loggableResponse) LogValue() slog.Value {
	attrs := make([]slog.Attr, 0, 4)
	attrs = append(
		attrs,
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
