# Option B — RPC-only conformance, ultra-performance storage

**Status:** Proposed
**Branch:** `feat/storage-optionb-ultra-perf`
**Author:** Deggen
**Date:** 2026-05-21

## Why

`go-wallet-toolbox` was built to mirror `ts-stack/wallet-toolbox` table-for-table, then layered BRC-40 sync on top so a wallet's state could replicate across storage backends. That coupling caps throughput at single-Postgres-node ceilings (~5-10K writes/s, hot-read latency rising with history length) and bakes in joins on every list call.

A class of operators want a Go wallet storage that:
- Scales linearly from a Raspberry Pi single-tenant deploy (10 tps target) to a sharded cluster (100K tps ceiling) without rewriting application code.
- Costs cents/month at 10 tps.
- Doesn't pay the price of BRC-40 sync compatibility when they don't use sync.
- Only conforms to the BRC-100 wallet JSON-RPC contract — internal state shape is an implementation detail.

This change rebuilds the storage layer from the ground up to those constraints. Internal schema is unrecognizable to `ts-stack/wallet-toolbox`. BRC-40 sync is removed. The only conformance bar is the BRC-100 JSON-RPC interface and a wallet behavioural test suite.

## What changes

### Conformance scope

- **In scope**: BRC-100 wallet JSON-RPC method contract (createAction, signAction, internalizeAction, listActions, listOutputs, listTransactions, listCertificates, acquireCertificate, proveCertificate, relinquishCertificate, abortAction, revealCounterpartyKeyLinkage, revealSpecificKeyLinkage, getPublicKey, encrypt, decrypt, createHmac, verifyHmac, createSignature, verifySignature, getHeight, getHeaderForHeight, getNetwork, getVersion, waitForAuthentication, isAuthenticated, discoverByIdentityKey, discoverByAttributes).
- **Out of scope**: BRC-40 sync. No `findUserById`, no chunked sync exports, no merge logic.
- **Out of scope**: Internal schema portability with TS. No conformance against `ts-stack/wallet-toolbox` storage shape.
- **Out of scope**: Direct SQL access. External tools that read storage tables are not supported; consumers use RPC.

### Storage backend abstraction

Define a `Storage` Go interface that exposes domain operations, not tables. Concrete implementations:

| Backend | Target deploy size | Backend tech |
|---------|--------------------|--------------|
| `embedded` | Single tenant, 1–100 tps | BadgerDB (pure Go, LSM on local SSD) |
| `single-node` | 100–5K tps, low-cost ops | BadgerDB + optional ClickHouse for historical queries |
| `cluster-hot` | 5K–50K tps | Aerospike (primary KV) + Redpanda (event log) + S3 (blobs) |
| `cluster-extreme` | 50K–100K tps | Aerospike (sharded) + Redpanda (sharded) + ClickHouse (replicated) + S3 |

All four backends implement the same `Storage` interface. Application code is backend-agnostic. Config picks the driver at startup.

### Data model — KV-shaped, not relational

Every entity is keyed by a deterministic byte key. No joins. No surrogate IDs. No foreign keys.

```
user:<identityKey>                                  → User
user:<identityKey>:settings                         → UserSettings
user:<identityKey>:basket:<basketName>              → Basket
user:<identityKey>:utxo-set                         → SortedSet of OutpointRefs by satoshi-range bucket
user:<identityKey>:utxo:<txid>:<vout>               → Output
user:<identityKey>:tx:<txid>                        → Transaction (status snapshot)
user:<identityKey>:tx-by-ref:<reference>            → txid pointer
user:<identityKey>:action-log:<seqno>               → Action event (append-only)
user:<identityKey>:label:<labelName>:<txid>         → label map entry
user:<identityKey>:tag:<tagName>:<outpoint>         → tag map entry
user:<identityKey>:cert:<certifier>:<type>:<serial> → Certificate
tx-proof:<txid>                                     → ProvenTx (global, deduped across users)
tx-pursuit:<txid>                                   → PursuitState (global, monitor-owned)
header:<height>                                     → BlockHeader
header-by-hash:<hash>                               → height pointer
chaintip                                            → current tip header ref
```

All per-user keys share the `user:<identityKey>:` prefix. Aerospike partitions on the prefix; a user's entire state lives on one node (one shard). No cross-shard transactions on the hot path.

### Hot-path operations

#### createAction (write path)

1. RPC enters worker pool. Load `user:<identityKey>` (single KV get, sub-ms).
2. Coin-select from `user:<identityKey>:utxo-set` using pre-bucketed sorted set. O(log n) on bucket count, not on UTXO count.
3. Build tx in-memory. Sign via cached derived keys.
4. Append `action-log:<seqno>` event with full tx + selected inputs. Single KV put.
5. Return BEEF to caller.
6. **Async, post-response:** materializer goroutine consumes `action-log`, updates `utxo-set` (mark spent inputs, add new outputs), updates `tx`, updates labels/tags. Reply has already returned to caller.

Latency target: <2ms p50, <10ms p99 at 100K tps per node.

#### listOutputs (read path)

1. Load `user:<identityKey>:basket:<basketName>` (single KV get).
2. Stream cursor over `user:<identityKey>:utxo:` prefix filtered by basket. Paginate via KV scan.
3. Return.

Latency target: <5ms p99 for 100-entry pages, regardless of history length.

#### listActions (read path)

1. Stream cursor over `user:<identityKey>:action-log:` prefix in reverse-seqno order. Filter by status / labels in-memory per page.
2. Return.

For deep history scans (rare): hand off to ClickHouse replica that materializes action-log into a columnar store. Background ingester keeps it within seconds of fresh.

#### Broadcast pursuit + proof tracking

- Pursuit is a global concern, not per-user. `tx-pursuit:<txid>` is a single KV entry per txid, mutated by the monitor goroutine. References (transaction IDs that care) are stored as a set within the entry. No per-user duplication.
- When proof arrives: write `tx-proof:<txid>` (immutable proof facts). Fan-out notifications to each user with a pending reference.

### Concurrency model

- Worker pool per shard. RPC calls land on the worker for the requested user (consistent hash on identityKey).
- Single-writer per user → no per-user locks. All user state mutations serialized through the owning worker.
- Cross-user state (global pursuit, headers, settings) uses optimistic CAS via Aerospike's per-record version (generation) field.
- Event log (Redpanda) gives durable ordering across all shards for replay/audit. Worker writes are mirrored to event log async; backpressure signal halts new RPC if log lag exceeds budget.

### Read scalability

- KV reads scale horizontally with shard count. 50K reads/s/node × 10 nodes = 500K reads/s ceiling on Aerospike alone.
- Historical-range queries (listActions older than 7 days, listTransactions across years) go to ClickHouse replica. ClickHouse handles billions of rows per node on aggregate queries.
- Hot working set (active wallets last 24h) stays in Aerospike RAM. Cold data evicts to SSD or to S3 + ClickHouse.

### Blob storage

- BEEF blobs, raw tx bytes, merkle paths over ~4KB: store in S3-compatible object store (R2, GCS, MinIO for self-host).
- KV records hold blob references + ETag.
- Small blobs (<4KB) inline in KV record for one-RTT fetch.

### Configuration

Config file selects backend:

```yaml
storage:
  backend: embedded | single-node | cluster-hot | cluster-extreme
  embedded:
    path: ./data
  aerospike:
    seeds: ["aero-1:3000", "aero-2:3000"]
    namespace: wallet
  redpanda:
    brokers: ["kafka-1:9092"]
    topic: wallet-action-log
  clickhouse:
    url: clickhouse://...
  blobs:
    s3:
      bucket: wallet-blobs
      region: us-east-1
```

The Go binary is one artifact. The backend choice is runtime config. A user on a $5 VPS runs `embedded` with one process. An exchange runs `cluster-extreme` with the same binary.

### Migration from existing Go schema

None. Existing data is discarded. No migration path is provided. Operators wishing to retain existing storage stay on Option A (or fork pre-Option-B Go).

### RPC compatibility

The BRC-100 JSON-RPC wire format does not change. Wallet clients (BRC-100 SDKs, MetaNet apps) continue to work without modification. Only the storage backend implementation is replaced.

## Impact

### Affected capabilities

- `storage-rpc` (primary spec, see delta)
- `storage-schema` (deleted; no longer the conformance bar)
- `storage-sync` (deleted; BRC-40 sync removed)
- `wallet-actions` (rewired to KV ops)
- `chaintracks-storage` (kept in storage; switches to KV-shaped headers)

### Affected code surfaces

Effectively a rewrite of `pkg/storage/`, `pkg/internal/storage/`, `pkg/wallet/` data-access paths. Estimated 60-70% of LOC under `pkg/` rewritten or deleted.

### Deletions

- `pkg/internal/storage/database/` — all gorm models + genquery output
- `pkg/internal/storage/repo/syncrepo/` — BRC-40 sync implementation
- `pkg/internal/storage/repo/migrator.go` — replaced by per-backend init
- Any code path that depends on SQL transactions

### Additions

- `pkg/storage/backend/` — `Storage` interface + 4 driver packages (`embedded`, `singlenode`, `clusterhot`, `clusterextreme`)
- `pkg/storage/keys/` — key-builder helpers (deterministic byte keys)
- `pkg/storage/eventlog/` — Redpanda producer/consumer
- `pkg/storage/blobs/` — S3-compatible blob store driver
- `pkg/storage/materializer/` — async state materializer goroutines
- `pkg/storage/coinpick/` — bucketed UTXO selection
- `pkg/storage/clickhouse/` — historical query driver

### Performance targets

| Backend | tps | p50 latency | p99 latency | Storage cost/month for 1M users |
|---------|-----|-------------|-------------|----------------------------------|
| `embedded` | 100 | <5ms | <50ms | $5 (VPS) |
| `single-node` | 5K | <3ms | <30ms | $50 (1 EC2 + S3) |
| `cluster-hot` | 50K | <2ms | <10ms | $1K (3-node Aerospike + Redpanda + S3) |
| `cluster-extreme` | 100K | <2ms | <15ms | $5K (10-node Aerospike + sharded Redpanda + ClickHouse + S3) |

Targets are aspirational; Phase 0 baseline + Phase 14 bench confirm.

### Breaking changes

- All existing Go storage data is discarded on adoption.
- BRC-40 sync no longer supported — any deployment that relies on sync must stay on Option A.
- `pkg/storage/`'s public Go API is rewritten. Library consumers must migrate.
- SQL-level introspection no longer possible.

## Decisions confirmed by author

1. **Drop BRC-40 sync entirely.** Multi-storage replication, if needed, is achieved by replaying the Redpanda event log, not by sync protocol.
2. **Conformance bar = BRC-100 JSON-RPC only.** No TS schema parity.
3. **Aerospike as the high-scale KV.** Alternatives (Scylla, FoundationDB, DynamoDB) considered; Aerospike chosen for sub-ms p99 on SSD and proven >1M ops/s/node.
4. **One binary, runtime-config backend.** No build tags, no separate distros. Same code runs Pi → cluster.
5. **No data migration.** Existing data is discarded.
6. **Event log as canonical replay.** All state mutations land in Redpanda; KV is a materialized projection. Cluster restart rebuilds from event log if needed.
7. **ClickHouse for historical reads.** OLAP queries get separate engine; KV stays narrow-fast.
8. **Blobs in S3.** Raw tx > 4KB never live in hot KV.
9. **Encryption at rest is out of scope.** Transaction data is public (on-chain). All sensitive metadata is already encrypted client-side before reaching storage. Storage-layer encryption would add CPU cost with no added confidentiality. Operators who still want disk-level encryption use the backend's native mechanism (Aerospike enterprise, EBS encryption, dm-crypt, etc.) without changes to this codebase.
10. **Aerospike enterprise license is the scale path.** Community edition serves up to 5 nodes / 4TB. Beyond that, operators pay for enterprise. No ScyllaDB fallback path; backend interface stays clean rather than supporting both.
11. **Read-your-writes is strict.** Post-write reads block on materializer catch-up with a bounded timeout (500ms default). No best-effort mode, no per-request opt-out. Simpler contract beats configurable correctness.
12. **Cluster-internal RPC is gRPC + protobuf; edge is JSON-RPC.** External BRC-100 clients see HTTP/2 JSON-RPC unchanged. Worker-to-worker and worker-to-materializer use gRPC for ~5x lower per-call CPU cost.
13. **In-memory mode for embedded driver is in scope.** `embedded` backend supports `--memory-only` flag for test/CI/ephemeral wallets. No durability. Cheap to add; useful immediately.
14. **Out of scope:** streaming RPC for deep listActions; read-replica fan-out hint; multi-region active-active. Revisit in a future Option B v2 if customer demand surfaces.

## Open questions

None outstanding. Items previously open are now resolved:

1. **BRC-100 internal-state leakage:** Phase 0 audit task still runs, but is verification, not a decision.
2. **Multi-device replication:** locked to event-log subscribe + materialize.
3. **Embedded SD-card endurance:** in-memory mode (D13) covers ephemeral cases; SSD is documented requirement for durable embedded.
4. **Aerospike vs ScyllaDB:** Aerospike committed (D10); enterprise license is the scale path. No ScyllaDB driver.
5. **Postgres fifth driver:** out — would undermine the performance story.

## Alternatives considered

- **Option A — TS schema conformance.** Different goal. Lives in parallel; orgs pick A or B.
- **Hybrid (RPC conformance + Postgres backend).** Cheap to ship, doesn't reach 100K tps. Defeats Option B's purpose.
- **Fork Postgres-only Option A and add caching layer.** Postpones the throughput cliff but doesn't eliminate it. Joins still bottleneck.
- **Pure event-sourcing without materialized KV.** Read latency unbounded; rejected.
- **Single-node SQLite with WAL.** Caps at ~1K tps. Not in target range.
