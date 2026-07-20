# Tasks — Option A schema conformance (orchestration playbook)

> **For the orchestrator (you):** This is not 16 separate PRs. It is **one project, one branch, run in a single sitting** as a sequence of waves. Within a wave, dispatch subagents in parallel by **file ownership**. Between waves, run the automated **gate** and proceed without human halt unless a gate fails and you cannot auto-fix it. Phase numbers from the original plan are preserved in the **Phase→Wave map** at the bottom for spec/design traceability.
>
> **Source of truth for every column/type/PK/index:** `target-schema.md`. If prose anywhere disagrees with `target-schema.md`, `target-schema.md` wins.

**Goal:** Make Go's storage schema byte-for-byte conformant with `ts-stack/wallet-toolbox` @ `7a840ff97e1f685f778210818933e6da0dac22c2`, so storage dumps round-trip and conformance fixtures are shared.

**Architecture:** GORM models → generated genquery (gorm.io/gen) → repos → sync repos → actions/funder → provider → wdk DTOs / v1adapter. The model package is a hard build-barrier; the fan-out lives downstream of it.

**Tech stack:** Go 1.26.3, GORM (+gorm.io/gen), magex task runner, golangci-lint, sqlite/postgres.

---

## READ FIRST — schema-reality corrections (validated against the pinned TS source)

The original prose plan diverged from the actual TS schema. These are corrected here and in `target-schema.md`; the spec/design/proposal have been updated to match.

1. **`created_at`/`updated_at` stay snake_case** (everything else camelCase). "camelCase every column" would emit `createdAt`/`updatedAt` and break conformance.
2. **Every table's PK renames to a per-table camelCase name** (`transactionId`, `outputId`, `certificateId`, `commissionId`, `syncStateId`, …) — not just baskets/tags/labels/users. `gorm.Model`'s generic `id` is non-conformant everywhere.
3. **`gorm.Model` injects `deleted_at`; TS has none.** Drop `gorm.Model` in favor of an explicit named PK + `Timestamps` embed (see `target-schema.md`). Soft delete = `isDeleted` boolean on exactly six tables (`certificates`, `output_baskets`, `output_tags`, `output_tags_map`, `tx_labels`, `tx_labels_map`).
4. **`sync_states.when` and `sync_states.satoshis` are KEPT** — they exist in TS at the pin. **Decision #2 (drop them) is overturned** (it was based on a wrong premise).
5. **Value-column renames:** `tags.name`→`output_tags.tag`, `labels.name`→`tx_labels.label`.
6. **History infra already exists** — `pkg/internal/storage/history` (`Builder`, event tags), `repo/tx_notes.go` (`addTxNote`/`addTxNotes`→`models.TxNote`), `wdk.HistoryNote`, `entity.TxHistoryNote`. Phase 5 = **redirect** these into the `history` JSON column, not a greenfield build.
7. **Perf gate deferred** — no benchmark harness exists. **Decision #5 (hard bench gate) is overturned.** Gate on build+tests+conformance+lint; apply the design's indexes; file a perf-bench follow-up. (See Wave 4.)
8. **Struct-rename collision hazard:** `Tag`→`OutputTag` while the old join `OutputTag`→`OutputTagsMap`; `Label`→`TxLabel` while `TransactionLabel`→`TxLabelsMap`. One agent must own all four atomically (Wave 1, agent M2).
9. **`numeric_id_lookup` has more call sites than the prose implied** — also `sync_output.go` (resolves basket numeric id via `CONCAT(user_id,'.',basket_name)`) and `sync_transaction.go`. All are basket/tag/label/known_tx-derived, so Phase-3 + Phase-6 surrogates cover them — but Wave 3 must prove zero consumers by grep before deleting.

---

## Execution model

- **One branch:** `feat/schema-conformance-optiona` (current). One commit per wave (or per agent within a wave); no inter-wave PRs.
- **Gates, not reviews:** each wave ends with an automated gate (commands below). Proceed automatically while green. On red: the wave's reconciler agent (or you) fixes, re-runs the gate, then proceeds. Never weaken/skip a test to pass a gate.
- **Parallelism by file ownership:** the file-ownership matrix per wave is authoritative. **No two agents in the same wave edit the same file.** Shared files (`migrator.go`, `gen_gorm/gorm_gen.go`, `models/base.go`) have exactly one owner per wave.
- **No worktrees needed:** waves are sequential, agents within a wave own disjoint files → they share the one working tree safely. (Reserve `isolation: worktree` only if you choose to run two waves concurrently, which this plan does not require.)

### Subagent contract (give every dispatched agent)

- **Inputs:** the pinned commit; `target-schema.md`; your exact owned-file list; the transformation rules for your area; the verification command(s); your Definition of Done.
- **Rules:** edit ONLY your owned files; for any cross-file column name/type/PK, read it from `target-schema.md` (don't invent); use TDD where you change behavior; if blocked by another package's not-yet-changed surface, **report the signature you need** rather than guessing or editing outside your files.
- **Return (structured):** files changed; every cross-package signature you changed (`symbol: old → new`); commands you ran + results; remaining failures + root cause; any deviation from `target-schema.md` + why.

### Gates (exact commands)

```bash
# G-models  (Wave 1)
go build ./pkg/internal/storage/database/models/...
go generate ./pkg/internal/storage/database/...        # re-runs gorm.io/gen
go build ./pkg/internal/storage/database/genquery/...

# G-build   (Wave 2c, 3, 4)
go build ./...

# G-test
magex test:short -timeout 20m        # fallback: go test ./...

# G-lint
magex lint && go vet ./...           # config: .golangci-lint.yml

# G-conf    (Wave 3, 4)
go test ./pkg/internal/storage/repo/syncrepo/... -run TestBRC40Conformance -v
go test ./pkg/storage/... -run AdapterConformance -v
go test ./pkg/wallet/... -run BRC100Conformance -v
```

### Dependency DAG

```
Wave 0  pin + target-schema contract + vector reconcile        ── blocking
   │
Wave 1  models → final shape + genquery regen        ── barrier: G-models
   │
Wave 2a  repo ∥ syncrepo ∥ entity/history (data layer)
   │
Wave 2b  actions/funder ∥ provider/wdk ∥ monitor (app layer)
   │
Wave 2c  integrate                                   ── barrier: G-build, G-test, G-lint
   │
Wave 3  drop numeric_id_lookup + conformance         ── barrier: G-conf, G-test
   │
Wave 4  docs + indexes + final pin + archive-ready
```

---

## Wave 0 — Foundation (blocking; you + 1 agent)

**Goal:** lock the contract everything codes to. No model code yet.

- [x] `ts-pin.md` is pinned to `7a840ff97e1f685f778210818933e6da0dac22c2` (done). Confirm the local checkout `/Users/personal/git/ts-stack` is at that commit.
- [x] **Finalize `target-schema.md`:** ✅ All ⚠ RESOLVE items confirmed against sorted-key migration order. Corrections: (1) `wasBroadcast`+`rebroadcastAttempts` are **PRESENT** (added by last migration, not dropped); (2) `activeStorage` is **PRESENT** (add+alter, never dropped — original "net-absent" was wrong); (3) `derivationPrefix`/`Suffix` final size is **200** (not 32); (4) `description`/`outputDescription` final size is **2048**. All spec/design/proposal/tasks files updated.
- [x] **Reconcile conformance vectors:** ✅ Ran `refresh-vectors.sh 7a840ff9...`. All 11 tracked vector files downloaded. `conformance/SOURCE` updated to new pin (sha=`7a840ff9...`, fetched_at=2026-07-08). Diffs are formatting/indentation only (JSON whitespace normalization + field reordering) — no semantic content changes. No schema-scope changes pulled in.
- [x] (Optional, cheap) emit a `schema-diff` note: ✅ Created `schema-diff.md` with per-table Go-current vs target column deltas, organized by Wave 1 agent ownership (M-base, M1, M2).
- **Gate:** `target-schema.md` has zero ⚠ RESOLVE left; vectors committed. **No bench** (deferred — Wave 4 files the follow-up).
- *Covers original Phase 0 (minus baseline bench).*

## Wave 1 — Models to final shape + genquery regen (barrier)

**Goal:** `pkg/internal/storage/database/models/` reaches the exact `target-schema.md` shape and genquery regenerates. Whole-repo build is expected to break here; that's fine — the gate is G-models only.

**Sequencing inside the wave:** (1) agent M-base writes `models/base.go` and commits; (2) agents M1 + M2 run in parallel on disjoint files; (3) coordinator updates the gen list, regenerates, runs G-models, commits.

**File ownership:**

| Agent | Owns (in `models/`) | Work |
|-------|---------------------|------|
| **M-base** | `base.go` (new) | ✅ Created `Timestamps` embed to match TS `addTimeStamps()`. |
| **M1 (core)** | `user.go`, `output.go`, `transaction.go`, `output_baskets.go`, `certificate.go`, `commission.go`, `settings.go`, `sync_state.go`, `key_value.go`, `chaintracks_*.go`; **new** `proven_tx.go`; refactor `known_tx.go`→`proven_tx_req.go` | ✅ Applied all exact model schemas: named PKs, camelCase, `isDeleted`, new tables, etc. |
| **M2 (tag/label/maps + new + drops)** | `tags.go`→`output_tags.go`, `labels.go`→`tx_labels.go`, `output_tags.go`→`output_tags_map.go`, `tx_labels.go`→`tx_labels_map.go`; **new** `monitor_events.go`; **delete** `user_utxo.go`, `tx_note.go`, `numeric_id_lookup.go` | ✅ Renamed lookup/map structs & tables, implemented drops, built `monitor_events.go`. |
| **coordinator** | `gen_gorm/gorm_gen.go` only | ✅ Updated `ApplyBasic(...)` list, cleared stale `.gen.go` files, ran `go generate`, verified `G-models` build. |

- **Gate: G-models.** (Repo/consumer breakage expected and deferred to Wave 2.)
- *Covers original Phases 1, 2, 3 (model parts), 4, 5 (cols only), 6 (models only), 8 (model), 9, 10 (corrected: keep when/satoshis), 11 (model drop).*

## Wave 2 — Consumer fan-out

### Wave 2a — Data layer (parallel: 3 agents)

| Agent | Owns | Work |
|-------|------|------|
| **C-repo** | `repo/*.go` except sync — `outputs.go`, `transactions.go`, `output_baskets.go`, `certificates.go`, `commission.go`, `settings.go`, `sync_state.go`, `key_value.go`, `utxos.go`, `all.go`, `gen_cmp_condition.go`, `cached_basket_maker.go`; rename `known_tx.go`→`proven_tx_req.go` + `known_tx_get_beef.go`→`proven_tx_req_get_beef.go`; **delete** `user_utxo.go`, `tx_notes.go`; `migrator.go` `AutoMigrate(...)` list | ✅ Rewrote UTXO selection, rewired transactions joins, updated all generated queries and migrator list. |
| **C-sync** | `repo/syncrepo/*.go` | ✅ Rewired `numeric_id_lookup` to native surrogate IDs, split `known_tx` sync, updated all queries to genquery. |
| **C-entity** | `pkg/entity/*`, `pkg/internal/storage/entity/*`, `pkg/internal/storage/history/*`, plus the `addTxNote(s)` redirect | ✅ Split domain entities, implemented `history` JSON serialization, redirected `addTxNotes`. |

- **Gate (soft):** `go build ./pkg/internal/storage/...` advances as far as cross-package surfaces allow; each agent returns the signatures it changed. Hand drift to 2c. ✅ (Done. C-repo drift logged for C-actions).
- *Covers original Phases 3 (sync rewire), 5 (consumer), 6 (sync/repo), 11 (UTXO rewrite).*

### Wave 2b — App layer (parallel: 3 agents)

| Agent | Owns | Work |
|-------|------|------|
| **C-actions** | `pkg/storage/internal/actions/*`, `pkg/internal/storage/funder/*` | Update `create.go`/`internalize.go`/`list_*.go`/`create_process_inputs.go` and `funder/sql.go`+`pool.go` to the new repo signatures + spendable-based selection. |
| **C-provider** | `pkg/storage/provider.go`, `pkg/storage/rpcserver/*`, `pkg/storage/v1adapter/*`, `pkg/wdk/*` | Update provider methods to new repo/entity surfaces; **wdk DTO field names** are hand-written Go structs + JSON tags (NOT proto) — align JSON tags to TS field names; update v1adapter; rpcserver is deprecated (minimal touch). Bump RPC/DTO schema version (major). |
| **C-monitor** | monitor task package (locate: `grep -rl "monitor" pkg/ --include=*.go` for the task lifecycle) | Emit `monitor_events` rows at tracked lifecycle steps, aligned to TS monitor semantics; the promote-to-proven path INSERTs `proven_txs` and UPDATEs `proven_tx_reqs.provenTxId`+`status`. |

- *Covers original Phases 6 (provider/monitor), 8 (emission), 13 (RPC/wdk).*

### Wave 2c — Integration (1 reconciler)

- [ ] `go build ./...` green — reconcile any cross-package signature drift the 2a/2b agents reported.
- [ ] `magex test:short` — triage; fix compile/behavior regressions from the schema change. Genuinely conformance-shaped failures → note for Wave 3.
- [ ] `magex lint && go vet ./...`.
- **Gate: G-build, G-test, G-lint.** Commit `wave2: consumers conformant to TS schema`.

## Wave 3 — Cross-cutting correctness + conformance (barrier)

| Agent | Work |
|-------|------|
| **drop-numeric-id** | Prove zero consumers: `grep -rn "NumericIDLookup\|numeric_id_lookup\|upsertNumericIDLookup\|joinWithNumericIDLookupScope\|findNumericIDLookup\|saveNumericIDLookup"` returns nothing in non-deleted code. Then delete `models/numeric_id_lookup.go`, `genquery/numeric_id_lookups.gen.go`, `syncrepo/numeric_id.go`; remove from `gorm_gen.go` + `migrator.go`; `go generate`; G-build. *(Phase 7)* |
| **conformance** | Point/refresh `conformance/` at the pin; run all three G-conf suites; fix deltas. Add vectors for `proven_txs`/`proven_tx_reqs` lifecycle, `monitor_events` emission, and the `history` JSON shape. *(Phase 12)* |
| **history-roundtrip** | Round-trip a TS-produced `history` blob through Go: parse → struct-equal → re-serialize byte-identical (key-order normalized). Confirms the flattened-extras shape. |

- **Gate: G-conf + G-test green.** Commit `wave3: numeric_id dropped, conformance passing`.

## Wave 4 — Finalize

- [ ] Docs: update `docs/wallet.md`; CHANGELOG breaking-schema note (pre-Option-A storage incompatible; RPC field names changed); update any diagram referencing `known_tx`. *(Phase 14)*
- [ ] Verify indexes exist: `outputs (userId, spendable, basketId)`, FK indexes on `transactions.provenTxId`, `proven_tx_reqs.txid`, `proven_txs.txid`, `output_tags_map.outputId`, `tx_labels_map.transactionId`.
- [ ] **File the deferred perf follow-up** (issue): baseline + re-bench UTXO selection and `ListTransactions` join at 10/50/100 tps once a harness exists. *(replaces Phase 0 baseline + Phase 15 re-bench)*
- [ ] Final `ts-pin.md` + `conformance/SOURCE` refresh; re-run G-conf. *(Phase 15 minus bench)*
- [ ] Storage-dump round-trip smoke (Go → TS load) if feasible in-session; else note in the conformance vectors.
- [ ] Archive readiness: confirm waves merged; **do not auto-run `openspec archive`** — leave for author. *(Phase 16)*
- **Gate: G-build, G-test, G-lint, G-conf all green.**

---

## Phase → Wave traceability

| Orig phase | Wave | Notes |
|-----------|------|-------|
| 0 Audit/baseline | 0 | baseline bench dropped (deferred); adds target-schema + vector reconcile |
| 1 camelCase | 1 (M1/M2) | corrected: `created_at`/`updated_at` stay snake_case |
| 2 isDeleted | 1 (M1/M2) | corrected: also strip `gorm.Model` `deleted_at` everywhere; 6 tables get `isDeleted` |
| 3 surrogate IDs | 1 models + 2a C-sync | corrected: ALL tables get named PKs, not just 4 |
| 4 renames | 1 (M2) | + value-col renames `tag`/`label`; struct-collision handled atomically |
| 5 tx_notes fold | 1 (cols) + 2a C-entity | reuse existing history infra; flattened-extras JSON |
| 6 known_tx split | 1 models + 2a/2b | provider join + monitor promote in 2b |
| 7 drop numeric_id | 3 | grep-proven zero consumers first |
| 8 monitor_events | 1 (model) + 2b (emission) | |
| 9 column adds | 1 | |
| 10 column removals | 1 | **corrected: keep `sync_states.when`/`satoshis`**; **corrected: keep `activeStorage`** (present in TS) |
| 11 drop user_utxo | 1 (model) + 2a (UTXO rewrite) | partial index added |
| 12 conformance | 3 | |
| 13 RPC layer | 2b C-provider | corrected: hand-written wdk structs + JSON, not proto |
| 14 docs | 4 | |
| 15 post-validation | 3 (conf) + 4 (pin) | perf re-bench deferred to follow-up |
| 16 archive | 4 | author runs archive |

## Self-review checklist (run before declaring done)

- [ ] Every shared table in `target-schema.md` matches Go's regenerated migration column-for-column (incl. snake_case timestamps, named PKs, no `deleted_at`).
- [ ] `grep -rn "gorm.Model"` in `models/` returns nothing (replaced by `Timestamps`+named PK).
- [ ] No `numeric_id_lookup` / `user_utxo` / `tx_note` symbols remain.
- [ ] `history`/`notify` JSON round-trips byte-identically against a TS fixture.
- [ ] All three conformance suites pass at the pinned commit.
- [ ] `sync_states.when`/`satoshis` present; `users.activeStorage` present; `proven_tx_reqs.wasBroadcast`/`rebroadcastAttempts` present.
