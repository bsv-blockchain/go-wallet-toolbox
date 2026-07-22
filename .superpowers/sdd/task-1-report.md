# Task 1 Report — Stream package harden

## Status
**DONE**

## Commit
- `f2e740424f1b3d642f4421e7adf7e051a6f442cf` — `feat(dashboard): harden stream controller and hashed opreturn`

## Scope
`cmd/throughput_dashboard/internal/stream/**` only.

## What changed

### `opreturn.go`
- `HashPayload(iteration, ts)` documents/implements `sha256(FormatUint(iteration,10) + ts.UTC().Format(RFC3339Nano))` (always 32 bytes).
- `OpReturnLockingScriptForHash` rejects empty **and** non-32-byte payloads.
- `OpReturnLockingScriptForIteration` unchanged composition: hash then OP_RETURN push.

### `controller.go`
- Fixed misleading Start comment (not idempotent; errors if already running).
- Snapshot TPS/workers/originator at Start and pass into `run`/`runOne` (no mid-run field races).
- **Stop/Start generation race fix**: Stop only clears controller state if `c.done` still matches the generation it cancelled; `run` similarly only marks stopped for its own `done`. Prevents a concurrent Start from being torn down by a late Stop.
- CreateAction args remain: single 0-sat OP_RETURN output, `AcceptDelayedBroadcast: true`.
- In-flight cancel still increments `Failed` (documented; tests do not require Succeeded==Attempted after Stop).

### Tests
- Hash: determinism, UTC normalization, iteration/time uniqueness.
- OP_RETURN: contains hash + OP_RETURN; rejects empty/wrong length; iteration helper.
- Controller: start/stop, double-start error, failure counting, CreateAction arg shape (ADB/satoshis/single OP_RETURN/unique scripts/originator), restart after stop, option overrides, defaults, SnapshotAndDelta, parent-context cancel.

## Test summary
`go test ./cmd/throughput_dashboard/internal/stream/... -count=1` — **PASS** (also `-race` PASS).

## Requirements checklist
1. HashPayload formula — yes
2. OpReturn locking scripts push hash — yes
3. Controller Start/Stop/Running/Stats + rate limit + worker pool + unique iteration/hash — yes
4. CreateAction: ADB true, 0 sats, single OP_RETURN — yes (asserted in tests)
5. Tests cover hash, OP_RETURN, start/stop, failures; no Succeeded==Attempted after Stop — yes
6. Package tests pass — yes

## Concerns
None.

## Self-review
- No secrets.
- testify/require used.
- No edits outside stream package (report path is SDD artifact).
- Race on Stop clearing a newer Start generation fixed and covered indirectly via restart + parent-cancel tests.
