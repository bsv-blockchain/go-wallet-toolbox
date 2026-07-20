# Arcade-First Broadcast Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Arcade the sole default broadcast path (binary EF, identity-derived `X-CallbackToken`, SSE status stream), with circuit-breaker failover to TAAL ARC → GorillaPool ARC → WhatsOnChain → Bitails.

**Architecture:** New `pkg/services/internal/arcade` client (broadcast + query + SSE). A broadcast router in `pkg/services` replaces broadcast-to-all: Arcade primary, sequential failover only when a circuit breaker opens. Monitor gains a long-lived SSE consumer goroutine that feeds status updates into existing storage write paths, persisting `Last-Event-ID` in the `key_value` table.

**Tech Stack:** Go 1.24+, resty v2 (HTTP), gocron (existing monitor), GORM (existing storage), `httptest` for fakes. Spec: `docs/superpowers/specs/2026-06-10-arcade-broadcast-design.md`.

**Verified live API** (https://arcade-v2-us-1.bsvblockchain.tech): `POST /tx` binary EF octet-stream → 202; `GET /tx/:txid`; `GET /events?callbackToken=`; `GET /health`; NOT `/v1/*` ARC paths. Statuses: `RECEIVED, SEEN_ON_NETWORK, SEEN_ON_MULTIPLE_NODES, MINED, IMMUTABLE, REJECTED`.

---

## Execution waves (for parallel subagent dispatch)

- **Wave 1 (independent, parallel):** Task 1 (defs config), Task 2 (arcade client core), Task 3 (SSE client — same package as Task 2 but separate files; run after Task 2 merges OR same agent), Task 4 (circuit breaker)
- **Wave 2 (after wave 1):** Task 5 (broadcast router + services wiring), Task 6 (storage: external status updates + KV)
- **Wave 3 (after wave 2):** Task 7 (monitor SSE consumer), Task 8 (infra wiring + token derivation)
- **Wave 4:** Task 9 (full verification, existing-test updates, lint)

Each task: TDD (failing test → minimal impl → pass → commit). Run `go build ./... && go test ./<changed packages>/...` before each commit. Repo lint: `golangci-lint run <dirs>` if available.

---

### Task 1: Arcade + GorillaPool config in `pkg/defs`

**Files:**
- Create: `pkg/defs/arcade.go`
- Create: `pkg/defs/arcade_test.go`
- Modify: `pkg/defs/services.go`

- [ ] **Step 1.1: Write failing test** `pkg/defs/arcade_test.go`:

```go
package defs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func TestDefaultServicesConfigArcade(t *testing.T) {
	t.Run("mainnet defaults", func(t *testing.T) {
		cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
		assert.True(t, cfg.Arcade.Enabled)
		assert.Equal(t, "https://arcade-v2-us-1.bsvblockchain.tech", cfg.Arcade.URL)
		assert.Equal(t, cfg.Arcade.URL, cfg.Arcade.EventsURL)
		assert.True(t, cfg.Arcade.FullStatusUpdates)
		assert.Equal(t, uint(3), cfg.Arcade.CircuitBreaker.FailureThreshold)
		assert.Equal(t, uint(30), cfg.Arcade.CircuitBreaker.HealthProbeIntervalSeconds)
		// GorillaPool ARC failover present on mainnet
		assert.True(t, cfg.ArcGorillaPoolConfig.Enabled)
		assert.Equal(t, "https://arc.gorillapool.io", cfg.ArcGorillaPoolConfig.URL)
	})

	t.Run("testnet defaults: arcade and gorillapool disabled", func(t *testing.T) {
		cfg := defs.DefaultServicesConfig(defs.NetworkTestnet)
		assert.False(t, cfg.Arcade.Enabled)
		assert.False(t, cfg.ArcGorillaPoolConfig.Enabled)
	})
}

func TestArcadeValidate(t *testing.T) {
	t.Run("disabled passes", func(t *testing.T) {
		a := defs.Arcade{Enabled: false}
		require.NoError(t, a.Validate())
	})
	t.Run("enabled requires url", func(t *testing.T) {
		a := defs.Arcade{Enabled: true}
		require.Error(t, a.Validate())
	})
	t.Run("enabled with url passes and defaults events url", func(t *testing.T) {
		a := defs.Arcade{Enabled: true, URL: "https://arcade.example.com"}
		require.NoError(t, a.Validate())
		assert.Equal(t, "https://arcade.example.com", a.EventsURL)
	})
}
```

- [ ] **Step 1.2: Run** `go test ./pkg/defs/ -run 'TestDefaultServicesConfigArcade|TestArcadeValidate' -v` — expect FAIL (types undefined).

- [ ] **Step 1.3: Implement** `pkg/defs/arcade.go`:

```go
package defs

import "fmt"

// ArcadeServiceName is the service name used in queues/results for the Arcade broadcaster.
const ArcadeServiceName = "Arcade"

// ArcGorillaPoolServiceName is the service name for the GorillaPool ARC failover broadcaster.
const ArcGorillaPoolServiceName = "ARC-GorillaPool"

// ArcadeURL is the default mainnet Arcade instance.
const ArcadeURL = "https://arcade-v2-us-1.bsvblockchain.tech"

// GorillaPoolArcURL is the default mainnet GorillaPool ARC instance (failover only).
const GorillaPoolArcURL = "https://arc.gorillapool.io"

// ArcadeCircuitBreaker configures when the wallet fails over away from Arcade.
type ArcadeCircuitBreaker struct {
	// FailureThreshold is the number of consecutive transport failures that opens the circuit.
	FailureThreshold uint `mapstructure:"failure_threshold"`
	// HealthProbeIntervalSeconds is how often /health is probed while the circuit is open.
	HealthProbeIntervalSeconds uint `mapstructure:"health_probe_interval_seconds"`
}

// Arcade is the configuration for the Arcade broadcaster (primary broadcast path).
type Arcade struct {
	Enabled bool `mapstructure:"enabled"`
	// URL is the base URL of the Arcade instance.
	URL string `mapstructure:"url"`
	// EventsURL is the base URL for the SSE /events endpoint; defaults to URL.
	EventsURL string `mapstructure:"events_url"`
	// CallbackToken scopes webhooks and the SSE stream to this wallet instance.
	// When empty it is derived from the wallet identity key at wiring time.
	CallbackToken string `mapstructure:"callback_token"`
	// CallbackURL is an optional public webhook endpoint (X-CallbackUrl).
	CallbackURL string `mapstructure:"callback_url"`
	// FullStatusUpdates requests every status transition (X-FullStatusUpdates).
	FullStatusUpdates bool `mapstructure:"full_status_updates"`

	CircuitBreaker ArcadeCircuitBreaker `mapstructure:"circuit_breaker"`
}

// Validate checks the Arcade configuration and defaults EventsURL to URL.
func (a *Arcade) Validate() error {
	if !a.Enabled {
		return nil
	}
	if a.URL == "" {
		return fmt.Errorf("arcade is enabled but url is empty")
	}
	if a.EventsURL == "" {
		a.EventsURL = a.URL
	}
	return nil
}
```

- [ ] **Step 1.4: Modify** `pkg/defs/services.go`:
  - Add to `WalletServices` struct (after `ArcConfig ARC` field):
    ```go
    Arcade               Arcade `mapstructure:"arcade"`
    ArcGorillaPoolConfig ARC    `mapstructure:"arc_gorillapool"`
    ```
  - In `Validate()` add after the ArcConfig check:
    ```go
    if err = ws.Arcade.Validate(); err != nil {
        return fmt.Errorf("invalid Arcade config: %w", err)
    }
    if err = ws.ArcGorillaPoolConfig.Validate(); err != nil {
        return fmt.Errorf("invalid GorillaPool ARC config: %w", err)
    }
    ```
  - In `DefaultServicesConfig` add to the `cfg` literal:
    ```go
    Arcade: Arcade{
        Enabled:           chain == NetworkMainnet,
        URL:               ArcadeURL,
        EventsURL:         ArcadeURL,
        FullStatusUpdates: true,
        CircuitBreaker: ArcadeCircuitBreaker{
            FailureThreshold:           3,
            HealthProbeIntervalSeconds: 30,
        },
    },
    ArcGorillaPoolConfig: ARC{
        Enabled: chain == NetworkMainnet,
        URL:     GorillaPoolArcURL,
    },
    ```
  Keep existing ARC/WoC defaults unchanged (they become failover/verification roles; the router in Task 5 controls who broadcasts).

- [ ] **Step 1.5: Run** `go test ./pkg/defs/ -v` — expect PASS (all tests, not just new ones).

- [ ] **Step 1.6: Commit** `feat(defs): add Arcade and GorillaPool ARC configuration`

---

### Task 2: Arcade client — broadcast + query + status mapping

**Files:**
- Create: `pkg/services/internal/arcade/arcade_service.go`
- Create: `pkg/services/internal/arcade/tx_info.go`
- Create: `pkg/services/internal/arcade/arcade_broadcast.go`
- Create: `pkg/services/internal/arcade/arcade_query_transaction.go`
- Create: `pkg/services/internal/arcade/arcade_service_test.go`

Mimic the structure/conventions of `pkg/services/internal/arc/` (resty client, `logging.Child`, tracing spans, `withBroadcastNote` history-notes pattern, `wdk.PostedTxID` results). Read `arc_service.go`, `arc_broadcast.go`, `tx_info.go`, `tx_status.go` first.

**Pinned public API (other tasks depend on these exact signatures):**

```go
package arcade

const ServiceName = defs.ArcadeServiceName

// ErrBackpressure is returned when Arcade responds 503; RetryAfter carries the server hint.
type BackpressureError struct{ RetryAfter time.Duration }
func (e *BackpressureError) Error() string

type Service struct { /* logger, httpClient, config, urls, broadcastHeaders */ }

func New(logger *slog.Logger, httpClient *resty.Client, config defs.Arcade) *Service

// PostEF broadcasts a single tx. efHex is decoded to binary and POSTed to /tx
// as application/octet-stream. Transport errors return err != nil (circuit breaker
// counts these). Arcade-level rejections (400) return a result with Error set and err == nil.
// 503 returns (*BackpressureError) as err — callers must NOT count it as failure.
func (s *Service) PostEF(ctx context.Context, efHex, txID string) (*wdk.PostedTxID, error)

// QueryTx returns the lifecycle state from GET /tx/{txID}. nil, wdk.ErrNotFoundError on 404.
func (s *Service) QueryTx(ctx context.Context, txID string) (*TXInfo, error)

// Healthy probes GET /health.
func (s *Service) Healthy(ctx context.Context) bool
```

```go
// tx_info.go
type TxStatus string

const (
	StatusReceived            TxStatus = "RECEIVED"
	StatusSeenOnNetwork       TxStatus = "SEEN_ON_NETWORK"
	StatusSeenOnMultipleNodes TxStatus = "SEEN_ON_MULTIPLE_NODES"
	StatusMined               TxStatus = "MINED"
	StatusImmutable           TxStatus = "IMMUTABLE"
	StatusRejected            TxStatus = "REJECTED"
)

type TXInfo struct {
	TxID         string   `json:"txid"`
	TxStatus     TxStatus `json:"txStatus"`
	Timestamp    string   `json:"timestamp"`
	BlockHash    string   `json:"blockHash"`
	BlockHeight  uint32   `json:"blockHeight"`
	MerklePath   string   `json:"merklePath"`
	ExtraInfo    string   `json:"extraInfo"`
	CompetingTxs []string `json:"competingTxs"`
}
```

**Behavioral requirements:**
- Headers on `POST /tx`: `Content-Type: application/octet-stream`, `X-CallbackToken` always (from config), `X-FullStatusUpdates: true` when `config.FullStatusUpdates`, `X-CallbackUrl` only when `config.CallbackURL != ""`. `User-Agent: go-wallet-toolbox` (use `httpx.NewHeaders()` like arc does — but NOT `ContentTypeJSON`).
- `efHex` decode failure → result with `PostedTxIDResultError`, err nil (bad input, not transport).
- 202/200 → map `TXInfo` to `wdk.PostedTxID` like `arc.toResultForPostTxID` does: `REJECTED` + competingTxs → `DoubleSpend: true`; parse `MerklePath` hex via `transaction.NewMerklePathFromHex`; marshal full info into `.Data`; attach history notes (`history.NewBuilder().PostBeefSuccess/PostBeefError`).
- 400 → result with `PostedTxIDResultError` and the server error message, err nil.
- 503 → `(nil, *BackpressureError)` with `Retry-After` parsed (seconds int or HTTP date; default 5s when absent/unparseable).
- Network error / 5xx (≠503) → `(nil, err)`.

- [ ] **Step 2.1:** Write failing tests in `arcade_service_test.go` using `httptest.NewServer`. Cover: binary body received exactly (decode hex of a fixed valid EF tx hex — reuse a tx fixture from `pkg/services/internal/arc/testabilities` if importable, otherwise inline a known-good EF hex constant); header assertions (octet-stream, token, full-status); 202 success mapping; REJECTED mapping (DoubleSpend true); 400 mapping (err nil, result error); 503 → BackpressureError with RetryAfter 7s for header `Retry-After: 7`; connection-refused → err != nil; `QueryTx` 200 + 404; `Healthy` true/false.
- [ ] **Step 2.2:** Run `go test ./pkg/services/internal/arcade/ -v` — expect FAIL.
- [ ] **Step 2.3:** Implement the four source files.
- [ ] **Step 2.4:** Run `go test ./pkg/services/internal/arcade/ -v` — expect PASS.
- [ ] **Step 2.5:** Commit `feat(services): add Arcade broadcaster client with binary EF support`

---

### Task 3: Arcade SSE client

**Files:**
- Create: `pkg/services/internal/arcade/sse.go`
- Create: `pkg/services/internal/arcade/sse_test.go`

**Pinned API:**

```go
// StatusEvent is one SSE status frame from GET /events.
type StatusEvent struct {
	EventID string // SSE "id:" field — nanosecond timestamp, used as Last-Event-ID
	TXInfo         // embedded: parsed from the "data:" JSON payload
}

// StreamEvents connects to {EventsURL}/events?callbackToken=... and invokes onEvent
// sequentially for each status event. Reconnects automatically with exponential
// backoff (1s base, 2x, 60s cap), sending Last-Event-ID from the most recently
// delivered event (or the lastEventID argument before any event arrives).
// Blocks until ctx is cancelled; returns ctx.Err().
// If onEvent returns an error, it is logged and the event is treated as delivered.
func (s *Service) StreamEvents(ctx context.Context, lastEventID string, onEvent func(ev StatusEvent) error) error
```

**Implementation notes:**
- Plain `net/http` (resty is poor for streaming): `GET`, headers `Accept: text/event-stream`, `Cache-Control: no-cache`, `Last-Event-ID: <id>` when non-empty. No read timeout on the stream (use `http.Client{Timeout: 0}` with a dial/TLS-handshake-timeouted `http.Transport`); rely on ctx cancellation.
- Parse with `bufio.Scanner` over lines (raise buffer to 1 MiB): accumulate `id:`, `event:`, `data:` fields; dispatch on blank line; ignore comment lines starting with `:` (keepalives). Only dispatch `event: status` (or absent event type) frames whose data parses as `TXInfo` JSON.
- Backoff resets to base after a connection that delivered ≥1 event.

- [ ] **Step 3.1:** Failing tests with an `httptest` SSE handler (use `http.Flusher`): delivers 2 events then closes → client reconnects with `Last-Event-ID` equal to second event's id (assert on the reconnect request); keepalive comments ignored; malformed data line skipped without killing the stream; ctx cancel terminates with `context.Canceled`.
- [ ] **Step 3.2:** Run — FAIL. **Step 3.3:** Implement. **Step 3.4:** Run `go test ./pkg/services/internal/arcade/ -v -race` — PASS.
- [ ] **Step 3.5:** Commit `feat(services): add Arcade SSE event stream client`

---

### Task 4: Circuit breaker

**Files:**
- Create: `pkg/services/internal/circuitbreaker/circuitbreaker.go`
- Create: `pkg/services/internal/circuitbreaker/circuitbreaker_test.go`

**Pinned API:**

```go
package circuitbreaker

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

type Config struct {
	FailureThreshold uint          // consecutive failures to open; min 1
	ProbeInterval    time.Duration // how often probe runs while open
	Probe            func(ctx context.Context) bool // optional health probe; nil = time-based half-open only
	Clock            func() time.Time               // injectable for tests; nil = time.Now
}

type CircuitBreaker struct { /* mutex, state, consecutiveFailures, openedAt, cfg, logger */ }

func New(logger *slog.Logger, cfg Config) *CircuitBreaker

// Allow reports whether a call may proceed: true when closed, or when open/half-open
// and a trial is due (ProbeInterval elapsed since openedAt or last trial).
func (cb *CircuitBreaker) Allow() bool
func (cb *CircuitBreaker) RecordSuccess() // resets failures; closes circuit
func (cb *CircuitBreaker) RecordFailure() // increments; opens at threshold; re-opens half-open
func (cb *CircuitBreaker) State() State

// StartHealthProbe runs Probe every ProbeInterval while the circuit is open and
// closes it on a successful probe. No-op when Probe is nil. Returns when ctx done.
func (cb *CircuitBreaker) StartHealthProbe(ctx context.Context)
```

Pure synchronous core (mutex-guarded, injectable clock) so tests are deterministic — no goroutines except `StartHealthProbe`.

- [ ] **Step 4.1:** Failing table tests: stays closed below threshold; opens at threshold; `Allow()` false while open before interval, true (one trial) after interval; success in half-open closes; failure in half-open re-opens and resets interval window; `StartHealthProbe` with stubbed probe closes circuit (use short interval + `require.Eventually`).
- [ ] **Step 4.2:** Run — FAIL. **Step 4.3:** Implement. **Step 4.4:** `go test ./pkg/services/internal/circuitbreaker/ -v -race` — PASS.
- [ ] **Step 4.5:** Commit `feat(services): add circuit breaker for broadcast failover`

---

### Task 5: Broadcast router + services assembly

**Files:**
- Create: `pkg/services/broadcast_router.go`
- Create: `pkg/services/broadcast_router_test.go`
- Modify: `pkg/services/services.go`

**5a. Router** — `pkg/services/broadcast_router.go`:

```go
package services

// broadcastTarget is a single named broadcaster the router can use.
type broadcastTarget struct {
	name string
	post func(ctx context.Context, efHex string, rawTx []byte, txID string) (*wdk.PostedTxID, error)
}

type broadcastRouter struct {
	logger    *slog.Logger
	primary   broadcastTarget
	breaker   *circuitbreaker.CircuitBreaker
	failovers []broadcastTarget
	// maxBackpressureWait bounds honored Retry-After delays (default 30s)
	maxBackpressureWait time.Duration
	sleep               func(ctx context.Context, d time.Duration) // injectable for tests
}

// broadcast routes one tx. Returns results for every service actually attempted
// (happy path: exactly one — Arcade).
func (r *broadcastRouter) broadcast(ctx context.Context, efHex string, rawTx []byte, txID string) []*wdk.PostFromBEEFServiceResult
```

**Routing rules (encode as table tests):**
1. `breaker.Allow()` true → call primary.
   - err == nil → `RecordSuccess()`, return that single result (even when the result itself carries a validation `Error` — a tx rejection is NOT a service failure and must NOT fail over).
   - `*arcade.BackpressureError` → wait `min(RetryAfter, maxBackpressureWait)` via `r.sleep`, retry primary once; second backpressure → treat as transport failure for this call (RecordFailure) and fall through to rule 3. Backpressure itself never increments the breaker except via this fall-through.
   - other err → `RecordFailure()`, fall through to rule 3.
2. `breaker.Allow()` false → rule 3 directly (no primary attempt).
3. Failover: iterate `failovers` **sequentially**; first target whose err == nil wins — append its result and stop. err != nil → append error result, continue. All failed → return all error results.

**5b. Assembly** — in `services.New` (`pkg/services/services.go`):

- After the `config.ArcConfig.Enabled` block, add GorillaPool ARC (reuses the arc package — classic ARC dialect):
  ```go
  var arcGorillaPoolService *arc.Service
  if config.ArcGorillaPoolConfig.Enabled {
      arcGorillaPoolService = arc.NewNamed(defs.ArcGorillaPoolServiceName, logger, options.RestyClientFactory.New(), config.ArcGorillaPoolConfig)
  }
  ```
  Add to `pkg/services/internal/arc/arc_service.go` a `NewNamed(name string, ...)` constructor (existing `New` delegates with `ServiceName`); store the name on `Service` and use it in `withBroadcastNote`/results instead of the package constant. Keep `New` signature unchanged.
- Keep TAAL `arcService` registration for `MerklePath` as-is, capture the handle:
  ```go
  var arcService *arc.Service
  if config.ArcConfig.Enabled { arcService = arc.New(...); predefined = append(... /* MerklePath only — remove PostEF from Implementation */) }
  ```
- **Remove broadcast methods from the parallel queues**: drop `PostEF` from the ARC `Implementation` and `PostTX` from WhatsOnChain and Bitails `Implementation` literals. `postEFServices`/`postTXServices` queues remain (custom implementations via `options.customImplementations` still flow there) but contain no predefined broadcasters.
- Build the router when Arcade is enabled:
  ```go
  var router *broadcastRouter
  if config.Arcade.Enabled {
      arcadeService := arcade.New(logger, options.RestyClientFactory.New(), config.Arcade)
      breaker := circuitbreaker.New(logger, circuitbreaker.Config{
          FailureThreshold: config.Arcade.CircuitBreaker.FailureThreshold,
          ProbeInterval:    time.Duration(config.Arcade.CircuitBreaker.HealthProbeIntervalSeconds) * time.Second,
          Probe:            arcadeService.Healthy,
      })
      router = newBroadcastRouter(logger, breaker, arcadeService, arcService, arcGorillaPoolService, wocService, bitailsService)
      walletServices.arcade = arcadeService
  }
  ```
  `newBroadcastRouter` builds targets, skipping nils: primary = arcade `PostEF` adapter; failovers in order TAAL arc (`post` wraps `PostEF(ctx, efHex, txID)`), GorillaPool arc (same), WhatsOnChain (`PostTX(ctx, rawTx)`), Bitails (`PostTX(ctx, rawTx)`). NOTE: `wocService`/`bitailsService` variables must be hoisted out of their `if config.X.Enabled` blocks.
- Store on struct: `arcade *arcade.Service`, `broadcastRouter *broadcastRouter` fields; remove the line-35 `NOTE: add p2p client here when arcade is implemented` comment.

**5c. PostFromBEEF** — in the per-txID loop replace the two `.All(...)` calls:

```go
if s.broadcastRouter != nil {
    allResults = append(allResults, s.broadcastRouter.broadcast(ctx, efHex, rawTx, txID)...)
    continue
}
// legacy path (Arcade disabled, e.g. testnet): unchanged .All() broadcast below
```

**5d. Expose stream + health for monitor/infra (used by Tasks 7–8):**

```go
// BroadcastStatusEvents streams Arcade SSE status events. Returns wdk.ErrNotFoundError
// sentinel-style error when Arcade is disabled.
func (s *WalletServices) BroadcastStatusEvents(ctx context.Context, lastEventID string, onEvent func(wdk.BroadcastStatusEvent) error) error
```

Add `wdk.BroadcastStatusEvent` in new file `pkg/wdk/broadcast_status_event.go`:

```go
package wdk

// BroadcastStatusEvent is a transaction lifecycle update pushed by the broadcaster.
type BroadcastStatusEvent struct {
	EventID      string // opaque stream cursor (Arcade: nanosecond timestamp)
	TxID         string
	Status       string // RECEIVED | SEEN_ON_NETWORK | SEEN_ON_MULTIPLE_NODES | MINED | IMMUTABLE | REJECTED
	BlockHash    string
	BlockHeight  uint32
	MerklePath   string // hex BUMP, may be empty
	ExtraInfo    string
	CompetingTxs []string
}
```

`BroadcastStatusEvents` adapts `arcade.StatusEvent` → `wdk.BroadcastStatusEvent` field-by-field.

- [ ] **Step 5.1:** Failing router table tests (fake targets as closures recording calls; fake breaker via real breaker with clock injection): happy path calls only primary; validation-error result does not fail over; transport error → failover order TAAL→GP→WoC→Bitails, stops at first success; backpressure honored then retried; open circuit skips primary.
- [ ] **Step 5.2:** Run `go test ./pkg/services/ -run TestBroadcastRouter -v` — FAIL. **Step 5.3:** Implement 5a–5d. **Step 5.4:** `go test ./pkg/services/... -race` — PASS, including existing services tests (update any that assert WoC/ARC appear in PostFromBEEF results by default — they now require either Arcade config pointing at a test server or the legacy path with Arcade disabled).
- [ ] **Step 5.5:** Commit `feat(services): route broadcasts through Arcade with circuit-breaker failover`

---

### Task 6: Storage — external status updates + KeyValue access

**Files:**
- Modify: `pkg/storage/provider.go`
- Create: `pkg/storage/internal/actions/process_external_status.go` (mirror style of `process_confirm_double_spends.go`)
- Modify: `pkg/monitor/interface.go` (MonitoredStorage)
- Test: `pkg/storage/provider_external_status_test.go`

**Pinned API (Provider + MonitoredStorage additions):**

```go
// ProcessExternalTxStatusUpdate applies a broadcaster-pushed lifecycle update.
// Returns the synchronized statuses for any tx whose stored state changed.
func (p *Provider) ProcessExternalTxStatusUpdate(ctx context.Context, ev wdk.BroadcastStatusEvent) ([]wdk.TxSynchronizedStatus, error)

// GetKeyValue / SetKeyValue expose the key_value table for small instance state.
func (p *Provider) GetKeyValue(ctx context.Context, key string) ([]byte, bool, error)
func (p *Provider) SetKeyValue(ctx context.Context, key string, value []byte) error
```

KV methods delegate to existing `repo.KeyValue` (`pkg/internal/storage/repo/key_value.go`) — check how Provider holds repos (`repo.All`?) and follow that pattern.

**ProcessExternalTxStatusUpdate semantics (reuse existing internals — do not reimplement):**
- Unknown txid (no KnownTx/ProvenTxReq row) → no-op, return nil (SSE token is instance-scoped but be defensive).
- Already-terminal stored status (completed/failed) → no-op.
- `MINED` / `IMMUTABLE`: requires merkle path — use `ev.MerklePath` when non-empty (parse `transaction.NewMerklePathFromHex`, validate root for height via the same path `synchronize_tx_statuses.go` uses), otherwise fetch via `services.MerklePath(ctx, txID)`. Then apply through the existing `UpdateKnownTxAsMined` flow (see `pkg/internal/storage/repo/known_tx.go:359` and its call site in `synchronize_tx_statuses.go:373-412`) so Transaction + UserUTXO updates stay atomic.
- `SEEN_ON_NETWORK` / `SEEN_ON_MULTIPLE_NODES` / `RECEIVED`: if stored status is `Unsent`/`Sending`, advance to the same status `broadcastTxs` sets after successful broadcast (`Unmined` — confirm exact constant in `wdk.ProvenTxReqStatus` and reuse the existing transition helper); add a TxNote.
- `REJECTED`: do **not** terminally fail directly. Build an `AggregatedPostFromBEEF`-shaped verdict (or call the existing exported entry) and run it through `confirmDoubleSpends` (`pkg/storage/internal/actions/process_confirm_double_spends.go:34`) so the network re-check (3 retries, WhatsOnChain `checkTxsKnownToNetwork`) decides between false positive (success), serviceError (retry later) and confirmed double spend. `ev.CompetingTxs` feeds the verdict.

- [ ] **Step 6.1:** Failing tests in `pkg/storage/provider_external_status_test.go` modeled on `provider_broadcast_double_spend_test.go` (same testabilities fixtures): MINED with merkle path in event → KnownTx completed + outputs mined; MINED without path → services.MerklePath fallback used; SEEN_ON_NETWORK advances Sending→Unmined; REJECTED but network knows tx → not failed; REJECTED confirmed → invalid; unknown txid → no error, no change; KV Get/Set roundtrip.
- [ ] **Step 6.2:** Run — FAIL. **Step 6.3:** Implement; add the two methods to `monitor.MonitoredStorage` interface:
  ```go
  ProcessExternalTxStatusUpdate(ctx context.Context, ev wdk.BroadcastStatusEvent) ([]wdk.TxSynchronizedStatus, error)
  GetKeyValue(ctx context.Context, key string) ([]byte, bool, error)
  SetKeyValue(ctx context.Context, key string, value []byte) error
  ```
  (Update any MonitoredStorage test fakes in pkg/monitor tests.)
- [ ] **Step 6.4:** `go test ./pkg/storage/... ./pkg/monitor/... -race` — PASS, including `provider_broadcast_double_spend_test.go` untouched and green.
- [ ] **Step 6.5:** Commit `feat(storage): apply external broadcaster status updates with double-spend verification`

---

### Task 7: Monitor SSE consumer

**Files:**
- Create: `pkg/monitor/broadcast_events.go`
- Create: `pkg/monitor/broadcast_events_test.go`
- Modify: `pkg/monitor/monitor.go`, `pkg/monitor/options.go`

**Pinned API:**

```go
// BroadcastEventStreamer is implemented by services.WalletServices.
type BroadcastEventStreamer interface {
	BroadcastStatusEvents(ctx context.Context, lastEventID string, onEvent func(wdk.BroadcastStatusEvent) error) error
}

// In options.go: a DaemonEventOption
func WithBroadcastEventStream(streamer BroadcastEventStreamer) DaemonEventOption
```

**Behavior (`broadcast_events.go`, started from `Daemon.Start` as a goroutine when the streamer option is set, alongside the existing `handleReorgEvents` pattern):**
- `LastEventIDKey = "arcade_sse_last_event_id"`.
- On start: `id, _, _ := d.storage.GetKeyValue(ctx, LastEventIDKey)`; pass `string(id)`.
- `onEvent`: call `d.storage.ProcessExternalTxStatusUpdate(ctx, ev)`; on success persist `ev.EventID` via `SetKeyValue` and forward proven results through the existing `d.sendProvenEvents(results)`; on error log and still persist the EventID (the polling tasks are the safety net — do not wedge the stream on one bad event).
- Streamer returning (ctx done) ends the goroutine; all reconnect logic lives in the arcade SSE client.

- [ ] **Step 7.1:** Failing tests with a fake streamer (emits scripted events) and fake MonitoredStorage: last-event-id loaded and passed; events forwarded to ProcessExternalTxStatusUpdate in order; EventID persisted after each; proven events go out on OnTxProven channel; processing error does not stop subsequent events.
- [ ] **Step 7.2:** Run — FAIL. **Step 7.3:** Implement. **Step 7.4:** `go test ./pkg/monitor/... -race` — PASS.
- [ ] **Step 7.5:** Commit `feat(monitor): consume Arcade SSE status stream with persisted replay cursor`

---

### Task 8: Wiring — callback token derivation + infra server

**Files:**
- Create: `pkg/wdk/arcade_token.go` + `pkg/wdk/arcade_token_test.go`
- Modify: `pkg/infra/server.go`

**Token helper:**

```go
package wdk

// DeriveArcadeCallbackToken derives a stable, instance-identifying callback token
// from the wallet identity public key (DER hex). Deterministic so the SSE stream
// scope and Last-Event-ID replay survive restarts.
func DeriveArcadeCallbackToken(identityKeyHex string) string {
	mac := hmac.New(sha256.New, []byte(identityKeyHex))
	mac.Write([]byte("go-wallet-toolbox/arcade-sse/v1"))
	return hex.EncodeToString(mac.Sum(nil))
}
```

**server.go changes:**
- Move the `storageIdentityKey, err := wdk.IdentityKey(cfg.ServerPrivateKey)` call (currently line ~78) ABOVE `services.New` (line ~76); before `services.New` add:
  ```go
  if cfg.Services.Arcade.Enabled && cfg.Services.Arcade.CallbackToken == "" {
      cfg.Services.Arcade.CallbackToken = wdk.DeriveArcadeCallbackToken(storageIdentityKey)
  }
  ```
- Where `monitorOpts` are assembled (line ~150s), add `monitor.WithBroadcastEventStream(activeServices)` when `cfg.Services.Arcade.Enabled`.

- [ ] **Step 8.1:** Failing test: token is 64 hex chars, deterministic for same key, different for different keys.
- [ ] **Step 8.2:** Run — FAIL. **Step 8.3:** Implement both files. **Step 8.4:** `go test ./pkg/wdk/ ./pkg/infra/... -race && go build ./...` — PASS. **Step 8.5:** Commit `feat(infra): wire Arcade with identity-derived callback token and SSE monitor`

---

### Task 9: Full verification & cleanup

- [ ] **Step 9.1:** `go build ./... && go vet ./...`
- [ ] **Step 9.2:** `go test ./... -race` — fix any test still assuming broadcast-to-all (notably `pkg/services` PostFromBEEF tests and storage broadcast tests using testabilities service fakes; preferred fix: their fixtures disable Arcade → legacy path exercises old behavior, plus new fixtures with a fake Arcade server exercise the router path).
- [ ] **Step 9.3:** `golangci-lint run ./...` (if installed; otherwise skip and note).
- [ ] **Step 9.4:** Re-read the spec (`docs/superpowers/specs/2026-06-10-arcade-broadcast-design.md`) section by section, confirm each requirement maps to landed code; fix gaps.
- [ ] **Step 9.5:** Commit `test: verify Arcade-first broadcast end to end`

---

## Self-review notes

- **Spec coverage:** binary EF (T2), callback token (T8), SSE + replay persistence (T3, T6, T7), circuit-breaker failover order (T4, T5), WoC/Bitails demotion with verification roles intact (T5 removes PostTX from queues; all other WoC/Bitails methods untouched), REJECTED-through-confirmDoubleSpends (T6), 503 backpressure (T2, T5), monitor safety-net unchanged (no task touches existing task intervals). ✅
- **Type consistency:** `wdk.BroadcastStatusEvent` defined once (T5d), consumed by T6/T7; `arcade.Service` API pinned in T2/T3, consumed by T5; breaker API pinned in T4, consumed by T5. `arc.NewNamed` introduced in T5 — agents touching `arc` package must keep `New` behavior identical.
- **Known judgment calls left to implementing agents:** exact `ProvenTxReqStatus` constant for the SEEN_* transition (read `pkg/wdk/tx_status.go`); how Provider exposes repos for KV (follow existing pattern); reusing vs adapting `confirmDoubleSpends` entry signature. Agents must read the named files before coding.
