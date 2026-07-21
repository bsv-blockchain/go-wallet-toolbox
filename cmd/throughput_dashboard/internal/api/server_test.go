package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/api"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/funding"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/metrics"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/stream"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const operatorPrivHex = "0000000000000000000000000000000000000000000000000000000000000001"

// fakeActionCreator satisfies stream.ActionCreator.
type fakeActionCreator struct {
	calls atomic.Uint64
}

func (f *fakeActionCreator) CreateAction(ctx context.Context, _ sdk.CreateActionArgs, _ string) (*sdk.CreateActionResult, error) {
	f.calls.Add(1)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return &sdk.CreateActionResult{}, nil
}

// fakeWalletAPI satisfies metrics.WalletAPI.
type fakeWalletAPI struct {
	balance      uint64
	fuelTotal    uint32
	reserveTotal uint32
}

func (f *fakeWalletAPI) Balance(context.Context) (uint64, error) {
	return f.balance, nil
}

func (f *fakeWalletAPI) ListOutputs(_ context.Context, args sdk.ListOutputsArgs, _ string) (*sdk.ListOutputsResult, error) {
	var total uint32
	switch args.Basket {
	case wdk.BasketNameForFuel:
		total = f.fuelTotal
	case wdk.BasketNameForReserve:
		total = f.reserveTotal
	}
	return &sdk.ListOutputsResult{TotalOutputs: total}, nil
}

// fakeInternalizer satisfies funding.ActionInternalizer.
type fakeInternalizer struct {
	mu   sync.Mutex
	err  error
	args []sdk.InternalizeActionArgs
}

func (f *fakeInternalizer) InternalizeAction(
	_ context.Context,
	args sdk.InternalizeActionArgs,
	_ string,
) (*sdk.InternalizeActionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.args = append(f.args, args)
	if f.err != nil {
		return nil, f.err
	}
	return &sdk.InternalizeActionResult{}, nil
}

func (f *fakeInternalizer) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.args)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testPriv(t *testing.T) *ec.PrivateKey {
	t.Helper()
	priv, err := ec.PrivateKeyFromHex(operatorPrivHex)
	require.NoError(t, err)
	return priv
}

type testEnv struct {
	handler http.Handler
	ctrl    *stream.Controller
	sampler *metrics.Sampler
	wallet  *fakeInternalizer
	cancel  context.CancelFunc
}

func newTestEnv(t *testing.T, opts ...func(*api.Deps)) *testEnv {
	t.Helper()

	parent, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	actions := &fakeActionCreator{}
	ctrl := stream.NewController(actions, stream.Options{
		TPS:        10,
		Workers:    2,
		Originator: "test-origin",
	}, discardLogger())
	t.Cleanup(ctrl.Stop)

	walletAPI := &fakeWalletAPI{balance: 100_000, fuelTotal: 50, reserveTotal: 10}
	sampler := metrics.NewSampler(
		walletAPI,
		ctrl,
		"test-origin",
		time.Hour, // long interval; tests that need ticks call Run explicitly
		1000,      // target TPS
		20,        // denomination
		1000,      // target pool
		60,        // low water %
		100,       // high water %
		discardLogger(),
	)

	internalizer := &fakeInternalizer{}
	deps := api.Deps{
		Ctrl:       ctrl,
		Sampler:    sampler,
		Wallet:     internalizer,
		Priv:       testPriv(t),
		Network:    defs.NetworkMainnet,
		Originator: "test-origin",
		ServerURL:  "http://127.0.0.1:8101",
		Logger:     discardLogger(),
		ParentCtx:  parent,
		WebFS: fstest.MapFS{
			"index.html": &fstest.MapFile{Data: []byte("<html>dashboard</html>")},
		},
	}
	for _, o := range opts {
		o(&deps)
	}

	srv := api.New(deps)
	return &testEnv{
		handler: srv.Handler(),
		ctrl:    ctrl,
		sampler: sampler,
		wallet:  internalizer,
		cancel:  cancel,
	}
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), "body=%s", rec.Body.String())
	return out
}

func TestStatus_JSONShape(t *testing.T) {
	env := newTestEnv(t)

	rec := doJSON(t, env.handler, http.MethodGet, "/api/status", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))

	body := decodeMap(t, rec)
	assert.Equal(t, "main", body["network"])
	assert.Equal(t, "http://127.0.0.1:8101", body["server_url"])
	assert.Equal(t, "test-origin", body["originator"])
	assert.Equal(t, true, body["mainnet"])
	require.Contains(t, body, "tick")
	require.Contains(t, body, "events")
	_, ok := body["tick"].(map[string]any)
	require.True(t, ok, "tick must be a JSON object")
}

func TestStatus_IncludesTickAfterSample(t *testing.T) {
	// Drive sampler via Subscribe + sample by starting Run briefly.
	// metrics.Sampler.Run is the public sample loop; use a short interval.
	parent, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	actions := &fakeActionCreator{}
	ctrl := stream.NewController(actions, stream.Options{TPS: 1, Workers: 1, Originator: "o"}, discardLogger())
	t.Cleanup(ctrl.Stop)

	walletAPI := &fakeWalletAPI{balance: 42, fuelTotal: 7, reserveTotal: 3}
	sampler := metrics.NewSampler(walletAPI, ctrl, "o", 50*time.Millisecond, 10, 20, 100, 60, 100, discardLogger())
	go sampler.Run(parent)

	// Wait until LastTick is populated.
	require.Eventually(t, func() bool {
		return sampler.LastTick().Timestamp != ""
	}, 2*time.Second, 20*time.Millisecond)

	srv := api.New(api.Deps{
		Ctrl:       ctrl,
		Sampler:    sampler,
		Wallet:     &fakeInternalizer{},
		Priv:       testPriv(t),
		Network:    defs.NetworkMainnet,
		Originator: "o",
		ServerURL:  "http://example",
		Logger:     discardLogger(),
		ParentCtx:  parent,
	})

	rec := doJSON(t, srv.Handler(), http.MethodGet, "/api/status", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	body := decodeMap(t, rec)
	tick := body["tick"].(map[string]any)
	assert.Equal(t, float64(42), tick["default_sats"])
	assert.Equal(t, float64(7), tick["fuel_count"])
	assert.Equal(t, float64(3), tick["reserve_count"])
	streamStats := tick["stream"].(map[string]any)
	assert.Equal(t, false, streamStats["running"])
}

func TestStreamStartStop_StatusRunning(t *testing.T) {
	env := newTestEnv(t)

	// Start with overrides.
	rec := doJSON(t, env.handler, http.MethodPost, "/api/stream/start", map[string]any{
		"tps":     25,
		"workers": 3,
	})
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	startBody := decodeMap(t, rec)
	assert.Equal(t, true, startBody["ok"])
	stats := startBody["stats"].(map[string]any)
	assert.Equal(t, true, stats["running"])
	assert.Equal(t, float64(25), stats["tps"])
	assert.Equal(t, float64(3), stats["workers"])
	require.True(t, env.ctrl.Running())

	// Double start → 409 Conflict.
	rec2 := doJSON(t, env.handler, http.MethodPost, "/api/stream/start", map[string]any{"tps": 1})
	require.Equal(t, http.StatusConflict, rec2.Code)
	errBody := decodeMap(t, rec2)
	assert.Contains(t, errBody["error"], "already running")

	// Stop.
	rec3 := doJSON(t, env.handler, http.MethodPost, "/api/stream/stop", nil)
	require.Equal(t, http.StatusOK, rec3.Code)
	stopBody := decodeMap(t, rec3)
	assert.Equal(t, true, stopBody["ok"])
	stopStats := stopBody["stats"].(map[string]any)
	assert.Equal(t, false, stopStats["running"])
	require.False(t, env.ctrl.Running())

	// Stop when already stopped is still OK.
	rec4 := doJSON(t, env.handler, http.MethodPost, "/api/stream/stop", nil)
	require.Equal(t, http.StatusOK, rec4.Code)
}

func TestStreamStart_EmptyBodyUsesDefaults(t *testing.T) {
	env := newTestEnv(t)

	rec := doJSON(t, env.handler, http.MethodPost, "/api/stream/start", nil)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	body := decodeMap(t, rec)
	stats := body["stats"].(map[string]any)
	assert.Equal(t, true, stats["running"])
	// Defaults from NewController in newTestEnv.
	assert.Equal(t, float64(10), stats["tps"])
	assert.Equal(t, float64(2), stats["workers"])

	env.ctrl.Stop()
}

func TestFunding_ReturnsDepositAddress(t *testing.T) {
	env := newTestEnv(t)

	rec := doJSON(t, env.handler, http.MethodGet, "/api/funding", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	body := decodeMap(t, rec)
	assert.Equal(t, "main", body["network"])
	assert.NotEmpty(t, body["address"])
	assert.NotEmpty(t, body["locking_script_hex"])
	assert.Equal(t, funding.DerivationPrefixB64, body["derivation_prefix_b64"])
	assert.Equal(t, funding.DerivationSuffixB64, body["derivation_suffix_b64"])
	assert.Equal(t, float64(100_000), body["suggested_satoshis"])
}

func TestInternalize_AtomicPath(t *testing.T) {
	env := newTestEnv(t)

	// Unparseable atomic hex skips address validation and still reaches wallet.
	rec := doJSON(t, env.handler, http.MethodPost, "/api/funding/internalize", map[string]any{
		"atomic_tx_hex": "deadbeef",
		"output_index":  0,
	})
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	body := decodeMap(t, rec)
	assert.Equal(t, true, body["ok"])
	assert.Equal(t, 1, env.wallet.callCount())
}

func TestInternalize_InvalidJSON(t *testing.T) {
	env := newTestEnv(t)

	req := httptest.NewRequest(http.MethodPost, "/api/funding/internalize", bytes.NewBufferString("{not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeMap(t, rec)
	assert.Contains(t, body["error"], "invalid json")
}

func TestInternalize_MissingFields(t *testing.T) {
	env := newTestEnv(t)

	rec := doJSON(t, env.handler, http.MethodPost, "/api/funding/internalize", map[string]any{})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeMap(t, rec)
	assert.Contains(t, body["error"], "atomic_tx_hex or txid")
	assert.Equal(t, 0, env.wallet.callCount())
}

func TestCORS_Options(t *testing.T) {
	env := newTestEnv(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/status", nil)
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "POST")
}

func TestStatic_Index(t *testing.T) {
	env := newTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "dashboard")
}

func TestEvents_SSEHeadersAndInitialTick(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	actions := &fakeActionCreator{}
	ctrl := stream.NewController(actions, stream.Options{TPS: 1, Workers: 1, Originator: "o"}, discardLogger())
	t.Cleanup(ctrl.Stop)

	walletAPI := &fakeWalletAPI{fuelTotal: 1}
	sampler := metrics.NewSampler(walletAPI, ctrl, "o", 50*time.Millisecond, 10, 20, 100, 60, 100, discardLogger())
	go sampler.Run(parent)
	require.Eventually(t, func() bool {
		return sampler.LastTick().Timestamp != ""
	}, 2*time.Second, 20*time.Millisecond)

	srv := api.New(api.Deps{
		Ctrl:       ctrl,
		Sampler:    sampler,
		Wallet:     &fakeInternalizer{},
		Priv:       testPriv(t),
		Network:    defs.NetworkMainnet,
		Originator: "o",
		ServerURL:  "http://example",
		Logger:     discardLogger(),
		ParentCtx:  parent,
	})

	ctx, cancelReq := context.WithCancel(context.Background())
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/events", nil)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.Handler().ServeHTTP(rec, req)
	}()

	// Wait for first SSE payload then cancel client.
	require.Eventually(t, func() bool {
		return rec.Body.Len() > 0
	}, 2*time.Second, 20*time.Millisecond)
	cancelReq()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not exit after client cancel")
	}

	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), "event: tick")
	assert.Contains(t, rec.Body.String(), "data:")
}
