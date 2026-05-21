# Tasks — Option B RPC-only ultra-performance storage

Sequenced by risk. Each phase is independently mergeable. Early phases are foundational; later phases scale.

## Phase 0 — Audit and baseline

- [ ] Enumerate every BRC-100 JSON-RPC method consumed by the existing `pkg/storage/` and `pkg/wallet/`. Build the conformance method list in `rpc-methods.md`.
- [ ] Audit each method for internal-state leakage (e.g., returns surrogate IDs the client must roundtrip). Flag any leaks; design around them.
- [ ] Prototype event-log replay between two storage instances to confirm it can replace BRC-40 sync for multi-device users.
- [ ] Capture baseline benchmark of the current `pkg/storage/provider.go` against a SQLite backend at 10/100/1000/5000 tps. Save to `bench/baseline-current-storage.md`.
- [ ] Stand up a one-node Aerospike + one-node Redpanda + one-node ClickHouse + MinIO test environment via docker-compose. Document setup in `infra/optionb-dev/`.

## Phase 1 — `Storage` interface and key-builder

- [ ] Define `pkg/storage/backend/storage.go`: the canonical `Storage` interface exposing every domain operation the RPC layer needs. No table accessors. Operations are e.g. `GetUser`, `PutUser`, `SelectCoins(userID, amount, basket)`, `AppendActionLog(userID, event)`, `MaterializeAction(userID, txid)`, `GetUTXO(userID, outpoint)`, etc.
- [ ] Define `pkg/storage/keys/keys.go`: deterministic byte-key builders for every entity. Functions like `UserKey(identityKey)`, `UTXOKey(identityKey, outpoint)`, `ActionLogKey(identityKey, seqno)`. All keys are byte slices; never strings post-encoding.
- [ ] Write fuzz tests that confirm key builders produce sortable, prefix-scannable byte ranges for cursor iteration.
- [ ] Write a `MemStorage` reference impl of `Storage` (in-memory map) for unit tests.

## Phase 2 — Embedded BadgerDB driver

- [ ] Implement `pkg/storage/backend/embedded/badger.go` against the `Storage` interface.
- [ ] Wire BadgerDB lifecycle (open, close, GC, value log).
- [ ] Implement prefix scans for list operations using BadgerDB iterators.
- [ ] Add CAS via BadgerDB's `Txn` with conflict detection.
- [ ] **In-memory mode**: support `--memory-only` flag; initialize BadgerDB with `Options.InMemory = true`. Zero-durability semantics documented. Tests and ephemeral wallets use this mode.
- [ ] Conformance test suite: run a wallet RPC harness against the embedded backend, both durable and in-memory modes. All BRC-100 methods must pass.
- [ ] Bench embedded at 10/100/500/1000 tps. Save to `bench/embedded.md`.

## Phase 3 — RPC layer rewire

- [ ] Replace all `pkg/storage/provider.go` methods with calls to the `Storage` interface. Drop the gorm-backed implementation.
- [ ] Delete `pkg/internal/storage/database/`.
- [ ] Delete `pkg/internal/storage/repo/`.
- [ ] Delete `pkg/internal/storage/repo/syncrepo/`.
- [ ] Rewire `pkg/wallet/` and `pkg/storage/internal/actions/` to call `Storage`-interface methods.
- [ ] Run existing wallet behavioural tests against `embedded` driver. Fix breakages.

## Phase 4 — Bucketed UTXO selection

- [ ] Design satoshi-range bucket scheme. Buckets are log-scale (e.g. `[0,1K)`, `[1K,10K)`, `[10K,100K)`, ...). Per-user, per-basket bucket index lives at `user:<id>:basket:<name>:bucket:<range>` → SortedSet of outpoints.
- [ ] Implement `SelectCoins(userID, amount, basket)`: pick the smallest bucket whose total ≥ amount, scan within. Worst-case O(bucket count) ≈ O(log satoshi range).
- [ ] Maintain bucket index on every UTXO add/spend in the materializer.
- [ ] Bench selection latency vs naive scan at 10K/100K UTXOs per user. Confirm O(log) behaviour.

## Phase 5 — Event log integration (Redpanda)

- [ ] Implement `pkg/storage/eventlog/redpanda.go`: producer publishes every state-mutating RPC as an event. Topic per shard.
- [ ] Implement consumer that materializes events into the local KV. Idempotent on seqno.
- [ ] Define event schema: protobuf or msgpack, versioned.
- [ ] Backpressure: if consumer lags > N seconds, RPC layer signals 503 with retry-after.
- [ ] Single-node mode: in-process log file (`pkg/storage/eventlog/embedded.go`) for `embedded` and `single-node` backends.

## Phase 6 — Async materializer

- [ ] Implement `pkg/storage/materializer/materializer.go`: per-shard goroutine that consumes the local event log and updates KV projections.
- [ ] Idempotency: every event carries (shard, seqno); materializer dedups by `materializer:cursor:<shard>` checkpoint.
- [ ] Crash recovery: restart resumes from last checkpoint, replays missing events.
- [ ] Bench: how many events/sec one materializer goroutine can absorb. Target 200K events/s per core.

## Phase 7 — S3 blob driver

- [ ] Implement `pkg/storage/blobs/s3.go`: read/write blobs by content hash (sha256).
- [ ] Inline cutoff: blobs ≤ 4KB stay inline in KV record; > 4KB go to S3, KV stores reference.
- [ ] Background uploader: KV writes can return before S3 upload completes; uploader retries with exponential backoff.
- [ ] Local fallback driver for `embedded` mode: filesystem-backed blob store.
- [ ] Bench: blob fetch latency from S3 at p50/p99. Establish if read-through caching is needed.

## Phase 8 — Aerospike driver

- [ ] Implement `pkg/storage/backend/aerospike/aerospike.go` against the `Storage` interface.
- [ ] Configure namespace, set, indexes. Use Aerospike's per-record generation for CAS.
- [ ] Prefix scans via Aerospike's `Scan` API or secondary indexes on key prefix.
- [ ] Sharding: identityKey hash → Aerospike partition. Confirm hot-key avoidance (no single-bin hotspots).
- [ ] Run wallet RPC harness against Aerospike backend.
- [ ] Bench at 1K/5K/10K/25K/50K tps single-node Aerospike. Save to `bench/aerospike-single.md`.

## Phase 9 — ClickHouse historical reads

- [ ] Define historical schema: append-only `actions` table partitioned by date, ordered by `(userId, timestamp)`.
- [ ] Background ingester: Redpanda consumer that writes events to ClickHouse in batched inserts.
- [ ] `listActions` with deep-history filter routes to ClickHouse; recent (<7d) stays in KV.
- [ ] Verify ClickHouse query latency for typical filters (userId + label + date range).
- [ ] Bench historical query latency at 1M, 100M, 1B rows in ClickHouse.

## Phase 10 — Sharding and consistent-hash routing

- [ ] Implement consistent-hash router for `cluster-hot` and `cluster-extreme` backends: identityKey → shard ID.
- [ ] Worker pool per shard; RPC dispatcher routes incoming calls to the owning worker.
- [ ] Rebalancing: shard map versioned; rebalance moves a user's state by replaying their event log on the new shard.
- [ ] Bench: routing overhead per RPC call. Target < 50μs.

## Phase 10a — Cluster-internal gRPC layer

- [ ] Define protobuf schema (`pkg/storage/internalrpc/proto/`) for every worker-to-worker and worker-to-materializer call. Mirror the `Storage` interface where applicable.
- [ ] Generate Go stubs via `buf generate` or `protoc-gen-go-grpc`.
- [ ] Implement gRPC server on each worker; expose only intra-cluster operations on a separate listener from the BRC-100 JSON-RPC edge.
- [ ] Implement gRPC client wrapper that the router uses for cross-shard dispatch.
- [ ] Edge gateway: translates incoming BRC-100 JSON-RPC into internal gRPC calls. One translation per request.
- [ ] Bench: per-call CPU cost JSON-RPC vs gRPC for the same operation. Target ≥4x reduction.
- [ ] Secure intra-cluster traffic: mTLS between workers; cert rotation strategy documented.

## Phase 11 — Cluster-hot backend wiring

- [ ] Implement `pkg/storage/backend/clusterhot/cluster.go`: composes Aerospike + Redpanda + S3 + materializer + router.
- [ ] Conformance test against full wallet RPC suite.
- [ ] Bench end-to-end at 5K/10K/25K/50K tps on a 3-node test cluster.

## Phase 12 — Cluster-extreme backend wiring

- [ ] Implement `pkg/storage/backend/clusterextreme/cluster.go`: composes sharded Aerospike + sharded Redpanda + ClickHouse + S3 + materializer + router.
- [ ] Conformance test.
- [ ] Bench at 50K/75K/100K tps on a 10-node test cluster.
- [ ] Tail-latency profile under sustained load. Document p99/p99.9.

## Phase 13 — Backpressure and graceful degradation

- [ ] RPC entrypoint reads three signals: materializer lag, event-log producer lag, Aerospike client queue depth.
- [ ] When any signal trips threshold: return 503 with `Retry-After`. Define thresholds per backend.
- [ ] Test under intentional partition: kill an Aerospike node, kill a Redpanda broker, verify graceful degradation.

## Phase 14 — Observability

- [ ] Prometheus metrics on every backend driver: ops/s, p50/p99 latency, error rate, queue depth.
- [ ] OpenTelemetry tracing on RPC path.
- [ ] Per-shard tps dashboards.
- [ ] Alerting rules for materializer lag, ClickHouse ingester lag, S3 upload failure rate.

## Phase 15 — Cost optimization sweep

- [ ] At each backend tier, measure $/1K-ops. Document in `cost-curve.md`.
- [ ] Tune Aerospike write-block size, hash-table memory, defrag-startup-minimum to lower RAM footprint.
- [ ] Tune ClickHouse compression codecs, partition pruning.
- [ ] Tune S3 storage class (Standard → IA → Glacier) per blob age.
- [ ] Validate "10 tps for $5/month" claim end to end.

## Phase 16 — Documentation and rollout

- [ ] `docs/storage-optionb.md` — operator guide for each backend tier.
- [ ] Migration guide: there is no migration path; document expected discard.
- [ ] Decision tree: when to pick Option B over Option A.
- [ ] Update CHANGELOG with breaking notice.

## Phase 17 — Archive change

- [ ] Verify all phases merged.
- [ ] Run `openspec archive` to merge deltas into main specs.
- [ ] Tag release.
