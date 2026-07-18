# KnownTx notify opaque payload — persist through sync upsert/read

**Issue:** [bsv-blockchain/go-wallet-toolbox#851](https://github.com/bsv-blockchain/go-wallet-toolbox/issues/851)
**Severity:** Low — single-device impact minimal; tracked for TS↔Go sync completeness.
**Estimated size:** S

> **Plan only** — this document describes the intended fix. No application code lands in this PR.

---

## Context for a fresh session

You are picking up a sync correctness bug in the Go storage layer. `ProvenTxReq.notify` is an opaque string (JSON blob) that TypeScript wallet clients use to track which DB transaction IDs to notify when merkle proofs arrive. The wire type already exposes it (`wdk.TableProvenTxReq.Notify`), but the Go persistence path never stores or returns it.

On `origin/main` today:

1. `mapModelToTableProvenTxReqForSync` hardcodes `Notify: "{}"` with a TODO that the field is "only used by JS-version … so we can ignore it for now".
2. `models.KnownTx` and `entity.KnownTx` have no `Notify` field.
3. `chunk_processor.upsertProvenTxReqs` never copies `chunkProvenTxReq.Notify` into the entity.

Any `ProvenTxReq` that round-trips through a Go storage server therefore loses its notify payload — an RPC-observable difference vs the TypeScript reference impl.

A prior code fix PR ([#937](https://github.com/bsv-blockchain/go-wallet-toolbox/pull/937) on `fix/851-known-tx-notify`) implemented this and was closed unmerged so the issue can land as **plan-first**. That branch is a useful reference draft (not authoritative until re-reviewed against current `main`).

---

## Problem summary + file anchors on current main

| Location | Symptom |
|----------|---------|
| `pkg/internal/storage/repo/syncrepo/sync_knowntx.go` ~L232 | `Notify: "{}"` hardcoded in `mapModelToTableProvenTxReqForSync` |
| `pkg/internal/storage/database/models/known_tx.go` | No `Notify` column on `KnownTx` |
| `pkg/entity/known_tx.go` | No `Notify` field on entity |
| `pkg/storage/internal/sync/chunk_processor.go` ~L216–228 (`upsertProvenTxReqs`) | Incoming `chunkProvenTxReq.Notify` not mapped into `entity.KnownTx` |
| `pkg/wdk/table_proven_tx_req.go` ~L23 | Wire type already has `Notify string \`json:"notify"\`` — no change needed |
| `pkg/storage/internal/actions/process.go` ~L267 | Separate TODO: `// TODO: Add db transactionID to KnownTx.Notify` — **out of scope** (this issue is opaque storage/round-trip only) |

Hardcoded read path (current main):

```go
// pkg/internal/storage/repo/syncrepo/sync_knowntx.go — mapModelToTableProvenTxReqForSync
Notify: "{}", // TODO: Notify includes transaction IDs and they are only used by JS-version of the wallet, so we can ignore it for now
```

---

## Root cause

Go intentionally discarded the notify blob under the assumption that only the JS wallet cares about it. That is wrong for a multi-storage / cross-impl sync path:

- TS client A writes `ProvenTxReq` with a non-empty `notify` into storage.
- Go server is the remote storage (or an intermediate hop).
- On `getSyncChunk` / `processSyncChunk`, Go emits / re-ingests `notify: "{}"`.
- Client loses the bookkeeping needed to fire proof notifications.

There is no schema column to hold the value, and both the upsert model-builder and the chunk ingest mapper drop it.

---

## Recommended fix

Treat `notify` as an **opaque string**. Do not parse, validate, or interpret it in Go. Persist and return it unchanged. Empty → default `"{}"` for backward compatibility with the previous hardcoded value and with TS empty-object defaults.

### 1. Model column

`pkg/internal/storage/database/models/known_tx.go` — add:

```go
// Notify is an opaque JSON blob used by the TypeScript wallet to track
// which transactions to notify when proofs arrive. Stored and returned
// unchanged for sync round-trip compatibility.
Notify string `gorm:"type:text;default:'{}'"`
```

Column is created via the existing GORM AutoMigrate path (same pattern as `WasBroadcast` / `rebroadcast_attempts`). No hand-written SQL migration.

### 2. Entity field

`pkg/entity/known_tx.go` — add:

```go
// Notify is an opaque payload (typically JSON) from ProvenTxReq.notify.
// Persisted and returned unchanged for sync round-trip; no Go-side semantics.
Notify string
```

### 3. Upsert path

`pkg/internal/storage/repo/syncrepo/sync_knowntx.go` — `UpsertKnownTxForSync`:

```go
notify := entity.Notify
if notify == "" {
    notify = "{}"
}
// ... include Notify: notify in models.KnownTx{...}
```

Normalize empty → `"{}"` **before** building the model so GORM `Updates` includes the field (zero-value string would otherwise be skipped). The existing BRC-40 stale-chunk guard (`updated_at` strict greater-than) continues to gate whether the UPDATE applies; notify is just another mutable field under that guard.

### 4. Read / getSyncChunk path

Same file — `mapModelToTableProvenTxReqForSync`:

```go
notify := model.Notify
if notify == "" {
    notify = "{}"
}
// ... Notify: notify  (replace the hardcoded "{}")
```

### 5. Chunk ingest

`pkg/storage/internal/sync/chunk_processor.go` — `upsertProvenTxReqs`:

```go
isNew, err := p.repo.UpsertKnownTxForSync(p.ctx, &pkgentity.KnownTx{
    // ...existing fields...
    Notified:            chunkProvenTxReq.Notified,
    Notify:              chunkProvenTxReq.Notify, // ADD
    RawTx:               chunkProvenTxReq.RawTx,
    // ...
})
```

`upsertProvenTx` (mined path) does **not** need notify — `TableProvenTx` has no such field.

### Optional / non-blocking

- **genquery regeneration:** same pattern as `WasBroadcast` / `RebroadcastAttempts` — the sync path uses GORM `Select("*")` / struct Create/Updates, so typed genquery fields are not required for this fix. Optional follow-up: `go generate` under `gen_gorm` if typed query filters on notify are ever needed.
- **Do not** implement `process.go` TODO (populate notify from local processAction). That invents Go-side notify semantics and is a separate feature.

---

## Files to change (implementation PR)

| File | Change |
|------|--------|
| `pkg/internal/storage/database/models/known_tx.go` | Add `Notify string` column tag |
| `pkg/entity/known_tx.go` | Add `Notify string` field |
| `pkg/internal/storage/repo/syncrepo/sync_knowntx.go` | Persist on upsert; return on map-to-table; empty→`"{}"` |
| `pkg/storage/internal/sync/chunk_processor.go` | Pass `chunkProvenTxReq.Notify` into entity |
| `pkg/internal/storage/repo/syncrepo/sync_knowntx_notify_test.go` | **New** unit tests (see below) |

No changes to `pkg/wdk/table_proven_tx_req.go` (wire type already correct).

---

## Test strategy

Add `pkg/internal/storage/repo/syncrepo/sync_knowntx_notify_test.go`:

1. **UpsertPersistsOpaquePayload** — seed via `UpsertKnownTxForSync` with a non-trivial JSON notify blob; `DB.First` asserts column equals the payload unchanged.
2. **UpsertDefaultEmptyToObject** — omit/empty Notify; assert stored value is `"{}"`.
3. **FindForSyncRoundTrip** — create user + linking `Transaction` (so `FindKnownTxsForSync` EXISTS filter matches) + KnownTx with notify; assert `FindKnownTxsForSync` returns the same payload on the `TableProvenTxReq` (unmined → reqs, not proven).
4. **UpdateReplacesPayload** — upsert at `T` with payload A, upsert at `T+1h` with payload B; assert column is B and other mutable fields updated. Use `require.JSONEq` for JSON string compares (testifylint `encoded-compare`).

Use 64-hex-char txids (Postgres `varchar(64)` PK). Keep tests under the existing `dbfixtures.TestDatabase` helper.

Verification commands:

```bash
go test ./pkg/internal/storage/repo/syncrepo/ -run TestKnownTxNotify -count=1
go test ./pkg/internal/storage/repo/syncrepo/ -count=1
```

Optional: `go test ./pkg/storage/ -run Sync -count=1` if a higher-level processSyncChunk fixture with notify is added later.

---

## Acceptance criteria

- [ ] `models.KnownTx` has a `notify` text column (default `'{}'`), created via AutoMigrate.
- [ ] `entity.KnownTx` carries `Notify` for the sync layer.
- [ ] `UpsertKnownTxForSync` persists incoming `entity.Notify` (empty → `"{}"`).
- [ ] `FindKnownTxsForSync` / `mapModelToTableProvenTxReqForSync` returns the stored value (empty → `"{}"`), never a hardcoded constant.
- [ ] `upsertProvenTxReqs` forwards `chunkProvenTxReq.Notify`.
- [ ] Opaque payloads round-trip byte-for-byte (modulo empty→`"{}"` normalization).
- [ ] Unit tests cover persist, empty default, find round-trip, and newer-`updated_at` replace.
- [ ] Existing `syncrepo` tests still pass; no regression of BRC-40 stale-chunk guard behaviour.
- [ ] Implementation PR body uses `Fixes #851` (this plan PR uses `Related to #851` only).

---

## Risks, non-goals, dependencies

### Risks

- **GORM Updates zero-skip:** empty string must be normalized to `"{}"` before Updates, or a deliberate clear-to-empty would be dropped. Normalization matches historical behaviour.
- **Column default portability:** `gorm:"type:text;default:'{}'"` should work on SQLite and Postgres used by this repo; confirm AutoMigrate on both if CI matrix covers them.
- **Stale-chunk interaction:** notify only updates when incoming `updated_at` is strictly newer — correct and consistent with other mutable KnownTx fields. Do not special-case notify outside the BRC-40 guard.

### Non-goals

- Interpreting or rewriting notify contents (no JSON schema, no remapping of transaction IDs across storages).
- Implementing `process.go` TODO to *populate* notify on local processAction.
- Batch field support (`Batch: nil` TODO remains).
- Merkle index on ProvenTx (`Index: 0` TODO remains).
- genquery regeneration (optional).

### Dependencies

- None. Self-contained schema + mapping change.
- Prior closed draft: `fix/851-known-tx-notify` / PR #937 (lint-fixed with `require.JSONEq`) may be rebased as a starting point after this plan is approved.

### Related issues / PRs

- #852 — provenTx/provenTxReq idMap on processSyncChunk (adjacent sync completeness).
- #853 / `plans/brc40-stale-chunk-guard.md` — `updated_at` guard already present on `UpsertKnownTxForSync`; notify must live inside that guard, not bypass it.
- Closed code PR #937 — do not revive as the landing vehicle; land implementation from this plan in a separate `fix/851-…` PR after review.

---

## Useful cross-references

- Wire type: `pkg/wdk/table_proven_tx_req.go` (`Notify string \`json:"notify"\``).
- Upsert + map: `pkg/internal/storage/repo/syncrepo/sync_knowntx.go` (`UpsertKnownTxForSync`, `mapModelToTableProvenTxReqForSync`).
- Chunk ingest: `pkg/storage/internal/sync/chunk_processor.go` (`upsertProvenTxReqs`).
- Local process TODO (out of scope): `pkg/storage/internal/actions/process.go` ~L267.
- History note name used elsewhere for proofs: `pkg/internal/storage/history` `NotifyTxOfProof` — related conceptually, not part of this storage column.
- TS reference: wallet-toolbox `ProvenTxReq` / entity merge paths treat `notify` as a stored string field on the proven-tx-req row.

---

## Notes / gotchas

- Issue text mentions "~line 199"; on current main the hardcoded `Notify: "{}"` is in `mapModelToTableProvenTxReqForSync` (~L232). The *upsert* path never had a Notify assignment because the model field was missing entirely — both ends must be fixed.
- Mined known txs are exported as `TableProvenTx` (no notify field). Only the unmined / req path carries notify. Tests must leave MerklePath empty so the row maps to ProvenTxReq.
- `FindKnownTxsForSync` filters via EXISTS on user transactions — round-trip tests need a linking `Transaction` row with the same `tx_id`.
- Do not use `require.Equal` on JSON strings in tests; use `require.JSONEq` where semantic JSON equality is intended (testifylint).
