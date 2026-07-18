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

| Location | What happens |
|----------|----------------|
| `pkg/storage/internal/actions/internalize.go` `Internalize` (~L76–291) | UoW store, optional `updateKnownTxAsMined`, return fixed 4-field result |
| `storeNewTx` (~L401–491) | Sets KnownTx `unsent` + user tx `sending` for unproven; if mined/`AlreadySent` → `unmined`/`unproven` and skips broadcaster |
| `storeNewTx` (~L482–489) | `backgroundBroadcaster.Add(beef, []string{txID})` — async, result discarded |
| `pkg/storage/internal/actions/actions.go` (~L74–85) | Wires `processAction.backgroundBroadcaster` into `newInternalizeAction` |
| `pkg/storage/internal/actions/process.go` `BackgroundBroadcast` (~L863–907) | Sync post + `updateSingleTx`; returns `[]ReviewActionResult` — **already the right primitive** |
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
| `pkg/storage/internal/actions/internalize.go` | Replace `backgroundBroadcaster` dependency with `*process`; `storeNewTx` returns `shouldBroadcast`; post-commit sync broadcast + map results; helper `sendWithResultsFromReview` |
| `pkg/storage/internal/actions/actions.go` | Pass `processAction` into `newInternalizeAction` instead of `processAction.backgroundBroadcaster` |
| `pkg/storage/provider_internalize_action_test.go` | Assert result fields on happy path; add serviceError + merge-omit tests; adapt reorg intermediate-state test for sync broadcast |
| `pkg/storage/internal/integrationtests/internalize_create_process_test.go` | Update golden JSON for new optional fields |

Optional / follow-up (out of minimal fix, document as non-goal or small add-on):

| File | Note |
|------|------|
| `pkg/wdk/tx_status.go` | If you need a narrow “in-flight broadcast” predicate, add `IsInFlight()` (or reuse carefully — see below). `Sending()` today includes terminal-ish states (`invalid`, `doubleSpend`) and is broader than “someone else is posting this tx”. |
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

Keep storage semantics; change only the broadcast decision surface:

| Condition | KnownTx / user tx status (existing) | `shouldBroadcast` |
|-----------|-------------------------------------|-------------------|
| `tx.MerklePath != nil` (mined in BEEF) | unmined / unproven | `false` |
| Existing KnownTx `AlreadySent()` | unmined / unproven | `false` |
| Existing KnownTx already in-flight by another path | unsent / sending (etc.) — **do not re-post** | `false` |
| Fresh unknown unproven | unsent / sending | `true` |

**In-flight skip (important):**

Today main has `AlreadySent()` and `Sending()`, but no `IsInFlight()`. Options:

1. **Preferred:** add a small helper on `ProvenTxReqStatus`, e.g. `IsInFlight()`, covering statuses where another worker owns the post (`unsent`, `sending`, `unprocessed`, and any other “actively being posted” states you confirm against ProcessAction). Keep it narrower than `Sending()` if `invalid`/`doubleSpend` should still allow a deliberate re-queue (internalize of a reorged/`WasBroadcast` path is already handled separately).
2. **Minimal:** inline `switch` on `unsent|sending|unprocessed` at the call site without a named helper.

Do **not** call `backgroundBroadcaster.Add` inside the UoW anymore. Persistence only.

#### 4. Post-commit broadcast in `Internalize`

After UoW success (and existing `updateKnownTxAsMined` when applicable):

```text
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

#### 5. Map review → sendWith

Mirror ProcessAction expectations:

| `ReviewActionResultStatus` | `SendWithResultStatus` |
|----------------------------|------------------------|
| `success` | `unproven` |
| `serviceError` | `sending` (still in-flight; monitor retries) |
| `doubleSpend` / `invalidTx` | `failed` |
| default | `sending` |

Implement as a small private helper in `internalize.go` (e.g. `sendWithResultsFromReview`).

### Status / attempt side-effects (know these)

- `BackgroundBroadcast` → `PostFromBEEF` → `updateSingleTx` increments attempts and transitions KnownTx (e.g. success → `unmined`, serviceError → `sending`). Happy-path tests should assert `WithAttempts(1)` and final KnownTx status accordingly.
- `MarkKnownTxsAsSubmitting` (if still only transitions `unprocessed` → `sending`) is a **no-op** for internalize’s initial `unsent` — same as today’s delayed path. Do not expand its state machine in this fix unless you prove a regression; document as a known no-op.
- Removing in-UoW `Add` means the async channel path is no longer used for this call. Monitor `SendWaitingTransactions` remains the retry path for soft failures / hard post errors.

---

## Test strategy

### Unit / provider tests (`pkg/storage/provider_internalize_action_test.go`)

1. **Happy path (wallet payment / basket insertion)**  
   - Assert `SendWithResults[0].Status == unproven`, `NotDelayedResults[0].Status == success`.  
   - Assert KnownTx `unmined`, `WithAttempts(1)`.  
   - **Remove** `time.Sleep` waits for background broadcaster (broadcast is sync).

2. **Mined BEEF** (`TestInternalizeAction_UpdateKnownTxAsMined_HappyPath`)  
   - Assert empty `SendWithResults` / `NotDelayedResults` (no re-broadcast).

3. **Service error surfaces fields** (new)  
   - `ARC().WhenQueryingTx(txID).WillReturnNoBody()` (or equivalent soft-error fixture used by ProcessAction tests).  
   - Expect `accepted=true`, `SendWithResults: sending`, `NotDelayedResults: serviceError`, KnownTx stays `sending`, user tx `sending`.

4. **Merge path omits fields** (new)  
   - Internalize an already-owned faucet tx as merge.  
   - Assert `IsMerge=true` and both result slices empty.

5. **Reorg intermediate state** (`TestInternalizeAction_ReorgedKnownTx_DoesNotClaimNetworkAcceptance`)  
   - Because broadcast is now synchronous, hold ARC POST, run `InternalizeAction` in a goroutine, `Eventually` assert KnownTx `unsent|sending` after storeNewTx, then release ARC and wait for completion.  
   - Assert result still accepted + non-empty `SendWithResults` after release.

6. **Existing merge / multi-user tests**  
   - Drop sleeps that only waited for the old async broadcaster; keep DB assertions.

### Integration golden

`pkg/storage/internal/integrationtests/internalize_create_process_test.go` — extend the internalize result JSON to include `sendWithResults` / `notDelayedResults` for the unproven faucet path (exact `reference` comes from `TestRandomizer` fixtures already used there).

### Commands

```bash
go test ./pkg/storage/ -run TestInternalizeAction -count=1
go test ./pkg/storage/internal/integrationtests/ -count=1
go test ./pkg/storage/ ./pkg/storage/internal/actions/ ./pkg/wdk/ -count=1
```

---

## Acceptance criteria

- [ ] New unknown unproven internalize **broadcasts before return** via `process.BackgroundBroadcast` (request BEEF).
- [ ] `wdk.InternalizeActionResult` includes optional `sendWithResults` and `notDelayedResults` with ProcessAction-compatible shapes and JSON tags.
- [ ] Soft service errors leave `accepted: true` and populate both result fields (`sending` / `serviceError`); KnownTx remains retryable.
- [ ] Hard post errors do not fail the Internalize RPC; result shows in-flight `sending`; monitor/`SendWaiting` can retry.
- [ ] Merge path and mined/`AlreadySent` path omit both optional fields.
- [ ] Cross-user / already in-flight KnownTx does **not** double-post (skip re-broadcast).
- [ ] No in-UoW fire-and-forget `backgroundBroadcaster.Add` for this path.
- [ ] Provider + integration tests above green; no regressions in ProcessAction or monitor send-waiting tests.
- [ ] Issue #818 remains open until an implementation PR lands with `Fixes #818` (this plan PR only uses `Related to #818`).

---

## Risks / gotchas

| Risk | Mitigation |
|------|------------|
| Sync broadcast increases Internalize latency under ARC lag | Match TS; hold is intentional for caller visibility. Timeouts already exist on service clients. |
| Holding ARC in tests can deadlock provider Stop | Always release ARC on all exit paths (`defer` + once-flag pattern from #945 draft). |
| Shared KnownTx + incomplete raw bytes | Always post the verified request BEEF, not a reconstituted one. |
| Double broadcast with background worker | Removing `Add` for this path avoids double post on the same call; in-flight skip protects concurrent cross-user internalize. |
| `MarkKnownTxsAsSubmitting` no-op on `unsent` | Acceptable; attempts still bump via broadcast path. Raise separately if product wants an explicit `unsent→sending` edge. |
| Wallet SDK still only maps `Accepted` | Storage/RPC parity is the issue scope; SDK field expansion is a separate go-sdk + mapping change. |
| Cycle risk wiring `process` into `internalize` | `process` is constructed first in `actions.New` today; pass the pointer — no new package cycle. |

---

## Non-goals

- Changing merge semantics, output/basket construction, or BEEF verification.
- Expanding go-sdk `InternalizeActionResult` or wallet mapping beyond storage.
- Replacing or removing the background broadcaster for ProcessAction delayed sends.
- Altering `AlreadySent` / reorg WasBroadcast semantics (Track S territory).
- Closing #818 from the plan PR.

---

## Dependencies

- Relies on existing `(*process).BackgroundBroadcast` (`process.go` ~L863+).
- Relies on ARC/test fixtures already used by ProcessAction and internalize suites (`givenProvider.ARC()`, `HoldBroadcasting`, etc.).
- Conformance schema note already documents the optional fields; no vector file rewrite required for the minimal fix (happy-path vector may still omit optional fields).

---

## Estimated size

**M** — focused API surface + one action path + test rewrites; no schema migration. Prior draft (#945) is a usable implementation sketch once rebased on current `main`.

---

## Useful cross-references

- Issue: <https://github.com/bsv-blockchain/go-wallet-toolbox/issues/818>
- Closed draft impl: <https://github.com/bsv-blockchain/go-wallet-toolbox/pull/945>
- TS reference: `wallet-toolbox` `internalizeAction.ts` `newInternalize` + `shareReqsWithWorld`
- ProcessAction result types: `pkg/wdk/storage_process_action_result.go`
- Background broadcaster (current async path): `pkg/storage/internal/service/background_broadcaster.go`
- Storage adapter schema note: `conformance/vectors/wallet/storage/adapter-conformance.json` (~L390)
- Related status semantics: `pkg/wdk/tx_status.go` (`AlreadySent`, `Sending`, `SendWithResultStatus`)
