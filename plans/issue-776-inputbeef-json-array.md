# InputBEEF wire format — JSON number array for TypeScript storage interop

**Issue:** [bsv-blockchain/go-wallet-toolbox#776](https://github.com/bsv-blockchain/go-wallet-toolbox/issues/776)
**Related open code PR:** [#931](https://github.com/bsv-blockchain/go-wallet-toolbox/pull/931) (`fix/beef-json-encoding`) — **preferred implementation path** (see below).
**Severity:** High for remote-storage users — Go clients talking to TypeScript storage servers (e.g. `storage.babbage.systems`) fail or mis-handle `CreateAction` when `inputBEEF` is present; SPV / proof attachment breaks downstream.
**Estimated size:** S

> **Plan-only PR note:** this document is the implementable checklist for #776. It does **not** close the issue (`Related to #776`, not `Fixes`). Do not land Go code on the plan branch.

### Related code PR #931 (do not re-implement if landable)

As of HEAD `cc19064` on `fix/beef-json-encoding`, **#931 already implements this plan’s Option A end-to-end**:

| Plan item | #931 status |
|-----------|-------------|
| `BEEF.MarshalJSON` → number array via `ExplicitByteArray` | Done |
| `BEEF.UnmarshalJSON` dual-accept (number array + legacy base64 + null) | Done |
| `OutPoint` tags `json:"txid"` / `json:"vout"` | Done |
| Remove obsolete `//nolint:musttag` on createAction in `v1adapter/handler.go` | Done |
| `pkg/wdk/primitives/beef_test.go` unit cases | Done |
| `pkg/wdk/storage_create_action_args_json_test.go` wire regression | Done |

**Default action for implementers:** review/merge **#931** (resolve any remaining review feedback; CI was green after the nolint/lint fix). Only open a **new** code PR if #931 is abandoned or diverges from this plan. This plan PR must stay markdown-only and must not duplicate #931’s code.

---

## Context for a fresh session

You are fixing a **JSON wire-format mismatch** between this Go toolbox and the TypeScript `wallet-toolbox` storage protocol.

When a Go service uses `storage.NewClient()` against a TS storage server and calls `CreateAction` with explicit inputs (so `InputBEEF` is populated), the request body currently serializes:

```json
"inputBEEF": "AAEC..."
```

(base64 string — Go's default `encoding/json` behavior for `[]byte`).

TS storage expects:

```json
"inputBEEF": [0, 1, 2, ...]
```

(an explicit array of integers 0–255). The TS side reads the field with something equivalent to `makeReader`, which requires `Uint8Array | number[]` and rejects base64 strings with errors like:

```text
bin must be Uint8Array or number[]
```

A second, tightly related wire bug often co-fires on the same path: nested `OutPoint` has **no JSON tags**, so it marshals as `{"TxID":"…","Vout":1}` while TS reads `outpoint.txid` / `outpoint.vout`. That produces follow-on failures such as:

```text
inputBEEF ... must be valid and contain proof data for possibly known undefined
```

Fix both on the same change if still broken on `main`; they share the createAction wire path and the same interop goal.

**Bug status on `origin/main` (verified at plan time):** still present. `primitives.BEEF` is a bare `[]byte` alias with no custom JSON methods; `OutPoint` has no tags.

---

## Files / anchors on current main

| File | What to look at |
|------|-----------------|
| `pkg/wdk/storage_create_action_args.go` | `ValidCreateActionArgs.InputBEEF` is `primitives.BEEF` with `json:"inputBEEF,omitempty"` (~L71). Nested `ValidCreateActionInput.Outpoint` is `OutPoint`. |
| `pkg/wdk/primitives/beef.go` | `type BEEF []byte` only — comment already says “array of integers 0–255”, but no `MarshalJSON`/`UnmarshalJSON`. |
| `pkg/wdk/primitives/explicit_byte_array.go` | Reference implementation: marshals to `[0..255]` number array; unmarshals number arrays (+ `null`). **Reuse this** rather than reimplementing digit packing. |
| `pkg/wdk/outpoint.go` | `OutPoint{TxID, Vout}` with **no** `json` tags → PascalCase keys on the wire. |
| `pkg/wdk/storage_create_action_result.go` | Result path already uses `ExplicitByteArray` for `inputBeef` (~L35) — evidence that number-array is the intentional WDK wire shape elsewhere. |
| `pkg/wdk/table_transaction.go`, `table_proven_tx_req.go` | Sync/table DTOs already use `ExplicitByteArray` for `inputBEEF`. |
| `pkg/storage/client.go` | `authriteRequester.post` does `json.Marshal(payload)` (~L314) — no custom codecs; fixing the types is sufficient for the HTTP client. |
| `pkg/storage/v1adapter/handler.go` | Server-side `json.Unmarshal` into `ValidCreateActionArgs` for createAction; may carry a `//nolint:musttag` that becomes obsolete once `OutPoint` gains tags. |

Call chain that hits the bug:

1. Wallet / app builds `CreateAction` with `InputBEEF` + explicit inputs.
2. Mapping produces `wdk.ValidCreateActionArgs`.
3. Remote client marshals args via standard `encoding/json`.
4. TS storage rejects base64 `inputBEEF` (and/or missing lowercase outpoint keys).

---

## Root cause

1. **`primitives.BEEF` is an unadorned `[]byte`.** Go's `encoding/json` always encodes `[]byte` as a base64 **string**. The WDK/TS protocol for binary blobs on this surface is an explicit JSON number array (same convention as `ExplicitByteArray`).
2. **Issue text suggests changing the field type** of `InputBEEF` to `ExplicitByteArray`. That works for this one field, but:
   - The type name `BEEF` and its doc comment already describe the TS-shaped value.
   - Only one call site uses `primitives.BEEF` today (`ValidCreateActionArgs.InputBEEF`), but fixing the **type** keeps future uses correct by default.
3. **`OutPoint` lacks `json` tags**, so nested outpoints in `inputs[].outpoint` and `options.noSendChange` leave the wire as PascalCase — incompatible with TS.

Not a storage business-logic bug: local in-process storage (same process, no JSON hop) is unaffected. Only the JSON RPC / remote client↔server boundary is wrong.

---

## Recommended approach

Prefer fixing the **type** so all JSON uses of `BEEF` are correct, and fix `OutPoint` tags in the same PR (required for real createAction interop).

### Option A (recommended) — custom JSON on `primitives.BEEF`

In `pkg/wdk/primitives/beef.go`:

```go
// BEEF is transaction data in BEEF (BRC-62) format.
// JSON wire format is an explicit [0..255] number array (not base64),
// matching TypeScript wallet-toolbox / storage servers.
type BEEF []byte

func (b BEEF) MarshalJSON() ([]byte, error) {
    return ExplicitByteArray(b).MarshalJSON()
}

// UnmarshalJSON accepts a number array (canonical) or a legacy base64 string
// (old Go []byte encoding) so older payloads still decode. null → nil.
func (b *BEEF) UnmarshalJSON(data []byte) error {
    // if data starts with '"': base64 path
    // else: reuse ExplicitByteArray.UnmarshalJSON for number array / null
}
```

Notes:

- **Marshal always emits a number array** (including empty → `[]`). With `omitempty` on the field, nil/empty slices are still omitted by `encoding/json` before/around marshal in the usual way for slice-backed types — verify with a table test.
- **Unmarshal dual-accept** (number array + base64) is intentional compatibility; do **not** re-emit base64 on marshal.
- Use a **value** receiver for `MarshalJSON` and a **pointer** receiver for `UnmarshalJSON` (same pattern / `recvcheck` rationale as `ExplicitByteArray`).

Do **not** need to change the field type on `ValidCreateActionArgs` under this option.

### Option B (issue text) — change field type only

In `storage_create_action_args.go`:

```go
// Before
InputBEEF primitives.BEEF `json:"inputBEEF,omitempty"`
// After
InputBEEF primitives.ExplicitByteArray `json:"inputBEEF,omitempty"`
```

Works, but leaves `BEEF` as a footgun and requires every assignment site to convert types if signatures differ. Prefer Option A unless there is a strong reason to delete `BEEF`.

### Also required: `OutPoint` JSON tags

In `pkg/wdk/outpoint.go`:

```go
type OutPoint struct {
    TxID string `json:"txid"`
    Vout uint32 `json:"vout"`
}
```

This is a **breaking change** for any Go consumer that previously marshaled/unmarshaled PascalCase `TxID`/`Vout`. That form never matched TS storage; lowercase matches the rest of the WDK wire types. Call it out in the PR description.

### Cleanup when tags land

If `pkg/storage/v1adapter/handler.go` still has:

```go
//nolint:musttag // wdk.OutPoint (nested) has no JSON tags …
```

on the createAction unmarshal, **remove** the obsolete nolint once tags exist — otherwise `nolintlint` fails CI.

---

## Reference behavior (TS / existing Go)

- TS: `inputBEEF` consumed as binary via number-array / `Uint8Array` readers (`makeReader`-style), not base64 strings.
- Go already does the right thing for **response** and **table** shapes via `ExplicitByteArray` (`StorageCreateActionResult.InputBeef`, `TableTransaction.InputBEEF`, etc.). Request-side `BEEF` is the outlier.

---

## Test strategy

### Unit — `pkg/wdk/primitives/beef_test.go` (new)

| Case | Assert |
|------|--------|
| Marshal non-empty | `BEEF{0,1,255}` → `"[0,1,255]"` (not base64) |
| Marshal empty | `BEEF{}` → `"[]"` |
| Round-trip number array | marshal → unmarshal equals original |
| Unmarshal base64 (legacy) | `"AAH/"` → `{0,1,255}` (if dual-accept adopted) |
| Unmarshal `null` | result `nil` |
| Unmarshal out-of-range | `[0,256]` → error |
| Unmarshal invalid base64 | error (if dual-accept adopted) |

### Unit — wire regression on full args

Add `pkg/wdk/storage_create_action_args_json_test.go` (or extend an existing JSON test):

1. Build a `ValidCreateActionArgs` with non-empty `InputBEEF` and one input outpoint.
2. `json.Marshal` and parse into `map[string]json.RawMessage`.
3. Assert `raw["inputBEEF"]` is exactly a number-array encoding (e.g. `"[0,1,255]"`), **not** a quoted base64 string.
4. Assert nested `inputs[0].outpoint` has keys `txid` / `vout` and **not** `TxID` / `Vout`.
5. Optional: marshal `OutPoint` alone and `JSONEq` against `{"txid":"…","vout":N}`.

### Integration / manual (optional but high signal)

Against a live TS storage server (or a recorded fixture):

1. Go `storage.NewClient(url, wallet)`.
2. `CreateAction` with explicit inputs + real `InputBEEF`.
3. Before fix: server error / ignored field / SPV failure.
4. After fix: createAction succeeds and returned beef verifies.

Local-only provider tests should remain green (no JSON hop).

### Commands

```bash
go test ./pkg/wdk/primitives/ ./pkg/wdk/ -count=1
golangci-lint run ./pkg/wdk/... ./pkg/storage/v1adapter/... --config .golangci.json
```

---

## Acceptance criteria

- [ ] `json.Marshal` of `ValidCreateActionArgs` with non-empty `InputBEEF` emits a JSON **number array** under `"inputBEEF"`, never a base64 string.
- [ ] `json.Unmarshal` accepts the number-array form into `InputBEEF` / `primitives.BEEF`.
- [ ] (Recommended) Unmarshal still accepts legacy base64 for backward compatibility with older Go-produced payloads.
- [ ] Nested `OutPoint` marshals/unmarshals with lowercase `txid` / `vout`.
- [ ] Unit tests above are green; existing `pkg/wdk` and storage client tests still pass.
- [ ] No obsolete `musttag` nolint left on createAction if tags made it unnecessary.
- [ ] Remote createAction against a TS storage server with explicit inputs + `inputBEEF` succeeds (manual or automated interop check).

---

## Risks, non-goals, dependencies

**Risks**

- **Breaking JSON for `OutPoint`:** any external Go client decoding PascalCase outpoints will break. Mitigate by documenting in the PR; the old shape was already wrong for the shared protocol.
- **`omitempty` + empty BEEF:** confirm nil vs empty slice behavior so clients do not unexpectedly send `"inputBEEF":[]` if the field should be omitted (mirror pre-fix omit behavior).
- **Dual-accept unmarshal:** accepting base64 forever can hide producer bugs; keep it as a deliberate compatibility note, or gate it behind a short deprecation window if the team prefers strictness.
- **PR #931 collision:** #931 already implements Option A + OutPoint tags + tests + nolint cleanup. Prefer landing that PR; do not open a second concurrent implementation. If #931 merges, mark #776 fixed with evidence and close this plan as done.

**Non-goals**

- Changing BEEF **binary** (BRC-62) layout or validation rules.
- Reworking storage createAction business logic, funding, or SPV verification beyond wire decode.
- Migrating every `[]byte` in the repo to number arrays — only WDK wire types that must match TS.
- Closing #776 from a plan-only PR (use `Related to #776`, not `Fixes #776`).
- Duplicating or superseding #931 while it is still open and landable.

**Dependencies**

- None beyond standard library `encoding/json` and existing `ExplicitByteArray`.
- **Coordinate with [#931](https://github.com/bsv-blockchain/go-wallet-toolbox/pull/931):** default is land/review #931. Only implement this plan on a fresh branch if #931 is closed without merge or is abandoned.

---

## Suggested implementation order

1. Confirm bug still on `origin/main` (re-read `beef.go` / `outpoint.go`).
2. **Check #931 first:** if open and complete (Option A + tags + tests) → review/merge it; stop re-implementing. If merged and green → mark #776 fixed with evidence.
3. Else (only if #931 is not landable) implement Option A on `primitives.BEEF` + OutPoint tags + remove obsolete nolint.
4. Add `beef_test.go` + full-args wire regression test (or confirm #931’s equivalents cover the acceptance criteria).
5. Run unit + lint commands above.
6. Optional live TS storage smoke test.
7. Open/land **code** PR with `Fixes #776` (implementation PR only — never this plan PR).

---

## Useful cross-references

- Issue: https://github.com/bsv-blockchain/go-wallet-toolbox/issues/776
- Open fix PR (related): https://github.com/bsv-blockchain/go-wallet-toolbox/pull/931
- Existing number-array helper: `pkg/wdk/primitives/explicit_byte_array.go`
- Already-correct response field: `StorageCreateActionResult.InputBeef` in `pkg/wdk/storage_create_action_result.go`
- Remote marshal site: `pkg/storage/client.go` (`authriteRequester.post`)
- Style reference for this plan doc: `plans/brc40-stale-chunk-guard.md`
