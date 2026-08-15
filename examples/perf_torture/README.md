# perf_torture — BEEF hot-path torture test & profiling harness

Replicates the production slowdown reported against the storage server: with
`change_basket.number_of_desired_utxos: 1`, every transaction respends the
single change UTXO, so between blocks the wallet builds an ever-deeper chain of
unmined transactions. Every `createAction` then rebuilds the full unmined
ancestry BEEF, producing the flame graph dominated by
`transaction.(*Beef).MergeTransaction` recursion with
`ComputeRoot`/`GetOffsetLeaf`/`Sha256` towers.

The harness has three parts:

| Part | What it does |
|------|--------------|
| `perf_torture.go` | Mainnet end-to-end run: N createActions (0-sat `OP_FALSE OP_RETURN deadbeef`) through Arcade, tracked to MINED, with pprof exposed |
| `profile.sh` | Captures a 30s CPU profile from the running process and diffs it against the previous capture |
| `beefbench/` + `bench.sh` | Pure in-memory benchmarks of the same hot paths — the fast inner loop for optimization work |

## Configuration

- Network: **mainnet** (`infra.Defaults()`)
- Broadcast: Arcade `https://arcade-v2-us-1.bsvblockchain.tech` (override with `-arcade`)
- Torture condition: `change_basket.number_of_desired_utxos = 1`
- Storage: local SQLite (`storage.sqlite` in this directory) with the monitor
  daemon in-process (`check_for_proofs` every 1 min attaches merkle proofs →
  MINED)
- Keys: auto-generated into `perf-torture-config.yaml` on first run
  (git-ignored — it holds private keys)

## Running

```bash
# 1. First run prints a funding address and exits
go run ./examples/perf_torture

# 2. Send mainnet funds (a few thousand sats) to the printed address, then:
go run ./examples/perf_torture -txid <funding-txid>

# Later runs (balance already in storage.sqlite):
go run ./examples/perf_torture
```

Flags: `-n 100` (tx count), `-interval 0` (pacing between createActions),
`-mined-timeout 4h`, `-poll 15s`, `-pprof localhost:6060`, `-arcade <url>`.

Every run writes `results-<runlabel>.csv` with per-tx create latency, broadcast
status, and seconds-to-MINED, and logs a summary (p50/p95 create latency and
time-to-mined, failed/aborted counts).

## Capturing the flame graph

While the run is active:

```bash
./examples/perf_torture/profile.sh 30
# or interactively:
go tool pprof -http=:8080 "http://localhost:6060/debug/pprof/profile?seconds=30"
```

`profile.sh` stores each capture under `profiles/` and prints a
`-diff_base` report against the previous capture.

## The optimization iteration loop

The mainnet run takes hours (block-paced). The inner loop is the benchmark
suite, which reproduces the same code paths in memory:

```bash
./examples/perf_torture/bench.sh          # runs + benchstat vs previous run
```

Iterate:

1. `./bench.sh` → baseline saved to `benchresults/`
2. Apply an optimization (toolbox code, or a `replace` on
   `github.com/bsv-blockchain/go-sdk` for beef.go/merklepath.go changes)
3. `./bench.sh` → benchstat shows the delta with confidence intervals
4. Re-run the mainnet torture test + `profile.sh` to confirm the flame graph
   improvement end-to-end

## Known hot paths (from code analysis of go-sdk v1.3.3 + toolbox)

Per-createAction ancestry rebuild — O(D) per action, O(D²) cumulative while
the chain stays unmined:

- `pkg/internal/storage/repo/known_tx_get_beef.go` — `recursiveBuildValidBEEF`
  rebuilds the full unmined ancestry from scratch on every call (no caching
  between calls); invoked per createAction via `mergeAllocatedUTXOs`
  (`pkg/storage/internal/actions/create.go`).
- go-sdk `transaction/beef.go:741` — `MergeTransactionWithTxid` recurses over
  all `SourceTransaction` ancestors with no visited-set and removes/re-adds
  txs already present.
- go-sdk `transaction/transaction.go:265` — `TxID()` is uncached: every call
  re-serializes the whole tx + double-SHA256. Called per ancestor per merge,
  and again per tx in `Beef.Bytes()`.
- go-sdk `transaction/merklepath.go:219` — `ComputeRoot` rebuilds its indexed
  path and recomputes everything on every call; `MergeBump` + `Combine` run up
  to 4 root computations per same-height bump pair.
- go-sdk `transaction/merklepath.go:35` — `GetOffsetLeaf` recursively rebuilds
  interior merkle nodes (sha256d each) and memoizes nothing; `Combine` drops
  parent nodes, making combined bumps progressively more expensive.

Candidate fixes, roughly by impact: cache/persist built ancestry BEEFs keyed
by txid (invalidate when a proof arrives); add an already-present fast path +
visited-set to `MergeTransaction`; cache `TxID()` on `Transaction`; memoize
`ComputeRoot` per bump and `GetOffsetLeaf` results.
