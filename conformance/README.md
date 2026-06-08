# Cross-impl conformance vectors

Vendored copies of BRC conformance vectors maintained in
[`bsv-blockchain/ts-stack`](https://github.com/bsv-blockchain/ts-stack)
under `conformance/vectors/`. Each vector subdir ships a small Go package
exposing the JSON via `go:embed` (e.g. `conformance/vectors/sync/sync.go`).
Test runners import that package — no filesystem paths, no `runtime.Caller`,
no sibling-repo checkout. Tests stay hermetic, work identically for every
cloner.

## Why vendor?

- **Determinism** — every commit in this repo pins the exact vector revision
  the code passes against.
- **Offline / CI hermeticity** — no GitHub fetch during `go test`.
- **Diffability** — vector changes show up in PR review.

## Refreshing

Use the helper script:

```sh
./conformance/scripts/refresh-vectors.sh           # pull latest from ts-stack main
./conformance/scripts/refresh-vectors.sh <sha>     # pin to specific upstream commit
```

The script:

1. Resolves the upstream commit SHA (default: `main` tip).
2. Downloads each tracked vector from `raw.githubusercontent.com`.
3. Updates `conformance/SOURCE` with the pinned SHA + timestamp.

Re-run the test suite after refresh (BRC-40 is in the syncrepo package; BRC-100 and storage adapter have their own suites):

```sh
go test ./pkg/internal/storage/repo/syncrepo/... -run TestBRC40Conformance -v
go test ./pkg/storage/... -run AdapterConformance -v
go test ./pkg/wallet/... -run BRC100Conformance -v
```

If new vectors land that the Go impl doesn't satisfy, fix the impl or open an
issue tagged with the failing vector ID.

## Tracked vector files

| Path                                               | Upstream                                                                 |
|----------------------------------------------------|--------------------------------------------------------------------------|
| `vectors/sync/brc40-user-state.json`               | `conformance/vectors/sync/brc40-user-state.json` in `ts-stack`           |
| `vectors/wallet/storage/adapter-conformance.json`  | `conformance/vectors/wallet/storage/adapter-conformance.json` (the `/storage/v1/*` remoting contract) |
| `vectors/wallet/brc100/*.json` (selected core files) | `conformance/vectors/wallet/brc100/*.json` (BRC-100 wallet method vectors) |

Run the BRC-100 and storage adapter conformance suites with:

```sh
go test ./pkg/storage/... -run AdapterConformance -v
go test ./pkg/wallet/... -run BRC100Conformance -v
```
