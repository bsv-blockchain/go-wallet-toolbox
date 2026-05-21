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

	requester := newAuthriteRequester(addr, wallet, options)

	impl := newClientImpl(requester)

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

// newClientImpl wires the V1 HTTP storage adapter operations to the requester.
// Each op is extracted to a standalone factory to keep complexity per-function low.
func newClientImpl(r *authriteRequester) *walletStorageProviderImpl {
	return &walletStorageProviderImpl{
		Migrate:                   migrate(r),
		MakeAvailable:             makeAvailable(r),
		SetActive:                 setActive(),
		FindOrInsertUser:          findOrInsertUser(r),
		InternalizeAction:         internalizeAction(r),
		CreateAction:              createAction(r),
		ProcessAction:             processAction(r),
		InsertCertificateAuth:     insertCertificateAuth(r),
		RelinquishCertificate:     relinquishCertificate(r),
		RelinquishOutput:          relinquishOutput(r),
		ListCertificates:          listCertificates(r),
		ListOutputs:               listOutputs(r),
		ListActions:               listActions(r),
		ListTransactions:          listTransactions(r),
		GetSyncChunk:              getSyncChunk(r),
		FindOrInsertSyncStateAuth: findOrInsertSyncStateAuth(r),
		ProcessSyncChunk:          processSyncChunk(),
		AbortAction:               abortAction(r),
		FindOutputBasketsAuth:     findOutputBasketsAuth(),
		FindOutputsAuth:           findOutputsAuth(),
	}
}

// postArgs sends `{args: <args>}` to path and decodes the response into *R.
func postArgs[A, R any](r *authriteRequester, ctx context.Context, path string, args A) (*R, error) {
	var res R
	payload := struct {
		Args A `json:"args"`
	}{Args: args}
	if err := r.post(ctx, path, payload, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// postArgsNoResult sends `{args: <args>}` to path and discards the response body shape.
func postArgsNoResult[A any](r *authriteRequester, ctx context.Context, path string, args A) error {
	payload := struct {
		Args A `json:"args"`
	}{Args: args}
	var res map[string]string
	return r.post(ctx, path, payload, &res)
}

func migrate(r *authriteRequester) func(ctx context.Context, storageName, storageIdentityKey string) (string, error) {
	return func(ctx context.Context, storageName, storageIdentityKey string) (string, error) {
		var res struct {
			StorageName string `json:"storageName"`
		}
		payload := map[string]string{"storageName": storageName, "storageIdentityKey": storageIdentityKey}
		if err := r.post(ctx, "/storage/v1/migrate", payload, &res); err != nil {
			return "", err
		}
		return res.StorageName, nil
	}
}

func makeAvailable(r *authriteRequester) func(ctx context.Context) (*wdk.TableSettings, error) {
	return func(ctx context.Context) (*wdk.TableSettings, error) {
		var res wdk.TableSettings
		if err := r.get(ctx, "/storage/v1/settings", &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
}

func setActive() func(ctx context.Context, auth wdk.AuthID, newActiveStorageIdentityKey string) error {
	return func(ctx context.Context, auth wdk.AuthID, newActiveStorageIdentityKey string) error {
		return fmt.Errorf("SetActive not implemented in V1 client yet")
	}
}

func findOrInsertUser(r *authriteRequester) func(ctx context.Context, identityKey string) (*wdk.FindOrInsertUserResponse, error) {
	return func(ctx context.Context, identityKey string) (*wdk.FindOrInsertUserResponse, error) {
		var res wdk.FindOrInsertUserResponse
		payload := map[string]string{"identityKey": identityKey}
		if err := r.post(ctx, "/storage/v1/users", payload, &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
}

func internalizeAction(r *authriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.InternalizeActionArgs) (*wdk.InternalizeActionResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.InternalizeActionArgs) (*wdk.InternalizeActionResult, error) {
		var res wdk.InternalizeActionResult
		payload := struct {
			IdentityKey string                    `json:"identityKey,omitempty"`
			Args        wdk.InternalizeActionArgs `json:"args"`
		}{IdentityKey: auth.IdentityKey, Args: args}
		if err := r.post(ctx, "/storage/v1/actions/internalize", payload, &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
}

func createAction(r *authriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, error) {
		return postArgs[wdk.ValidCreateActionArgs, wdk.StorageCreateActionResult](r, ctx, "/storage/v1/actions", args)
	}
}

func processAction(r *authriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.ProcessActionArgs) (*wdk.ProcessActionResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.ProcessActionArgs) (*wdk.ProcessActionResult, error) {
		return postArgs[wdk.ProcessActionArgs, wdk.ProcessActionResult](r, ctx, "/storage/v1/actions/process", args)
	}
}

func insertCertificateAuth(r *authriteRequester) func(ctx context.Context, auth wdk.AuthID, certificate *wdk.TableCertificateX) (uint, error) {
	return func(ctx context.Context, auth wdk.AuthID, certificate *wdk.TableCertificateX) (uint, error) {
		var res struct {
			CertificateID uint `json:"certificateId"`
			ID            uint `json:"id"`
		}
		if err := r.post(ctx, "/storage/v1/certificates", certificate, &res); err != nil {
			return 0, err
		}
		if res.CertificateID != 0 {
			return res.CertificateID, nil
		}
		return res.ID, nil
	}
}

func relinquishCertificate(r *authriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.RelinquishCertificateArgs) error {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.RelinquishCertificateArgs) error {
		return postArgsNoResult[wdk.RelinquishCertificateArgs](r, ctx, "/storage/v1/certificates/relinquish", args)
	}
}

func relinquishOutput(r *authriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.RelinquishOutputArgs) error {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.RelinquishOutputArgs) error {
		return postArgsNoResult[wdk.RelinquishOutputArgs](r, ctx, "/storage/v1/outputs/relinquish", args)
	}
}

func listCertificates(r *authriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.ListCertificatesArgs) (*wdk.ListCertificatesResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.ListCertificatesArgs) (*wdk.ListCertificatesResult, error) {
		return postArgs[wdk.ListCertificatesArgs, wdk.ListCertificatesResult](r, ctx, "/storage/v1/list/certificates", args)
	}
}

func listOutputs(r *authriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.ListOutputsArgs) (*wdk.ListOutputsResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.ListOutputsArgs) (*wdk.ListOutputsResult, error) {
		return postArgs[wdk.ListOutputsArgs, wdk.ListOutputsResult](r, ctx, "/storage/v1/list/outputs", args)
	}
}

func listActions(r *authriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.ListActionsArgs) (*wdk.ListActionsResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.ListActionsArgs) (*wdk.ListActionsResult, error) {
		return postArgs[wdk.ListActionsArgs, wdk.ListActionsResult](r, ctx, "/storage/v1/list/actions", args)
	}
}

func listTransactions(r *authriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.ListTransactionsArgs) (*wdk.ListTransactionsResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.ListTransactionsArgs) (*wdk.ListTransactionsResult, error) {
		return postArgs[wdk.ListTransactionsArgs, wdk.ListTransactionsResult](r, ctx, "/storage/v1/list/transactions", args)
	}
}

func getSyncChunk(r *authriteRequester) func(ctx context.Context, args wdk.RequestSyncChunkArgs) (*wdk.SyncChunk, error) {
	return func(ctx context.Context, args wdk.RequestSyncChunkArgs) (*wdk.SyncChunk, error) {
		return postArgs[wdk.RequestSyncChunkArgs, wdk.SyncChunk](r, ctx, "/storage/v1/sync/chunk", args)
	}
}

func findOrInsertSyncStateAuth(r *authriteRequester) func(ctx context.Context, auth wdk.AuthID, storageIdentityKey, storageName string) (*wdk.FindOrInsertSyncStateAuthResponse, error) {
	return func(ctx context.Context, auth wdk.AuthID, storageIdentityKey, storageName string) (*wdk.FindOrInsertSyncStateAuthResponse, error) {
		var res wdk.FindOrInsertSyncStateAuthResponse
		payload := map[string]string{"storageIdentityKey": storageIdentityKey, "storageName": storageName}
		if err := r.post(ctx, "/storage/v1/sync/state", payload, &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
}

func processSyncChunk() func(ctx context.Context, args wdk.RequestSyncChunkArgs, chunk *wdk.SyncChunk) (*wdk.ProcessSyncChunkResult, error) {
	return func(ctx context.Context, args wdk.RequestSyncChunkArgs, chunk *wdk.SyncChunk) (*wdk.ProcessSyncChunkResult, error) {
		return nil, fmt.Errorf("ProcessSyncChunk not fully implemented in V1 client")
	}
}

func abortAction(r *authriteRequester) func(ctx context.Context, auth wdk.AuthID, args wdk.AbortActionArgs) (*wdk.AbortActionResult, error) {
	return func(ctx context.Context, auth wdk.AuthID, args wdk.AbortActionArgs) (*wdk.AbortActionResult, error) {
		return postArgs[wdk.AbortActionArgs, wdk.AbortActionResult](r, ctx, "/storage/v1/actions/abort", args)
	}
}

func findOutputBasketsAuth() func(ctx context.Context, auth wdk.AuthID, filters wdk.FindOutputBasketsArgs) (wdk.TableOutputBaskets, error) {
	return func(ctx context.Context, auth wdk.AuthID, filters wdk.FindOutputBasketsArgs) (wdk.TableOutputBaskets, error) {
		return wdk.TableOutputBaskets{}, nil
	}
}

func findOutputsAuth() func(ctx context.Context, auth wdk.AuthID, filters wdk.FindOutputsArgs) (wdk.TableOutputs, error) {
	return func(ctx context.Context, auth wdk.AuthID, filters wdk.FindOutputsArgs) (wdk.TableOutputs, error) {
		return wdk.TableOutputs{}, nil
	}
}

func newAuthriteRequester(addr string, wallet sdk.Interface, options clientOptions) *authriteRequester {
	var opts []func(*clients.AuthFetchOptions)
	if options.httpClient != nil {
		opts = append(opts, clients.WithHttpClient(options.httpClient))
	}
	opts = append(opts, clients.WithLogger(options.logger))

	authFetch := clients.New(wallet, opts...)

	return &authriteRequester{
		addr:       addr,
		log:        options.logger,
		httpClient: authFetch,
	}
}

type authriteRequester struct {
	log        *slog.Logger
	httpClient *clients.AuthFetch
	addr       string
}

func (r *authriteRequester) DoHTTPRequest(ctx context.Context, body []byte) (io.ReadCloser, error) {
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

func (r *authriteRequester) post(ctx context.Context, path string, payload, result any) error {
	var bodyBytes []byte
	var err error
	if payload != nil {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
	}

	respBody, err := r.doHTTPPost(ctx, path, bodyBytes)
	if err != nil {
		return err
	}
	defer func() { _ = respBody.Close() }()

	data, err := io.ReadAll(respBody)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
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
			return fmt.Errorf("failed to unmarshal response (raw: %s): %w", string(data), err)
		}
	}
	return nil
}

func (r *authriteRequester) get(ctx context.Context, path string, result any) error {
	respBody, err := r.doHTTPGet(ctx, path)
	if err != nil {
		return err
	}
	defer func() { _ = respBody.Close() }()

	data, err := io.ReadAll(respBody)
	if err != nil {
		return fmt.Errorf("failed to read GET response: %w", err)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &errResp) == nil && errResp.Error != "" {
		return fmt.Errorf("storage v1 error: %s", errResp.Error)
	}

	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("failed to unmarshal GET response: %w", err)
		}
	}
	return nil
}

func (r *authriteRequester) doHTTPPost(ctx context.Context, path string, body []byte) (io.ReadCloser, error) {
	target := buildTarget(r.addr, path)
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
			ctx, "POST request to storage server failed",
			slogx.Error(err),
		)
		return nil, fmt.Errorf("storage client POST request failed: %w", err)
	}
	log.DebugContext(
		ctx, "Successfully sent POST request to storage server",
		slog.Any("resp", (*loggableResponse)(resp)),
	)

	return resp.Body, nil
}

func (r *authriteRequester) doHTTPGet(ctx context.Context, path string) (io.ReadCloser, error) {
	target := buildTarget(r.addr, path)
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
			ctx, "GET request to storage server failed",
			slogx.Error(err),
		)
		return nil, fmt.Errorf("storage client GET request failed: %w", err)
	}
	log.DebugContext(
		ctx, "Successfully sent GET request to storage server",
		slog.Any("resp", (*loggableResponse)(resp)),
	)

	return resp.Body, nil
}

func buildTarget(base, path string) string {
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
