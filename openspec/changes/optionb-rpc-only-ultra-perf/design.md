# Design — Option B RPC-only ultra-performance storage

## Goals

1. Conform only to BRC-100 JSON-RPC wallet contract.
2. Run on a $5 VPS at 10 tps with sub-50ms p99.
3. Scale to 100K tps on a 10-node cluster with sub-15ms p99.
4. One Go binary; backend selected by runtime config.
5. Operational cost grows roughly linearly with throughput.

## Non-goals

1. BRC-40 sync protocol compatibility.
2. Internal schema portability with `ts-stack/wallet-toolbox`.
3. Direct SQL introspection of storage state.
4. Backwards-compatible data migration from existing Go storage.

## Architecture

### Layered model

```
┌─────────────────────────────────────────────────────┐
│ BRC-100 JSON-RPC server (HTTP/2)                    │
├─────────────────────────────────────────────────────┤
│ Wallet domain layer (createAction, signAction, ...) │
├─────────────────────────────────────────────────────┤
│ Storage interface (Go: pkg/storage/backend)         │
├─────────────────────────────────────────────────────┤
│ Backend driver: embedded | single | hot | extreme   │
├──────────┬────────────┬──────────────┬──────────────┤
│ BadgerDB │ Aerospike  │ Redpanda     │ ClickHouse   │
│ (KV)     │ (cluster   │ (event log)  │ (analytics)  │
│          │  KV)       │              │              │
├──────────┴────────────┴──────────────┴──────────────┤
│ S3 / MinIO (blob)                                   │
└─────────────────────────────────────────────────────┘
```

### Why each technology

**BadgerDB (embedded backend):** pure Go, no CGo, LSM-tree on SSD. Sustains 100K+ writes/s/core on commodity NVMe. Single-process embedded. Zero ops cost. Reasonable replacement for SQLite at higher write rates, with built-in compression and snapshots.

**Aerospike (cluster KV):** sub-ms p99 reads from SSD via hybrid memory architecture. Linear scaling to billions of records per node. Per-record CAS via generation counter. Proven >1M ops/s/node in production at financial-services scale. License-free community edition for our scale.

**Redpanda (event log):** Kafka-API compatible but written in C++ with thread-per-core architecture. No JVM. Sustains 1GB/s writes per node. Replaces Kafka without operational overhead.

**ClickHouse (analytics):** columnar, vectorized. Billions of rows per node for aggregate queries (listActions filtered by label across years). Read-replica of the event log keeps it fresh.

**S3 / MinIO (blobs):** large blobs (BEEF, raw txs > 4KB) never belong in hot KV. Object store with lifecycle policies handles cold storage cheaply.

### Why not other choices

- **ScyllaDB**: comparable to Aerospike; C++/Seastar, thread-per-core. Acceptable alternative. Aerospike preferred for primary-index sub-ms latency, ScyllaDB preferred for very wide rows. Decision deferred to Phase 8 bench.
- **FoundationDB**: strong serializable ACID, beautiful interface, but lower raw throughput per node and tighter transaction-size limits. Better fit for relational workloads, worse for our KV pattern.
- **DynamoDB**: managed, comparable perf, but vendor lock-in and unpredictable costs at scale.
- **Postgres**: caps at ~10K writes/s/node. Defeats the purpose.
- **etcd / Consul**: optimized for config, not throughput. Out.

### Sharding strategy

Per-user sharding. Consistent hash on `identityKey` selects a shard. All writes for a user go to the owning shard. Reads for a user go to the owning shard. Cross-user operations (global proven_tx, headers, settings) are rare; they use Aerospike's per-record CAS or eventual-consistency via event log.

This means **a single user is bounded by single-shard throughput** (~50K ops/s in `cluster-hot`). For wallets that need >50K ops/s for one user, that user's state would need to be partitioned (e.g. by basket) — out of scope until a customer asks.

### Single-writer-per-user

Worker pool sized to shard count × some multiplier. Each user is consistently routed to one worker. That worker is the sole writer for the user's state. No locks needed. Reads can happen on any worker (KV is eventually consistent across replicas).

### Event log as source of truth

Every state-mutating RPC produces an event in Redpanda before responding to the caller. KV is a materialized projection.

- Crash of a KV node → rebuild from event log replay.
- Migration to a new backend → consume event log into new backend.
- Audit / debugging → replay log into a test environment.
- BRC-40 sync replacement (multi-device) → second storage subscribes to user's event partition.

Trade-off: event log adds latency. Mitigated by:
- Co-locating Redpanda with KV in same datacenter (sub-ms write).
- Returning to caller after event log ack, before KV materialization. KV gets caught up by materializer goroutine.

### Materializer pipeline

Goroutine per shard reads local event log partition, applies events to KV. Checkpoint per shard at `materializer:cursor:<shard>`. Crash-safe: resume from checkpoint.

Throughput target: 200K events/sec per core. At 100K tps RPC, average 2 events per RPC = 200K events/s, fits one core per shard. Backpressure if a shard falls > 5s behind.

### Read-path tiering

| Query pattern | Tier | Latency budget |
|---------------|------|----------------|
| getUTXO, getUser, getCert, getBasket | Aerospike (RAM-resident hot set) | < 2ms p99 |
| listOutputs paginated | Aerospike scan | < 5ms p99 |
| listActions last 7 days | Aerospike scan | < 10ms p99 |
| listActions older than 7 days | ClickHouse | < 100ms p99 |
| listTransactions with proof status | Aerospike join client-side | < 10ms p99 |
| BEEF / raw tx | S3 (with KV cache for hot txs) | < 50ms p99 cold |

### Coin selection at scale

Naive `outputs WHERE spendable = true ORDER BY satoshis` does not scale past 100K UTXOs per user. Replacement: bucketed sorted set.

Buckets indexed by satoshi range (log scale: 0-1K, 1K-10K, 10K-100K, etc.). For a target amount T, scan buckets descending from the largest bucket ≤ T, then pick smallest fit. O(bucket count) ≈ O(log T).

Per-basket-per-user bucket maintenance happens in materializer when outputs are added/spent. KV key: `user:<id>:basket:<name>:bucket:<idx>` → SortedSet of outpoints by satoshi.

### Cost model

Approximate $/month per backend at target throughput, assuming 1M active users with ~10 ops/user/day:

- **embedded (10 tps)**: $5/mo (1 small VPS). User pays only their VPS bill.
- **single-node (5K tps)**: $50/mo (1 medium VPS + S3 storage).
- **cluster-hot (50K tps)**: $1K/mo (3 Aerospike nodes m6gd.2xlarge + 3 Redpanda nodes + S3 + ClickHouse single).
- **cluster-extreme (100K tps)**: $5K/mo (10 Aerospike nodes + 5 Redpanda nodes + 3-node ClickHouse + S3).

Cost dominated by RAM at high tiers (Aerospike). SSD-only mode halves RAM cost at ~30% latency penalty; available via Aerospike config.

## Risk register

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| BRC-100 method leaks internal-state assumption | Medium | High | Phase 0 audit; design around per method |
| Event-log replay can't fully replace BRC-40 sync for multi-device | Medium | Medium | Phase 0 prototype; if blocked, document limitation |
| Aerospike per-record gen CAS conflicts at hot global keys | Medium | Medium | Partition global keys (e.g. shard headers by height range); use Aerospike CDT for set-of-references |
| Materializer lag visible to user as "I created an action but listOutputs doesn't show it yet" | High | Medium | Read-your-writes guarantee: post-write returns include a cursor, listOutputs accepts cursor and waits until materializer caught up |
| BadgerDB embedded mode disk endurance on Raspberry Pi SD cards | Medium | Low | Document SSD requirement; provide memory-only mode for ephemeral wallets |
| ClickHouse ingester lag means historical reads miss recent data | Low | Low | Document 7-day window; reads in transition zone consult both KV and ClickHouse |
| Aerospike licensing terms change | Low | High | Backend abstraction allows swap to ScyllaDB with bounded effort |
| Operational complexity scares off small operators | High | Medium | Embedded + single-node tiers don't need cluster ops; same code |
| Sharding hot-key (whale wallet) | Low | High | Document per-user shard ceiling; provide guidance for partitioned-user mode |
| Crypto throughput becomes bottleneck before storage | Medium | Medium | Profile; cache derived keys; use hardware acceleration (AES-NI, AVX) |
| GC pauses in Go at high allocation rates | Medium | Medium | Pool buffers; sync.Pool for hot allocations; consider GOGC tuning |
| JSON parsing CPU at 100K tps | High | Medium | Use sonic or go-json; consider switching to msgpack on RPC for high-tier deployments |

## Decision log

### D1: Drop BRC-40 sync entirely

Rationale: sync is a hard correctness requirement when storage shape is part of the contract (Option A). Option B's contract is RPC only, so users replicate state by subscribing to the event log, not by walking sync chunks. Removing sync deletes ~40% of repo complexity and removes a major performance hazard (sync upserts have to be transactional across many tables).

### D2: One binary, runtime-configured backend

Rationale: operators want to start small and scale up without rewriting. Single binary means deployment and CI are simple. Backend interface forces every driver to satisfy the same contract, so behavioural conformance is uniform.

### D3: Aerospike over ScyllaDB by default

Rationale: sub-ms p99 reads. Aerospike's strong consistency mode covers our CAS needs. Community license is free at our expected scale. ScyllaDB remains as a fallback if licensing or operational concerns surface.

### D4: Event log as source of truth, KV as projection

Rationale: lets us swap KV backends, rebuild on crash, replay for audit, support multi-device replication. The classic event-sourcing trade-off: write latency slightly higher but failure recovery and flexibility much better. At 100K tps the marginal latency from a Redpanda ack is ~0.5ms — affordable.

### D5: No data migration from existing storage

Rationale: schema is unrecognizable to current storage. Migration tooling would cost weeks and bound the design (have to keep some semblance of relational state). Since no production users exist on Go yet, throw it away. Operators who want migration take Option A.

### D6: Bucketed UTXO selection by satoshi range

Rationale: O(log) selection regardless of UTXO count is mandatory at 100K tps. Sorted-set-by-bucket fits Aerospike's CDT operations naturally and embedded BadgerDB's prefix scans.

### D7: S3-compatible blob layer

Rationale: keeps hot KV records small (bytes per record matters for Aerospike RAM). S3-compatible is the universal blob protocol — operators pick AWS S3, Cloudflare R2, GCS, or self-host MinIO without code change.

### D8: ClickHouse for historical reads, not Aerospike

Rationale: aggregate queries (count actions, sum satoshis, filter by label across years) over billions of rows are columnar OLAP territory. Aerospike is row-store KV; running OLAP on it would be both slow and expensive. ClickHouse is the right tool. Separation also isolates analytical load from hot-path KV.

### D9: Backpressure via 503 with Retry-After

Rationale: the alternative — silent latency creep when materializer falls behind — produces a worse user experience than an explicit "try again in 200ms." RPC clients can retry; standardized via HTTP Retry-After header.

### D10: Per-user single-writer worker model

Rationale: eliminates locks entirely on the hot path. Sharded routing means each user maps to exactly one worker; that worker is the sole mutator. Latency floor is the worker's queue depth, not lock contention.

## What is the actual bottleneck at 100K tps?

In rough order, anticipated bottlenecks (numbers approximate):

1. **JSON-RPC parsing** — ~3μs per call with `encoding/json`, ~1μs with sonic. At 100K rps that's 100ms-300ms of CPU per second of wall time. Solvable with sonic + maybe msgpack at the highest tier.
2. **Signature verification** — ECDSA verify ~200μs without optimization. Cached pre-computed pubkeys + batch verification gets this to ~20μs. Critical to hit.
3. **Event-log write fsync** — Redpanda batched. Sub-ms p99 on NVMe with batching. Not a bottleneck if batched.
4. **KV write to Aerospike** — sub-ms p99 on hybrid mem-SSD. Not a bottleneck.
5. **Coin selection** — O(log) with bucketing. Sub-100μs. Not a bottleneck.
6. **Network RTT** — 0.1-1ms intra-DC. Bounded.
7. **GC pauses** — Go GC pauses can hit ms scale at high allocation rates. Pool buffers; goal < 100μs pause budget.

The real ceiling for `cluster-extreme` is likely **Aerospike client throughput per Go process**, around 50-100K ops/s/process. Solution: multiple worker processes per node behind a load balancer; horizontal scale.

Network bandwidth at 100K tps with ~1KB BEEF payloads = 100MB/s ingress. Single 10G NIC handles this with headroom. At 1Gbps deployments, this is a hard ceiling; document accordingly.

## Open architectural questions

1. **Do we expose a streaming RPC variant?** WebSocket or gRPC streaming for `listActions` over deep history could halve client-perceived latency. Defer until customer asks.
2. **In-memory mode for ephemeral wallets?** Test wallets, integration tests, CI fixtures don't need durability. `embedded` driver with `--memory-only` flag is cheap to add.
3. **Read-replica fan-out for `listOutputs` heavy clients?** Aerospike supports read replicas; do we expose a "read from any replica" hint for analytics-like consumers? Defer.
4. **Encryption at rest** beyond what the storage backend provides? Wallets store sensitive material. Consider per-user envelope encryption with KMS. Defer to security review.
5. **Multi-region active-active**? Event log can be mirrored; KV state can be regional. Out of scope until customer asks.
