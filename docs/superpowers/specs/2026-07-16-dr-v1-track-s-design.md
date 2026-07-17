# Track S Start — Design Spec (Decision Record v1, Discussion #933)

**Date:** 2026-07-16
**Basis:** Frozen [Decision Record v1](https://github.com/bsv-blockchain/go-wallet-toolbox/discussions/933) (quorum: 3 agent ACKs + maintainer, 0 BLOCK). Verified findings: `docs/reviews/CLAUDE_verification-and-strategy.md` (all anchors at `main` @ `69a9c74`, which is current HEAD).
**Goal:** improve robustness, concurrency-correctness, and hardening of go-wallet-toolbox by implementing the quorum's locked start order.

---

## 1. Scope of this iteration

Implements Decision Record v1 "Implementation start order" items 1–4:

| Item | What |
|------|------|
| **W0** | Postgres test harness (env-switchable engine), concurrency test suite with red tests encoding the live bugs, storage invariant checker |
| **W1-1** | RowsAffected equality in `markReservedOutputsAsNotSpendable` → closes provided-input double-claim |
| **W1-2** | UnFail invalid-path propagates output-restore errors (rollback, like its 3 sibling call sites) |
| **W1-3** | AbortAction evidence gate: no abort when shared KnownTx shows broadcast/in-flight evidence |
| **W1-4** | Silent 0-row status updates → typed errors; expected-status `WHERE` (positive CAS) on Transaction status writers |
| **W1-5** | `UserUTXO.BasketName` gorm tag fix (`not null,index` → `not null;index`); scan all models for same class |
| **W1-6** | Status predicate split (`IsInFlight`/`IsTerminalFailure`/`IsAcceptedUnproven`/`IsFinalMined`); settle `reorg` semantics; fix `AlreadySent()` misuse at internalize |
| **W1-7** | `SendWaitingTransactions` returns results + aggregated error; `attempts` bumped only for txs actually posted |
| **W2-1** | ServiceError broadcast result ⇒ change **not** spendable (first HighAssurance fund-rule merge, P5) |
| **W3a-1** | Stable monitor distributed-lock key (per-task, not `time.Millisecond` wall-clock bucket) + stale `RUNNING` reclaim + skipped-run visibility |

**Out of scope (subsequent waves, per locked sequence):** W2-2 quarantine/CompetingEvidence, W2-3 reorg demote cascade, W3a-2 row leases, W3a-3 cursors, W3b outbox, W4 lifecycle consolidation, Track P benchmarks/Option B. **Locked non-goal:** any public detail or fix of the private-GHSA payment-integrity issue (handled only in the private advisory).

## 2. Locked policy constraints that shape the design

- **P3 HighAssurance default ON** — safe behavior is the default; opt-outs are explicit.
- **P4** — every input-release path is evidence-gated; AbortAction *is* an input-release path.
- **P5** — service error / thin success never yields freely spendable change.
- **P7** — monitor + services run on every instance; correctness comes from DB locks/leases, not process mutexes.
- Truthfulness rule: **0 rows updated ⇒ error, never silent success.**

## 3. Design per work item

### W0 — Postgres harness + concurrency suite + invariant checker

**Current state:** 100% of CI runs SQLite in-memory only (fixture pins `MaxOpenConnections=1`). A `TEST_DB_MODE=postgres` env switch already exists (`pkg/internal/testabilities/testmode`) and `docker-compose.yaml` already ships a `postgres:17-alpine` service — but the Postgres path is broken for real use: the fixture inherits `SslMode: "verify-full"` (fails against non-TLS Postgres), keeps the 1-connection pool pin (serializes any concurrency test), and gives no per-test isolation (all tests share one database). `FOR UPDATE SKIP LOCKED` paths are gated off SQLite, so none of the concurrency behavior is tested on the production engine.

**Design:**
- **Engine switch (extend, don't invent):** fix `dbfixtures.DBConfigForTests()` Postgres branch — `SslMode=disable` (or `TEST_DB_SSLMODE` override), pool widened (e.g. 10) for the Postgres branch only (SQLite keeps 1 to avoid the documented pool deadlock), unique per-test schema via `defs.PostgreSQL.Schema` (repo already auto-runs `CREATE SCHEMA IF NOT EXISTS` + `search_path`). Postgres tests **skip with a clear message** when `TEST_DB_MODE` is unset (Decision Record W0 exit criterion allows documented skip-if-no-pg).
- **Local provisioning:** reuse the existing compose `db` service (`postgres:17-alpine`, user/pass `postgres`, port 5432). Document `docker compose up -d db && TEST_DB_MODE=postgres go test ./pkg/...`.
- **CI:** all 19 workflow files are template-synced from `mrz1836/go-broadcast` ([Sync] commits rewrite them) — inline edits get clobbered. Add a **new repo-local workflow** (`.github/workflows/postgres-tests.yml`, name not in the template) with a `postgres:17-alpine` service container and `TEST_DB_MODE=postgres`, running the storage-related packages.
- **Concurrency suite** (new package under the storage tests): scenarios run N goroutines against one storage provider on a real DB:
  1. *Provided-outpoint double-claim:* N concurrent `CreateAction`s consuming the same user-provided outpoint ⇒ **exactly one success**. Red before W1-1, green after.
  2. *Same-basket funding contention:* N concurrent `CreateAction`s funding from one basket ⇒ no output is spent twice; satoshi conservation holds. (Spurious `ErrNotEnoughFunds` under contention is recorded as a known Track-P item, not asserted away.)
- **Invariant checker:** helper callable after any scenario, asserting over raw tables: no output `spendable = true` with `spent_by` set; no double-spent output (two claiming txs); no `completed` Transaction whose KnownTx lacks a merkle proof; per-user satoshi conservation across a scenario. Used by the suite; designed to be liftable into a `VerifyIntegrity` job later.

### W1-1 — Provided-input claim becomes CAS

**Current:** `markReservedOutputsAsNotSpendable` (repo layer) updates outputs with a `spent_by IS NULL`-style guard but ignores the 0-row outcome; user-provided non-change inputs (`KnownOutputIDs`) are claimed only through it. Concurrent CreateActions can both "claim" the same outpoint; the losing UPDATE silently no-ops. (`noSendChange` inputs are already protected by RowsAffected-checked `reserveUTXOs` — the fix mirrors that pattern.)

**Target:** the update requires `RowsAffected == len(outputIDs)`; on mismatch return the existing contention error type used by `reserveUTXOs` (`ErrUTXOContention` family) so the enclosing create UoW rolls back and the caller surfaces a retryable conflict. No schema change.

### W1-2 — UnFail restores are all-or-nothing

**Current:** the UnFail invalid-path calls `RecreateSpentOutputs` / `MarkCreatedOutputsAsNotSpendable` and logs-but-continues on error inside a committing UoW — unique among the 4 call sites; result can be a half-released fund state.

**Target:** propagate the error so the UoW rolls back, exactly like the abort / process / sync-statuses call sites. Behavior change is error-path only.

### W1-3 — AbortAction is evidence-gated (P4)

**Current:** `validateTxStatusForAbort` decides abortability from the local `Transaction.Status` alone (`unprocessed`/`unsigned`/`noSend`/`nonFinal`/`unfail` abortable). A Transaction can still be `unprocessed` while its **shared KnownTx** is already `sending`/broadcast (same class as #926, which fixed only the background AbortAbandoned sweep via `FindTransactionIDsForAbort`). Abort releases inputs of an in-flight tx.

**Target:** abort validation also loads the KnownTx (when one exists) and refuses abort when its status carries broadcast/in-flight or accepted evidence. As implemented (deliberate tightening vs the first draft, per P4): abortable ONLY when KnownTx is absent or ∈ {`unprocessed`, `nosend`, `nonfinal`, `unknown`}; every other status — including the terminal-failure states `invalidTx`/`doubleSpend` — refuses abort, because terminal cleanup (the sync review loop) owns that release path, not user abort. Typed error tells the caller why. Mirrors the #926 filter so both release paths share semantics.

### W1-4 — Status writers tell the truth

**Current:** `updateKnownTxStatus` (skip-list UPDATE) checks `.Error` but never `RowsAffected`; `UpdateTransactionStatusByTxID`/`ByID`/`UpdateTransaction` are unconditional writes with no expected-status precondition. Lost/raced updates are invisible.

**Target:**
- `updateKnownTxStatus` returns rows-affected knowledge: 0 rows ⇒ typed `ErrStatusTransitionSkipped`-style error (name follows repo error conventions). History notes are appended **only when the UPDATE matched** (today they're written even for 0-row updates). Callers audited: where a skip is legitimate (row deliberately excluded by the skip-list, e.g. already beyond broadcast stage), handle via `errors.Is` + debug log; otherwise propagate.
- Transaction status writers gain a positive expected-status `WHERE` (caller states the allowed current statuses) + RowsAffected check. `UpdateTransactionStatusByTxID` legitimately updates multiple rows (all users sharing a txID, potentially in different statuses) — its contract is "0 rows ⇒ typed error when the caller asserts existence", not strict equality. Call sites audited and updated per a table in the plan.
- Fold in the truthfulness bug found during verification: `InvalidateMerkleProofsByBlockHash` returns `affected, nil` after the gorm transaction, silently swallowing the transaction error (`known_tx.go:719`) — return the error.
- This is CAS-hardening only — no version column, no schema change (W4 owns versioning).

### W1-5 — Model tag fix

**Current:** `models/user_utxo.go` tags `BasketName` with `gorm:"not null,index"` — comma joins directives, so AutoMigrate creates **neither** NOT NULL nor the index, and the funder hot path filters `basket_name`/`reserved_by_id` unindexed.

**Target:** `gorm:"not null;index"`. Migration test asserts the index and NOT NULL exist post-AutoMigrate (both engines). Repo-wide scan for other comma-joined gorm tags; fix any found the same way. Composite-index tuning deferred to the W5 Postgres plan audit (locked).

### W1-6 — Predicate split; `reorg` semantics settled once

**Current:** `ProvenTxReqStatus.Sending()` counts terminal `invalidTx`/`doubleSpend` as sending; `reorg` maps to `waiting` in `ToStandardizedStatus()` yet `AlreadySent()`/`WasBroadcastStatus()` return true for it; `internalize.go` (~:422) treats a reorged tx as already-sent — the one *live* mis-predicate.

**Target:** add intent-named predicates — `IsInFlight`, `IsTerminalFailure`, `IsAcceptedUnproven`, `IsFinalMined` — and migrate every caller of `Sending()`/`AlreadySent()`/`WasBroadcastStatus()` to the predicate matching its intent (full caller inventory in the plan; `Sending()` has exactly one consumer, via `SendWithResultStatus()`). `reorg` semantics (aligned with P6): **was broadcast** = true — `reorg` MUST stay in `WasBroadcastStatus()`, because reorg re-sync eligibility rides on `was_broadcast` (`withReadyForStatusSyncFilter`); **already sent as network-acceptance evidence** = false — `internalize` and the broadcast already-sent branch must not treat a reorged tx as currently accepted; standardized status stays `waiting` for re-proof. Old predicates either delegate to the new ones with corrected membership or are removed if all callers migrate; exhaustive status-table test pins every predicate × status.

### W1-7 — SendWaiting honesty

**Current:** `SendWaitingTransactions` assembles per-tx results then returns `nil, nil`; broadcast failures are only logged. `attempts` is incremented for every tx picked up pre-send — including delayed / already-sent txs never posted this round — and for proof-fetch misses.

**Target:** return the assembled results plus an aggregated error (`errors.Join` of per-batch failures); monitor task logs them at the task layer. Side effect (intended): the currently-dead result-forwarding in the monitor's send-waiting task starts flowing `TxBroadcasted` events; duplicates with the background-broadcaster path are possible for retried txs — acceptable, events are best-effort fan-out (durable outbox is W3b). `attempts` bumped only for txs actually posted this round (the ready-to-send set, not already-sent ones, not delayed-queued ones); the proof-miss bump at the sync path stays (it is a proof attempt counter there). No schema change (`rebroadcast_attempts` already separate). The existing concurrent-calls test that asserts nil-error + exact attempts count is updated to the new contract.

### W2-1 — ServiceError never frees change (P5)

**Current:** on `AggregatedPostedTxIDServiceError` (all/most providers erroring — *no network evidence at all*), the result-apply path sets outputs `spendable = true` while statuses go `sending`, creating spendable change UTXOs in the same UoW.

**Target:** ServiceError ⇒ `spendable = false`; statuses still go `sending` (SendWaiting retries later). Change-UTXO spendability is granted only on real acceptance evidence (success path — unchanged today; the fuller multi-source acceptance policy is W2-2+). Per P3/P5 this is the unconditional default; no opt-out flag in this wave (`HighAssurance` config surface arrives with W2-2 quarantine, where opt-out semantics are meaningful). Recovery paths verified to exist: on later success both the `updateSingleTx` Success branch and the already-sent branch call `CreateUTXOForSpendableOutputsByTxID` (idempotent `OnConflict` upsert), so funds are delayed, not stranded — a test pins this. Implementation must also confirm balance/listOutputs read from `user_utxos` rows (not `outputs.spendable`, which stays true) so the deferral is actually effective.

### W3a-1 — Distributed cron lock actually locks (P7)

**Current:** `NewDaemonWithGORMLocker` configures `gocron-gorm-lock` with `WithDefaultJobIdentifier(time.Millisecond)` — the lock key is the acquisition-time wall clock truncated to 1ms, so two instances virtually never collide on a key: both INSERTs succeed, both run the job. Crashed-worker `RUNNING` rows are never cleaned (TTL sweep deletes only `FINISHED`).

**Target:** slot-truncated wall-clock keys are ALSO vacuous here — gocron duration jobs are not aligned across pods (each pod's schedule starts at its own `Start()` time), so two pods' fire times straddle bucket boundaries and both run, every interval. The correct shape (sanctioned by the record's "or lease-claim" alternative):
- **Custom lease-based locker** implementing the `gocron.Locker`/`gocron.Lock` interfaces in `pkg/monitor`: stable key = job name; acquire = atomic claim (`INSERT` new row, or CAS `UPDATE ... WHERE lease_until < now`) with `owner`, `lease_until = now + TTL`; release = mark finished/free. Two pods always contend on the same key; loser's run is skipped by gocron and logged at Warn with the task name ("skipped, lock held by other instance" — the visibility requirement).
- **Stale reclaim is inherent:** a crashed owner's row becomes claimable when `lease_until` passes. Lease TTL derived from each task's configured interval (≥ 2× interval, floor of a few minutes), passed to the locker at daemon `Start()` when task intervals are known.
- `NewDaemonWithGORMLocker` keeps its signature; internals swap from `gocron-gorm-lock` to the lease locker (that library offers no reclaim API and its `RUNNING` rows are immortal). Lock acquisition uses its own short-lived statements on the provided `*gorm.DB`, never inside app transactions.
- In-process TryLock mutexes stay (cheap local guard). Per-item row leases are W3a-2, not this change.

## 4. Error handling conventions

New typed errors follow the repo's existing error-definition pattern (same package/style as `ErrUTXOContention`). Every new guard distinguishes: (a) contention/CAS-miss — retryable conflict; (b) evidence-refusal (abort gate) — caller-visible rejection with reason; (c) invariant violation — internal error, UoW rollback. No new panics; no silent logs where an error can propagate.

## 5. Testing strategy

- TDD per item: failing test first (on both engines where behavior is engine-sensitive), then fix.
- W0 red tests are the acceptance tests for W1-1 (double-claim) and stay as regressions.
- Every W1 item gets unit tests at the repo/action layer via existing testabilities fixtures; W2-1 and W1-3/W1-6 get action-level tests (given/when/then fixture style already in the repo).
- Invariant checker runs at the end of each concurrency scenario.
- Full existing suite must stay green on SQLite; storage packages additionally green on Postgres locally (CI wiring included; skip-if-no-pg keeps forks green).

## 6. Compatibility & migration notes

- W1-4 changes some storage-method signatures/behavior (typed errors instead of silent success). Internal packages only; call sites updated in the same change.
- W1-5 alters generated schema (new index + NOT NULL on next AutoMigrate). NOT NULL on `basket_name`: existing rows could violate; AutoMigrate on SQLite/Postgres will add the constraint — verify against populated fixtures; if unsafe, index-only now and NOT NULL deferred with a data backfill note.
- W2-1 changes visible funding behavior under provider outages: change is unavailable until evidence instead of instantly spendable. This is the locked P5 policy; changelog entry required.
- W3a-1 changes lock-table row shape/semantics only in key content; no schema migration expected (same library tables).

## 7. Acceptance criteria (iteration-level)

1. Concurrency suite exists and runs on Postgres; provided-outpoint scenario proves exactly-one-success (red→green across W1-1).
2. Zero silent 0-row status writes remain in `repo` (grep-able: every status UPDATE checks RowsAffected or documents why not).
3. Abort of a Transaction whose KnownTx is in-flight is refused (test).
4. ServiceError path yields no spendable outputs (test) and later success restores spendability (test).
5. Two daemons + one job slot ⇒ exactly one execution; crashed owner reclaimed after TTL (test with two in-process daemons on one DB).
6. Entire pre-existing suite green (SQLite); storage suites green (Postgres).

## 8. Risks

- **Fortress workflow sync** may overwrite CI edits — mitigated by repo-local workflow file if markers found.
- **Callers relying on silent-skip semantics** of status writers (W1-4) — mitigated by full call-site audit in plan; legitimate-skip sites get explicit handling.
- **`reorg` semantic change** (W1-6) touches internalize behavior — pinned by tests; aligns with locked P6.
- **Lease locker** replaces `gocron-gorm-lock` semantics — deployments observing the old `cron_job_locks` table will see behavior change (per-slot rows stop growing). One-way door is small: gocron's `Locker` interface is the stable seam, and the old library can be restored by reverting the constructor internals.
- **Redundant first-run broadcast disappears:** today `StartImmediately` + vacuous lock means every pod fires SendWaiting on startup (de-facto retry). With a real lock only one pod runs it; this is the intended P7 behavior but worth a changelog note.
