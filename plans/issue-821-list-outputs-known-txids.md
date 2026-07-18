# listOutputs `knownTxids` — BEEF optimization completeness

**Issue:** [bsv-blockchain/go-wallet-toolbox#821](https://github.com/bsv-blockchain/go-wallet-toolbox/issues/821)
**Severity:** Low–Medium — performance / response-size only (not a correctness bug when `includeTransactions` is true).
**Prior code PR (closed, plan-only policy):** [#940](https://github.com/bsv-blockchain/go-wallet-toolbox/pull/940) on `fix/821-list-outputs-known-txids` — draft still useful for the regression test (remote tip `b76259e` at time of plan).
**Verified against `main`:** `b7e05a2` (2026-07-18) — re-probe anchors before implementing if main has moved.

---

## Context for a fresh session

You are finishing a storage-layer parity item for `listOutputs`. The WDK/RPC surface already accepts `knownTxids` and validates each entry as a TXID hex string. When `includeTransactions: true`, the TypeScript wallet-toolbox uses that list so the returned BEEF can stub txs the caller already holds (`mergeTxidOnly`), shrinking the payload.

On current `main`, Go **already wires** `args.KnownTxids` into `GetBEEFForTxIDs` via `entity.WithKnownTxIDs`, and the recursive BEEF builder already short-circuits known IDs with `MergeTxidOnly`. What remains is:

1. An obsolete `// TODO: Handle args.KnownTxids` that makes the feature look unfinished.
2. **No listOutputs-level regression test** proving the optimization (only lower-level `GetBeef` / `CreateAction` known-tx tests exist).
3. Wallet SDK mapping cannot forward `knownTxids` yet because `sdk.ListOutputsArgs` has no field (storage/RPC clients already can).

**Issue body is stale:** #821 claims “Go ignores `knownTxids` and always returns full BEEF.” That was historically true; as of the verification SHA above, storage **does** forward known IDs. Closing #821 is “delete misleading TODO + lock with listOutputs regression test,” not green-field BEEF work. Do not re-implement the recursive builder; reuse the path that already works for createAction and getBeef.

---

## Problem summary + file anchors (current `main`)

| Location | What it shows |
|----------|----------------|
| `pkg/storage/internal/actions/list_outputs.go` ~L43–44 | Stale TODOs: `KnownTxids` (#821) **and** `IncludeLabels` (#820). Labels path is also already implemented (~L84–101 `labelMap`); only remove the KnownTxids TODO in the #821 PR |
| same file ~L110–118 | When `IncludeTransactions`, calls `knownTxRepo.GetBEEFForTxIDs(..., entity.WithKnownTxIDs(args.KnownTxids...), …)` |
| `pkg/wdk/storage_outputs_args.go` ~L21 | `KnownTxids []string` is part of the public storage args (JSON `knownTxids`) |
| `pkg/internal/validate/validate_list_outputs_args.go` ~L35–38 | Each known txid is validated as `TXIDHexString` |
| `pkg/internal/storage/entity/get_beef_options.go` ~L44–72 | `WithKnownTxIDs` / `IsKnownTxID` set membership |
| `pkg/internal/storage/repo/known_tx_get_beef.go` ~L79–85 | Known ID → `mergeToBeef.MergeTxidOnly(h)` and **return** (no ancestor recursion) |
| `pkg/storage/provider_list_outputs_test.go` | Has include-transactions coverage; **no** `KnownTxids` case on `main` |
| `pkg/wallet/internal/mapping/mapping_list_outputs.go` ~L21–49 | Does **not** map known txids (SDK type has no field) |
| `pkg/storage/server_test.go` ~L496–501 | RPC round-trip fixture already sends `KnownTxids` in list args |

**RPC-observable gap claimed by the issue:** “Go ignores `knownTxids` and always returns full BEEF.”  
**Read of current main:** storage **does** pass known IDs into BEEF construction. The gap is lack of an end-to-end listOutputs assertion and a misleading TODO. A fix PR should prove the optimization with a test; if a test ever fails, then re-investigate wiring — do not assume the issue body is still accurate without re-probing.

---

## Root cause

Historical incomplete implementation left a TODO at the top of `ListOutputs`. The BEEF path was later completed (`WithKnownTxIDs` on `GetBEEFForTxIDs`) without removing the TODO or adding a listOutputs regression test. From a code-review / issue-triage perspective the feature still looks unfinished; from a runtime perspective storage RPC clients that pass `knownTxids` with `includeTransactions: true` should already get TxIDOnly stubs for those IDs.

---

## Reference TypeScript semantics

TS `listOutputs` (wallet-toolbox storage methods) when building the reply BEEF for `includeTransactions`:

- Caller-supplied `knownTxids` are treated as already-held by the client.
- For each such txid encountered while assembling BEEF, emit **txid-only** (no raw tx / no merkle path / no further parent walk through that edge).
- Transactions the caller does **not** list remain fully embedded so the client can validate and use them.

Go already mirrors that in `recursiveBuildValidBEEF`:

```go
if options.IsKnownTxID(txID) {
    h, err := chainhash.NewHashFromHex(txID)
    // ...
    mergeToBeef.MergeTxidOnly(h)
    return nil
}
```

Same pattern is proven for createAction (`TestCreateActionWithKnownTxIDs`) and getBeef (`TestGetBeef_KnownTxIDsWithDuplicates_OnInputs_MergesParentAsTxIDOnly`). listOutputs should share that behaviour without a second implementation.

---

## Recommended fix

### 1. Remove the obsolete TODO (storage action)

In `pkg/storage/internal/actions/list_outputs.go`, delete:

```go
// TODO: Handle args.KnownTxids
```

Keep the existing `entity.WithKnownTxIDs(args.KnownTxids...)` call unchanged unless a regression test reveals a real bug.

**Adjacent TODO:** `// TODO: Handle args.IncludeLabels` is **out of scope for #821** — tracked by #820. Prefer leaving that line alone in the #821 fix PR so the two issues do not thrash the same hunk; if #820 is already merged by then, the line may already be gone.

Optional clarity (small comment above the BEEF block, not required):

```go
// KnownTxids are forwarded so GetBEEF can MergeTxidOnly for txs the caller already has.
```

### 2. Add listOutputs regression coverage

In `pkg/storage/provider_list_outputs_test.go`, add `TestListOutputs_KnownTxids` (draft on closed branch `fix/821-list-outputs-known-txids`, tip `b76259e` — still on remote at plan time).

**Imports to add** (file is `package storage_test`):

```go
"github.com/bsv-blockchain/go-sdk/transaction"
pkgtestabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
```

(`to` is already imported via other tests in some storage files; if missing here, also add `"github.com/go-softwarelab/common/pkg/to"`.)

**Shape / assertions**

1. Fund + process: `_, signedTx := given.Action(activeStorage).Processed()` (same harness as `TestListOutputs_IncludeTransactions`).
2. **Baseline:** `ListOutputs` with `IncludeTransactions: true`, empty/nil `KnownTxids` → full BEEF (today: **3** transactions — parent with BUMP, internalized, created).
3. **Optimized:** same call with `KnownTxids` = direct input parent txids of `signedTx`.
4. Assert:
   - `len(optimized.BEEF) < len(full.BEEF)`
   - each known parent is `transaction.TxIDOnly` via `pkgtestabilities.AssertBEEFState`
   - `len(optimizedBeef.Transactions) < len(fullBeef.Transactions)` (ancestors only reachable via known parents drop out)
   - newly created (unknown) tx remains fully embedded (not TxIDOnly)

**Ready-to-paste draft** (from #940; re-run and adjust counts if the fixture changes):

```go
// TestListOutputs_KnownTxids verifies that knownTxids optimizes returned BEEF by
// representing already-known transactions as TxIDOnly (matching TS listOutputs behavior).
func TestListOutputs_KnownTxids(t *testing.T) {
	// given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	_, signedTx := given.Action(activeStorage).Processed()
	signedTxID := signedTx.TxID().String()

	// baseline: full BEEF without knownTxids embeds parent + grandparent + created tx
	fullResult, err := activeStorage.ListOutputs(ctx, testusers.Alice.AuthID(), wdk.ListOutputsArgs{
		Limit:               100,
		IncludeTransactions: true,
	})
	require.NoError(t, err)
	require.NotNil(t, fullResult.BEEF)
	fullBeef := testutils.BEEFFromBytes(t, fullResult.BEEF)
	require.Len(t, fullBeef.Transactions, 3)

	// knownTxids = direct input parents of the created tx (same optimization surface as TS)
	knownTxids := make([]string, 0, len(signedTx.Inputs))
	for _, in := range signedTx.Inputs {
		require.NotNil(t, in.SourceTXID)
		knownTxids = append(knownTxids, in.SourceTXID.String())
	}
	require.NotEmpty(t, knownTxids)

	// when: listOutputs with knownTxids
	result, err := activeStorage.ListOutputs(ctx, testusers.Alice.AuthID(), wdk.ListOutputsArgs{
		Limit:               100,
		IncludeTransactions: true,
		KnownTxids:          knownTxids,
	})

	// then:
	require.NoError(t, err)
	require.NotNil(t, result.BEEF)
	assert.Less(t, len(result.BEEF), len(fullResult.BEEF),
		"knownTxids should reduce BEEF payload size by omitting embedded raw txs / proofs")

	optimizedBeef := testutils.BEEFFromBytes(t, result.BEEF)

	// known parents appear as TxIDOnly stubs (mergeTxidOnly), matching TS listOutputs
	for _, known := range knownTxids {
		pkgtestabilities.AssertBEEFState(t, result.BEEF, pkgtestabilities.ExpectedBeefTransactionState{
			ID:         known,
			DataFormat: to.Ptr(transaction.TxIDOnly),
		})
	}

	// further ancestors that were only reachable via known parents are dropped entirely
	assert.Less(t, len(optimizedBeef.Transactions), len(fullBeef.Transactions),
		"knownTxids should stop recursion so unused ancestors are excluded from BEEF")

	// the caller's unknown (newly created) tx remains fully embedded (not TxIDOnly)
	require.NotNil(t, optimizedBeef.FindTransaction(signedTxID))
	for hash, beefTx := range optimizedBeef.Transactions {
		if hash.String() == signedTxID {
			assert.NotEqual(t, transaction.TxIDOnly, beefTx.DataFormat,
				"created tx must keep full transaction data for the caller")
		}
	}
}
```

### 3. No production BEEF algorithm changes unless tests fail

If `TestListOutputs_KnownTxids` fails on current main, debug in this order:

1. Confirm `args.KnownTxids` is non-empty at the `GetBEEFForTxIDs` call.
2. Confirm `WithKnownTxIDs` populates `KnownTxIDsSet` (hex case / normalization — TXIDs must match the form stored/compared in `IsKnownTxID`).
3. Confirm known IDs are **parents in the BEEF graph**, not only unrelated wallet history (stubs only help when those IDs would otherwise be walked).

Do not invent a parallel “filter after serialize” path; fix the option wiring if needed.

---

## Files to touch (implementation PR — not this plan PR)

| File | Change |
|------|--------|
| `pkg/storage/internal/actions/list_outputs.go` | Remove KnownTxids TODO; leave BEEF call as-is |
| `pkg/storage/provider_list_outputs_test.go` | Add `TestListOutputs_KnownTxids` (+ imports: `transaction`, `pkgtestabilities`) |

**Not required for #821:**

| File | Why |
|------|-----|
| `pkg/internal/storage/repo/known_tx_get_beef.go` | Already correct |
| `pkg/wdk/storage_outputs_args.go` / validate | Already present |
| `pkg/wallet/internal/mapping/mapping_list_outputs.go` | Blocked on go-sdk `ListOutputsArgs` field |
| JSON-RPC codegen | Server test already exercises args shape |

---

## Acceptance criteria

- [ ] `// TODO: Handle args.KnownTxids` removed from `list_outputs.go`.
- [ ] With `includeTransactions: true` and non-empty `knownTxids`, listOutputs BEEF marks those txids as `TxIDOnly` and is strictly smaller than the no-knownTxids baseline for the standard Processed-action fixture.
- [ ] Unknown (caller-not-listed) txs in the same BEEF remain fully embedded.
- [ ] Empty / omitted `knownTxids` preserves today’s full-BEEF behaviour (existing `TestListOutputs_IncludeTransactions` still green).
- [ ] Invalid known txid hex still rejected by `validate.ListOutputsArgs` (existing validate tests).
- [ ] `go test ./pkg/storage/ -run 'TestListOutputs_' -count=1` passes.
- [ ] No drive-by changes to IncludeLabels (#820), wallet SDK mapping, or BEEF recursion limits.

---

## Test strategy

### Unit / integration (Go)

| Test | File | Asserts |
|------|------|---------|
| `TestListOutputs_KnownTxids` (**new**) | `provider_list_outputs_test.go` | Size shrink, TxIDOnly parents, full created tx, fewer BEEF graph nodes |
| `TestListOutputs_IncludeTransactions` (existing) | same | Full BEEF when known list empty |
| `TestCreateActionWithKnownTxIDs` (existing) | `provider_create_action_test.go` | Shared `WithKnownTxIDs` → TxIDOnly on input beef |
| `TestGetBeef_*KnownTxIDs*` (existing) | `provider_get_beef_test.go` | Builder short-circuit + duplicate known IDs |

### Manual / RPC smoke (optional)

Storage server client with `includeTransactions: true` and a known parent txid from a prior list/create; compare `len(BEEF)` with and without `knownTxids`.

### Conformance

No dedicated BRC-100 vector currently pins listOutputs knownTxids size. If ts-stack later adds one, wire it under the wallet/storage conformance runner — not a blocker for the Go fix PR.

---

## Risks, non-goals, dependencies

**Risks**

- **Hex normalization:** `HexString.Validate` accepts `[0-9a-fA-F]` (mixed case) but `IsKnownTxID` is an exact map lookup with **no** `ToLower`. Storage/fixtures use `chainhash`/`.String()` lowercase. Uppercase client `knownTxids` would validate yet miss the set — same behaviour as createAction/getBeef today; do **not** “fix” case folding in the #821 PR unless product asks for it (would be a separate, cross-API change). Prefer documenting / matching existing createAction tests.
- **Knowing the wrong set:** listing unrelated wallet txids does nothing useful; only txs that would appear in the BEEF walk matter.
- **Coupling to #820:** both TODOs at L43–44 are stale; labels are already wired. Still keep #821 to KnownTxids only so review stays focused.

**Non-goals**

- Exposing `knownTxids` on `sdk.ListOutputsArgs` / wallet `MapListOutputsArgs` (needs go-sdk change).
- Changing default wallet `WithAutoKnownTxids` behaviour for listOutputs.
- Implementing IncludeLabels (issue #820).
- Altering BEEF binary version or proof selection beyond known-txid stubs.

**Dependencies**

- None blocking. Closed draft PR #940 is a convenient patch reference only.
- go-sdk field for wallet-layer parity is a **follow-up**, not part of closing #821 for storage RPC.

**Estimated size:** **S** (TODO cleanup + one focused integration test; production logic already present).

**Suggested implementation commit message:**

```text
fix: listOutputs knownTxids BEEF optimization (#821)

Remove stale KnownTxids TODO (wiring already present) and add
TestListOutputs_KnownTxids locking TxIDOnly + smaller BEEF payload.
```

Implementation PR body should use `Fixes #821` (this plan PR uses `Related to #821` only).

---

## Useful cross-references

- Issue: <https://github.com/bsv-blockchain/go-wallet-toolbox/issues/821>
- Closed implementation draft: <https://github.com/bsv-blockchain/go-wallet-toolbox/pull/940> (`fix/821-list-outputs-known-txids` @ `b76259e`)
- Sibling labels work: #820 (`includeLabels` — also mostly “stale TODO + tests” once re-probed)
- CreateAction known-txids precedent: `pkg/storage/provider_create_action_test.go` `TestCreateActionWithKnownTxIDs`
- BEEF options: `pkg/internal/storage/entity/get_beef_options.go`
- Recursive short-circuit: `pkg/internal/storage/repo/known_tx_get_beef.go` (`IsKnownTxID` → `MergeTxidOnly`)
- Plan style sibling: `plans/brc40-stale-chunk-guard.md`

---

## Notes / gotchas

- `KnownTxids` only affects the response when `IncludeTransactions` is true; with transactions omitted there is no BEEF to optimize (validation of the array still runs).
- `uniqueTxTDsForAllOutputs` still requests BEEF for **all** output-producing txids; known-set filtering happens **inside** recursive build, not by dropping roots from that list. That matches “stub known ancestors / parents,” not “omit entire output txs from the graph if listed as known.” If a root output txid itself appears in `knownTxids`, it becomes TxIDOnly and its ancestors are not walked — same as getBeef.
- Prior cleanup closed code PR #940 because bug-track delivery for this repo wave is **plan-first**; the next code PR should be small and test-locked, linking `Fixes #821` only on the implementation PR (this plan PR uses `Related to #821` only).
