# InternalizeAction broadcast for unknown txs — `sendWithResults` / `notDelayedResults`

**Issue:** [bsv-blockchain/go-wallet-toolbox#818](https://github.com/bsv-blockchain/go-wallet-toolbox/issues/818)
**Severity:** Medium-High — internalized unproven txs may sit unbroadcast until the monitor/background worker runs; RPC callers have no visibility into broadcast status (TS surfaces soft failures via `shareReqsWithWorld`).
**Prior code PR (closed, plan-only policy):** [#945](https://github.com/bsv-blockchain/go-wallet-toolbox/pull/945) — useful draft; not the accepted delivery shape. Re-implement from this plan against current `main`.

---

## Context for a fresh session

You are fixing a TypeScript parity gap on `InternalizeAction`. When storage receives a **new** (non-merge) transaction that is unknown and has **no mining proof** in the BEEF, Go today:

1. Stores the user transaction + KnownTx as `unsent` / `sending`.
2. Enqueues a **fire-and-forget** background broadcast via `BackgroundBroadcaster.Add`.
3. Returns only `{ accepted, isMerge, txid, satoshis }` — never `sendWithResults` or `notDelayedResults`.

TypeScript (`internalizeAction.ts` `newInternalize` path, ~L417–436) creates a `ProvenTxReq` with status `unsent`, calls `shareReqsWithWorld()` **before returning**, and on broadcast failure populates `sendWithResults` / `notDelayedResults` so the caller can react.

A previous implementation PR (#945) was closed because bug-track work must land as plan-only first. Prefer reusing its design (sync `process.BackgroundBroadcast` after UoW commit) rather than inventing a second broadcast stack.

---

## Problem summary (verified on current `main`)

### RPC-observable difference

**Go response shape today** (`pkg/wdk/storage_internalize_action_args.go` ~L109–114):

```go
type InternalizeActionResult struct {
	Accepted bool   `json:"accepted"`
	IsMerge  bool   `json:"isMerge"`
	TxID     string `json:"txid"`
	Satoshis int64  `json:"satoshis"`
}
```

**TypeScript / schema expectation** (`conformance/vectors/wallet/storage/adapter-conformance.json` ~L390):

```text
StorageInternalizeActionResult: {
  accepted, isMerge, txid, satoshis,
  sendWithResults?, notDelayedResults?
}
```

On broadcast failure TS looks like:

```json
{
  "accepted": true, "isMerge": false, "txid": "...", "satoshis": 0,
  "sendWithResults": [{ "txid": "...", "status": "sending" }],
  "notDelayedResults": [{ "txid": "...", "status": "serviceError" }]
}
```

### Current Go path anchors

Line numbers verified against `main` as of this plan revision (Track S already merged). Re-check after large `actions/` refactors.

| Location | What happens |
|----------|----------------|
| `pkg/storage/internal/actions/internalize.go` `Internalize` (~L76–291) | UoW store, optional `updateKnownTxAsMined`, return fixed 4-field result |
| `storeNewTx` (~L401–492) | Sets KnownTx `unsent` + user tx `sending` for unproven; if mined/`AlreadySent` → `unmined`/`unproven` and skips broadcaster |
| `storeNewTx` (~L482–489) | `backgroundBroadcaster.Add(beef, []string{txID})` — async, result discarded |
| `pkg/storage/internal/actions/actions.go` (~L73–85) | Wires `processAction.backgroundBroadcaster` into `newInternalizeAction` |
| `pkg/storage/internal/actions/process.go` `singleTxBroadcastResult` (~L850–903) | Canonical review ↔ sendWith status mapping (source of truth for the helper) |
| `pkg/storage/internal/actions/process.go` `BackgroundBroadcast` (~L952–1009) | Sync `PostFromBEEF` + `updateSingleTx` + attempt bump; returns `[]ReviewActionResult` — **already the right primitive** |
| `pkg/wdk/tx_status.go` `IsInFlight()` (~L125–136) | Already on main (Track S): `{sending, unsent, unprocessed}` — **reuse; do not re-add** |
| `pkg/wdk/storage_process_action_result.go` | Canonical `SendWithResult` / `ReviewActionResult` types reused by ProcessAction |
| `pkg/wallet/internal/mapping/mapping_internalize_action.go` `MapInternalizeActionResult` (~L88–92) | SDK map only exposes `Accepted` (go-sdk shape) — storage/RPC fix is independent |

### Impact

- Internalized payments/basket insertions may lag on the network until the background worker drains the channel (or until `SendWaitingTransactions` if the enqueue fails).
- Callers cannot distinguish soft service errors from silent success.
- Window for double-spend of unbroadcast incoming funds is larger than TS.

---

## Root cause

1. **Result type incomplete** — `wdk.InternalizeActionResult` omits optional broadcast fields that ProcessAction and TS already share.
2. **Broadcast is async and opaque** — `BackgroundBroadcaster.Add` is best-effort fire-and-forget; its `BackgroundBroadcast` review results never reach the Internalize RPC response.
3. **No post-commit sync hook** — `Internalize` returns immediately after UoW + optional mined update, with no path to call `process.BackgroundBroadcast` and map outcomes.

The delayed ProcessAction / monitor architecture is **not** wrong and must stay. The gap is specifically: for **new unknown unproven** internalize, TS broadcasts in-band for parity and observability; Go only queues.

---

## Recommended approach

### Design principles

- **Reuse** `(*process).BackgroundBroadcast` (same post + status apply as delayed workers). Do **not** invent a parallel broadcaster for internalize.
- Broadcast **after** the UoW commits (storage must succeed even if broadcast soft-fails).
- Use the **request BEEF** already verified in `Internalize` — do not rebuild BEEF from shared KnownTx (another user may hold incomplete/in-flight raw bytes).
- Soft failures: keep `accepted: true`, populate result fields, leave KnownTx retryable for monitor/`SendWaitingTransactions`.
- Hard post errors: log, surface `sendWithResults: [{status: sending}]`, clear error so internalize does not reject; do **not** abort/fail the internalized user transaction on post errors (incoming funds must not be failed because ARC blipped).
- Merge path and mined/`AlreadySent` path: **no** broadcast result fields (omit empty via `omitempty`).

### Files to change

| File | Change |
|------|--------|
| `pkg/wdk/storage_internalize_action_args.go` | Add optional `SendWithResults []SendWithResult` and `NotDelayedResults []ReviewActionResult` with `json:",omitempty"` |
| `pkg/storage/internal/actions/internalize.go` | Replace `backgroundBroadcaster` dependency with `*process`; `storeNewTx` returns `shouldBroadcast`; capture flag out of UoW; post-commit sync broadcast + map results; helper `sendWithResultsFromReview` |
| `pkg/storage/internal/actions/actions.go` | Pass `processAction` into `newInternalizeAction` instead of `processAction.backgroundBroadcaster` |
| `pkg/storage/provider_internalize_action_test.go` | Assert result fields on happy path; drop ~10× `time.Sleep(200ms)` async waits; add serviceError + merge-omit tests; adapt reorg intermediate-state test for sync broadcast |
| `pkg/storage/internal/integrationtests/internalize_create_process_test.go` | Update internalize-result expectations / comments (post-internalize KnownTx is already `unmined` on happy path after sync broadcast) |

Optional / follow-up (out of minimal fix, document as non-goal or small add-on):

| File | Note |
|------|------|
| `pkg/wdk/tx_status.go` | **`IsInFlight()` already exists** on main (Track S). Prefer it over inventing a second predicate or using the broader `Sending()` (which includes terminal-ish `invalid` / `doubleSpend`). |
| `pkg/wallet/internal/mapping/mapping_internalize_action.go` | go-sdk `InternalizeActionResult` still only has `Accepted`. Do **not** block the storage fix on SDK expansion. |

### Detailed algorithm

#### 1. Extend `InternalizeActionResult`

```go
type InternalizeActionResult struct {
	Accepted          bool                 `json:"accepted"`
	IsMerge           bool                 `json:"isMerge"`
	TxID              string               `json:"txid"`
	Satoshis          int64                `json:"satoshis"`
	SendWithResults   []SendWithResult     `json:"sendWithResults,omitempty"`
	NotDelayedResults []ReviewActionResult `json:"notDelayedResults,omitempty"`
}
```

Same shapes as `ProcessActionResult` fields — reuse types, do not redefine.

#### 2. Wire `*process` into `internalize`

- Struct field: `process *process` (replaces `backgroundBroadcaster *service.BackgroundBroadcaster`).
- `newInternalizeAction(..., processAction *process)`.
- `actions.New`: pass `processAction` (created before internalize already).

#### 3. `storeNewTx` → `(shouldBroadcast bool, err error)`

Keep storage semantics; change only the broadcast decision surface. **Do not** call `backgroundBroadcaster.Add` inside the UoW anymore — persistence only.

| Condition | KnownTx / user tx status (existing) | `shouldBroadcast` |
|-----------|-------------------------------------|-------------------|
| `tx.MerklePath != nil` (mined in BEEF) | unmined / unproven | `false` |
| Existing KnownTx `AlreadySent()` | unmined / unproven | `false` |
| Existing KnownTx `IsInFlight()` (cross-user / concurrent path) | leave KnownTx as-is via `SkipForStatuses`; still create this user’s sending row | `false` — **do not re-post** |
| Fresh unknown unproven (incl. reorg re-queue: `AlreadySent(reorg)=false`) | unsent / sending | `true` |

**In-flight skip (important):**

`IsInFlight()` is **already on main** (`pkg/wdk/tx_status.go` ~L125–136): `{sending, unsent, unprocessed}`. Use it:

```go
if existingStatus, ok := statuses[txID]; ok {
    if existingStatus.AlreadySent() {
        alreadySent = true
    } else if existingStatus.IsInFlight() {
        // Another path owns the post; persist this user's row but skip re-broadcast.
        shouldBroadcast = false
    }
}
```

Do **not** use the broader `Sending()` predicate (includes terminal-ish `invalid` / `doubleSpend`). Reorged KnownTx (`reorg`) is neither `AlreadySent` nor `IsInFlight`, so it correctly re-queues (`shouldBroadcast=true`) — keep that W1-6 behaviour.

`SkipForStatuses` already protects shared KnownTx rows from being rewritten while `unsent|sending|unmined`; the in-flight check only gates the **post**, not the user-tx insert.

#### 4. Capture `shouldBroadcast` out of the UoW, then post-commit broadcast

`storeNewTx` runs inside the UoW callback. Declare an outer flag and assign it from the return value (merge path leaves it `false`):

```text
var shouldBroadcast bool

uow.Do(...):
  if isMerge:
    upsertExistingTx(...)
  else:
    shouldBroadcast, err = storeNewTx(...)   // no Add() inside

// after UoW success:
if tx.MerklePath != nil:
  updateKnownTxAsMined(...)   // existing best-effort path

result := {Accepted, IsMerge, TxID, Satoshis}

if shouldBroadcast && in.process != nil:
  reviewResults, err := in.process.BackgroundBroadcast(ctx, beef, []string{txID})
  if err != nil:
    log warn "broadcast after internalize failed; leaving tx for monitor retry"
    result.SendWithResults = [{TxID, Status: sending}]
    err = nil   // internalize still succeeds
  else:
    result.NotDelayedResults = reviewResults
    result.SendWithResults = sendWithResultsFromReview(reviewResults)

return result, nil
```

Always post the **request BEEF** already verified in `Internalize` (do not rebuild from shared KnownTx raw bytes).

#### 5. Map review → sendWith

Mirror `singleTxBroadcastResult` (`process.go` ~L850–903) — same statuses ProcessAction exposes:

| `ReviewActionResultStatus` | `SendWithResultStatus` |
|----------------------------|------------------------|
| `success` | `unproven` |
| `serviceError` | `sending` (still in-flight; monitor retries) |
| `doubleSpend` / `invalidTx` | `failed` |
| default | `sending` |

Implement as a small private helper in `internalize.go` (e.g. `sendWithResultsFromReview`). Prefer mapping from the returned `[]ReviewActionResult` rather than re-running aggregation.

### Status / attempt side-effects (know these)

- `BackgroundBroadcast` → `PostFromBEEF` → `updateSingleTx` transitions KnownTx (e.g. success → `unmined`, serviceError → `sending`), then **bumps attempts once at the end** of the completed post round (`IncreaseKnownTxAttemptsForTxIDs`, ~L1004). Happy-path tests should assert `WithAttempts(1)` and final KnownTx status accordingly.
- Foreground ProcessAction calls `MarkKnownTxsAsSubmitting` before posting; **`BackgroundBroadcast` does not**. Internalize’s initial KnownTx is `unsent`, so there is no `unprocessed→sending` edge on this path. Do not expand that state machine in this fix; monitor / `SendWaitingTransactions` remain the retry path for soft failures and hard post errors.
- Removing in-UoW `Add` means the async channel path is no longer used for this call (ProcessAction delayed sends still use it).

---

## Test strategy

### Unit / provider tests (`pkg/storage/provider_internalize_action_test.go`)

There are ~10 `time.Sleep(200 * time.Millisecond) // wait for background broadcaster` calls in this file today. After the sync path, **delete them** (broadcast completes before `InternalizeAction` returns). Prefer direct assertions; keep `require.Eventually` only where concurrency is intentional (reorg hold test).

1. **Happy path (wallet payment / basket insertion)**
   - Assert `SendWithResults[0].Status == unproven`, `NotDelayedResults[0].Status == success`.
   - Assert KnownTx `unmined`, `WithAttempts(1)`.
   - History notes still include `postBeefSuccess` / `aggregateResults` (same as today’s sleep-waited checks).

2. **Mined BEEF** (`TestInternalizeAction_UpdateKnownTxAsMined_HappyPath`)
   - Assert empty / nil `SendWithResults` and `NotDelayedResults` (no re-broadcast; `omitempty`).

3. **Service error surfaces fields** (new)
   - Soft-error fixture used by ProcessAction: e.g. `ARC().WhenQueryingTx(txID).WillReturnNoBody()` (see `provider_process_action_test.go`).
   - Expect `accepted=true`, `SendWithResults: sending`, `NotDelayedResults: serviceError`, KnownTx stays `sending`, user tx `sending`.

4. **Merge path omits fields** (new)
   - Internalize an already-owned faucet tx as merge (`TestInternalizeActionForAlreadyStoredTransaction` pattern).
   - Assert `IsMerge=true` and both result slices empty/nil.

5. **Reorg intermediate state** (`TestInternalizeAction_ReorgedKnownTx_DoesNotClaimNetworkAcceptance`)
   - Today the test holds ARC and calls `InternalizeAction` on the test goroutine (async `Add` returns immediately). With sync broadcast that pattern **deadlocks**.
   - Required shape: `HoldBroadcasting` + `defer ReleaseBroadcasting()` (always release — provider `Stop` waits on in-flight workers), run `InternalizeAction` in a goroutine, `Eventually` assert KnownTx `unsent|sending` after UoW commit, then release ARC and wait for the goroutine.
   - Assert result still `accepted` + non-empty `SendWithResults` after release. Keep W1-6 DB pins (`reorg` → rewrite to `unsent`, `WasBroadcast(true)`, Bob user tx `sending`).

6. **Cross-user / multi-user** (`TestInternalizeTheSameTxByDifferentUsers`, etc.)
   - Drop sleeps; keep DB assertions.
   - After Alice’s successful sync broadcast, Bob’s internalize should hit `AlreadySent()` / merge-or-store path with `shouldBroadcast=false` (no second post). Concurrent same-tick double internalize is a residual race (both may see non-in-flight before either posts); document but do not block the fix on a full distributed lock.

### Integration

`pkg/storage/internal/integrationtests/internalize_create_process_test.go`:

- Update comments that still say “after internalize the unmined tx is in Unsent/Sending state” — on the happy path KnownTx is already `unmined` once `InternalizeAction` returns.
- If any golden/JSON dump of the internalize result is asserted, include optional `sendWithResults` / `notDelayedResults` for the unproven faucet path (exact `reference` comes from `TestRandomizer` fixtures already used there). ProcessAction assertions later in the file already exercise `SendWithResults` and should remain green.

### Commands

```bash
go test ./pkg/storage/ -run TestInternalizeAction -count=1
go test ./pkg/storage/internal/integrationtests/ -count=1
go test ./pkg/storage/ ./pkg/storage/internal/actions/ ./pkg/wdk/ -count=1
```

---

## Acceptance criteria

- [ ] New unknown unproven internalize **broadcasts before return** via `process.BackgroundBroadcast` (request BEEF).
- [ ] `wdk.InternalizeActionResult` includes optional `sendWithResults` and `notDelayedResults` with ProcessAction-compatible shapes and JSON tags (`omitempty`).
- [ ] Soft service errors leave `accepted: true` and populate both result fields (`sending` / `serviceError`); KnownTx remains retryable.
- [ ] Hard post errors do not fail the Internalize RPC; result shows in-flight `sending`; monitor/`SendWaiting` can retry.
- [ ] Merge path and mined/`AlreadySent` path omit both optional fields.
- [ ] Existing KnownTx with `IsInFlight()` does **not** re-post (cross-user / concurrent skip).
- [ ] Reorg re-queue still broadcasts (`AlreadySent(reorg)=false`, not `IsInFlight`).
- [ ] No in-UoW fire-and-forget `backgroundBroadcaster.Add` for this path.
- [ ] Provider + integration tests above green; no regressions in ProcessAction or monitor send-waiting tests; no leftover `time.Sleep` broadcaster waits in internalize tests.
- [ ] Issue #818 remains open until an implementation PR lands with `Fixes #818` (this plan PR only uses `Related to #818`).

---

## Implementation order (suggested)

1. Extend `wdk.InternalizeActionResult` + compile.
2. Wire `*process` into `internalize` / `actions.New`; remove `backgroundBroadcaster` field from `internalize`.
3. Change `storeNewTx` to return `shouldBroadcast` (AlreadySent / mined / **IsInFlight** gates); delete in-UoW `Add`.
4. Post-commit `BackgroundBroadcast` + `sendWithResultsFromReview` helper + hard-error soft-success.
5. Rewrite provider tests (drop sleeps; add serviceError + merge-omit; reorg goroutine hold).
6. Touch integration comments/expectations; run the three test commands above.

---

## Risks / gotchas

| Risk | Mitigation |
|------|------------|
| Sync broadcast increases Internalize latency under ARC lag | Match TS; hold is intentional for caller visibility. Timeouts already exist on service clients. |
| Holding ARC in tests can deadlock provider Stop / test goroutine | Always `defer ReleaseBroadcasting()`; run blocking `InternalizeAction` on a separate goroutine when holding ARC. |
| Shared KnownTx + incomplete raw bytes | Always post the verified request BEEF, not a reconstituted one. |
| Double broadcast with background worker | Removing `Add` for this path avoids double post on the same call; `IsInFlight()` skip protects cross-user re-post after the first path has written `unsent|sending|unprocessed`. |
| Residual concurrent double-post race | Two internalize calls that both read empty KnownTx before either commits can both set `shouldBroadcast=true`. Accept for M; same class of race exists today with dual `Add`. |
| `BackgroundBroadcast` never calls `MarkKnownTxsAsSubmitting` | Known; attempts still bump after the post round. Raise separately if product wants an explicit `unsent→sending` edge before post. |
| Wallet SDK still only maps `Accepted` | Storage/RPC parity is the issue scope; SDK field expansion is a separate go-sdk + mapping change. |
| Cycle risk wiring `process` into `internalize` | `process` is constructed first in `actions.New` today; pass the pointer — no new package cycle (same package `actions`). |

---

## Non-goals

- Changing merge semantics, output/basket construction, or BEEF verification.
- Expanding go-sdk `InternalizeActionResult` or wallet mapping beyond storage.
- Replacing or removing the background broadcaster for ProcessAction delayed sends.
- Altering `AlreadySent` / reorg WasBroadcast semantics (Track S territory).
- Closing #818 from the plan PR.

---

## Dependencies

- Relies on existing `(*process).BackgroundBroadcast` (`process.go` ~L952–1009).
- Relies on existing `ProvenTxReqStatus.IsInFlight()` (`tx_status.go` ~L125–136) — already shipped with Track S; no new predicate required.
- Relies on ARC/test fixtures already used by ProcessAction and internalize suites (`givenProvider.ARC()`, `HoldBroadcasting` / `ReleaseBroadcasting`, `WhenQueryingTx(...).WillReturnNoBody()`, etc.).
- Conformance schema note already documents the optional fields (`adapter-conformance.json` ~L390); no vector file rewrite required for the minimal fix (happy-path vector may still omit optional fields).

---

## Estimated size

**M** — focused API surface + one action path + test rewrites (including reorg concurrency reshape and sleep removal); no schema migration. Prior draft (#945) is a usable implementation sketch once rebased on current `main`, but re-check against Track S status predicates (`IsInFlight`, reorg `AlreadySent` exclusion) rather than copying the draft blindly.

---

## Useful cross-references

- Issue: <https://github.com/bsv-blockchain/go-wallet-toolbox/issues/818>
- Closed draft impl: <https://github.com/bsv-blockchain/go-wallet-toolbox/pull/945>
- TS reference: `wallet-toolbox` `internalizeAction.ts` `newInternalize` + `shareReqsWithWorld`
- ProcessAction result types: `pkg/wdk/storage_process_action_result.go`
- Review ↔ sendWith mapping source: `pkg/storage/internal/actions/process.go` `singleTxBroadcastResult` (~L850–903)
- Sync broadcast primitive: `pkg/storage/internal/actions/process.go` `BackgroundBroadcast` (~L952–1009)
- Background broadcaster (current async path for delayed ProcessAction): `pkg/storage/internal/service/background_broadcaster.go`
- Storage adapter schema note: `conformance/vectors/wallet/storage/adapter-conformance.json` (~L390)
- Related status semantics: `pkg/wdk/tx_status.go` (`AlreadySent`, `IsInFlight`, `Sending`, `SendWithResultStatus`)
