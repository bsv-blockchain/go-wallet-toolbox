# Decision Record v1 — Track P (Funder Performance & Remaining Hardening) Design

Date: 2026-07-17
Branch: `feat/dr-v1-track-p` (base: `e0f8910`, main after PR #934 merge, v0.183.19)
Sources: UTXO-selection review consolidation (5 persona docs, all findings adversarially upheld), Decision Record v1 (discussion #933) follow-up list, Track S validation scorecard (2026-07-17).

## Goal

Remove the funder's O(N) whole-pool load + FOR-UPDATE lock (the dominant TPS ceiling and same-user serializer), land the remaining upheld review findings that Track S deliberately deferred, and put a measured benchmark baseline under all of it.

## Scope (findings addressed)

| ID | Finding | Track P task |
|---|---|---|
| P1-3 | Funder loads+locks entire (user,basket) pool per createAction | T7 bounded fetch |
| P2-2 | OFFSET+SKIP LOCKED re-return → spurious contention | T6 tiebreaker+dedup (sweep path); T7 removes OFFSET paging from non-sweep path |
| P2-6 | ErrUTXOContention retry contract unimplemented; sentinel unexportable | T8 bounded retry + public `wdk` sentinel |
| P2-4 | Div-by-zero panic on `MinimumDesiredUTXOValue == 0` | T2 |
| P2-1 (residual) | No composite selection index; `reserved_by_id` unindexed | T3 |
| P2-5 | Unguarded `reserved_by_id` writer via `UserUTXOs.Update` | T4 |
| P2-11 | Zero benchmarks repo-wide | T5 (baseline BEFORE T7) |
| P1-4/P2-9 (residual) | No change-path concurrency test; `reserveUTXOs` contention branch untested | T9 |
| P2-3 | SQLite: no WAL / busy_timeout | T10 |
| P2-7 / NIT-1 | Pool default undocumented; MySQL 8.0.1+ minimum undocumented | T11 |
| stale comment | `concurrency_create_action_test.go:24-29` still says "INTENTIONALLY RED / W1-1 not landed" (false since Track S) | T1 |
| NIT-2 | Dead `SortBy` config in `loadUTXOPool` | folded into T7 |

Out of scope: W2-2/W2-3/W3a-2 (separate DR waves), MySQL CI/testmode (NIT-4), pool-default value change (docs only — changing deployment defaults needs maintainer sign-off).

## Core design — T7 bounded fetch

### Why page-bounding is not enough

Bounding `loadUTXOPool` to "stop paging once funded" still locks up to 1000 rows (page 1) — for typical wallets that IS the whole pool, so same-user serialization and false `ErrNotEnoughFunds` persist. The fix must lock only rows actually considered.

### Chosen approach: per-allocation target-aware micro-queries

Current selection semantics (`pool.selectBest`, per tier mined→unproven→sending): (1) exact match, (2) smallest sufficient, (3) largest insufficient. These map exactly onto two indexed queries per tier:

- **Q-sufficient:** `WHERE user_id=? AND basket_name=? AND reserved_by_id IS NULL AND utxo_status=? AND satoshis >= remaining AND output_id NOT IN (forbidden+allocated) ORDER BY satoshis ASC LIMIT 1` — first row is the exact match if one exists, else the smallest sufficient. Replaces stages 1+2.
- **Q-accumulate:** same predicate with `satoshis < remaining ORDER BY satoshis DESC LIMIT 16` — largest-insufficient batch. Replaces stage 3; batching amortizes round trips when many small inputs are needed.

Funder non-sweep loop: while `!collector.IsFunded()`: compute `remaining`; for each tier in order: try Q-sufficient (allocate 1, recompute); else drain Q-accumulate batch (allocate greedily, recompute after each); next tier only when both empty. All tiers empty → `wdk.ErrNotEnoughFunds`.

Both queries carry `FOR UPDATE SKIP LOCKED` on Postgres/MySQL (same dialect gate as today). Rows we already allocated in this transaction remain visible to our own queries (own locks don't block, `reserved_by_id` still NULL until `reserveUTXOs`), so **already-allocated output IDs must be appended to the exclusion list** on every query.

Properties:
- Locks held per call: O(allocated + small constant), not O(pool).
- Selection result identical to `selectBest` for every single allocation step (proof: ASC-first-row ≡ exact-else-smallest-sufficient; DESC-first ≡ largest insufficient; tier walk order preserved).
- Concurrent same-user callers skip-lock past each other's handful of locked rows → parallel same-user funding works; false `ErrNotEnoughFunds` shrinks to genuine pool exhaustion.
- No OFFSET pagination on this path → P2-2 gone for non-sweep.
- SQLite: same queries minus the lock clause; `reserveUTXOs` CAS (Track S W1-1 layer) remains the guard, now backed by T8 retry.

Sweep keeps the exhaustive pager (needs every row) — hardened by T6 (unique `output_id ASC` tiebreaker in ORDER BY + dedup by OutputID during accumulation).

`allocatePriorityOutputs` (phase 1) unchanged.

Repository interface change (`funder.UTXORepository`): replace the single `FindNotReservedUTXOsForUpdate` consumer contract with three methods (repo keeps the old one for sweep):

```go
type UTXORepository interface {
    // sweep only — exhaustive, ordered, deduped
    FindNotReservedUTXOsForUpdate(ctx, tx, userID, basketName, page, forbiddenOutputIDs, includeSending) ([]*models.UserUTXO, error)
    // bounded: smallest UTXO with satoshis >= minSatoshis in the given status, or nil
    FindSmallestSufficientUTXOForUpdate(ctx, tx, userID, basketName string, status wdk.UTXOStatus, minSatoshis uint64, excludedOutputIDs []uint) (*models.UserUTXO, error)
    // bounded: up to `limit` UTXOs with satoshis < maxSatoshis in the given status, largest first
    FindLargestInsufficientUTXOsForUpdate(ctx, tx, userID, basketName string, status wdk.UTXOStatus, maxSatoshis uint64, limit int, excludedOutputIDs []uint) ([]*models.UserUTXO, error)
}
```

`utxoPool`/`selectBest` remain only for the sweep path's `all()`; non-sweep no longer builds a pool.

Edge cases pinned by tests: `remaining` recomputation after each allocation (fee grows with input count — a previously-sufficient row may become insufficient; loop recomputes before every query); exclusion list correctness (no row allocated twice); tier exhaustion ordering; `includeSending` gate (sending tier queried only when true); zero-candidate → `ErrNotEnoughFunds`.

## T8 — contention retry + public sentinel

- New `wdk.ErrUTXOContention` (public). `repo.ErrUTXOContention` becomes `fmt.Errorf("utxo contention: ...: %w", wdk.ErrUTXOContention)` so `errors.Is` matches through both.
- `create.go`: wrap the funding `c.db.Transaction(...)` closure in a bounded retry — 3 attempts total, `errors.Is(err, wdk.ErrUTXOContention)` only, 25ms×attempt sleep with ±50% jitter (`c.random`), context-cancellation aware. `existingUTXOs` snapshot reused across attempts (staleness only affects change count; documented). Each retry re-runs Fund → fresh selection excludes now-reserved rows.
- Doc comment on `reserveUTXOs` updated to name the actual retry site.

## T2 — MinimumDesiredUTXOValue=0 guard

Three layers: (1) `newCollector` clamps `minimumDesiredUTXOValue = max(1, v)` (defense in depth — panic becomes impossible); (2) `ValidBasketConfiguration` rejects `MinimumDesiredUTXOValue == 0` when provided; (3) `Provider.UpdateChangeBasket` rejects 0. Tests drive the previously-panicking path.

## T3 — selection index

`models.UserUTXO` tags: composite `idx_user_utxos_selection` over (user_id, basket_name, reserved_by_id, utxo_status, satoshis) — exactly the T7 query shape — plus single-column index on `reserved_by_id` (serves `UnreserveUTXOsByTransactionID`). AutoMigrate-only, additive (existing deployments gain them on upgrade; nothing dropped). Postgres test asserts both exist via `pg_indexes`.

## T4 — guard the second reserved_by_id writer

`UserUTXOs.Update`: when `spec.ReservedByID != nil`, add `WHERE reserved_by_id IS NULL` precondition and require `RowsAffected == 1`, returning an error wrapping `wdk.ErrUTXOContention` on mismatch. Other fields keep current semantics. (Callers today are test-only; this closes the latent hole without breaking the CRUD surface.)

## T5 — benchmarks (before T7)

`BenchmarkSQLFund` in `pkg/internal/storage/funder`: in-memory SQLite fixture, seeded pools of 1_000 and 10_000 UTXOs, target requiring 1-3 inputs; each iteration runs `Fund` inside a fresh gorm transaction (Fund only reads/locks — no state mutation, so pool stays constant). Baseline numbers recorded in the SDD ledger before T7 lands; re-run after T7 for the delta.

## T9 — test hardening

- Postgres concurrency test #2 (change path): seed M=4 change UTXOs sized to fund exactly one action each (values chosen so one UTXO covers target+fee, change below dust floor), launch N=8 concurrent CreateActions, assert exactly 4 succeed, 4 fail with `ErrNotEnoughFunds`, all allocated outputs distinct, `AssertStorageInvariants` passes. (With T8 retry, losers retry then fail genuinely.)
- Direct `reserveUTXOs` contention unit test (mirrors `transactions_claim_test.go`): pre-reserve a UserUTXO row, drive `CreateTransactionInTx` with that output in `ReservedOutputIDs`, assert `errors.Is(err, wdk.ErrUTXOContention)` and rollback.

## T10 — SQLite WAL + busy_timeout

`sqliteDialector` (dialectors.go): for file-based DSNs (not `:memory:`), append `_journal_mode=WAL&_busy_timeout=5000` unless the DSN already sets either parameter (operator override wins). Rationale: WAL removes global read/write blocking on commit; busy_timeout is mattn's default made explicit and survivable if the driver default changes. In-memory DSNs untouched (WAL is meaningless there; test fixtures unaffected).

## T11 — docs

`docs/storage.md`: MySQL minimum 8.0.1 (SKIP LOCKED); connection-pool default of 5 documented with guidance to raise for Postgres/MySQL under load; SQLite WAL note.

## Error handling & compatibility

- No breaking public API changes. New public symbol: `wdk.ErrUTXOContention`.
- Schema changes additive only (two new indexes). AutoMigrate applies them; release notes must mention potential index-build time on large `bsv_user_utxos` tables.
- Behavior change (documented): funder now prefers safest-tier bounded selection identical per-step to the old best-fit; for baskets >1000 eligible rows the old code could see the whole pool at once — per-step semantics identical, whole-sequence outcomes may differ only in pathological multi-input cases. Sweep unchanged.
- Retry changes failure profile under contention: previously instant `ErrUTXOContention`/false `ErrNotEnoughFunds`; now up to 2 retries then the genuine error.

## Testing strategy

Every task: unit tests both engines (`go test ./...` sqlite; targeted `TEST_DB_MODE=postgres -p 1` for storage packages), plus the postgres CI workflow already covering `pkg/internal/storage/... pkg/storage/... pkg/internal/testabilities/... pkg/monitor/...`. T7 additionally: full funder test suite must pass unmodified (selection-semantics regression net), plus new bounded-fetch unit tests, plus benchmark delta.
