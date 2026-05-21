# Spec delta — `storage-rpc` capability

Adds Option B's RPC-only conformance bar and the storage backend interface contract. Removes BRC-40 sync.

## ADDED Requirements

### Requirement: BRC-100 JSON-RPC as the sole conformance bar

The wallet storage layer SHALL conform to the BRC-100 JSON-RPC wallet method contract. No internal-state shape, no schema layout, and no storage backend choice is part of the conformance bar.

#### Scenario: BRC-100 method invocation

- **GIVEN** a wallet client that speaks BRC-100 JSON-RPC
- **WHEN** the client calls any method in the BRC-100 surface area (e.g. `createAction`, `listOutputs`, `internalizeAction`)
- **THEN** the response SHALL match the BRC-100 contract exactly
- **AND** no observable behaviour SHALL depend on which backend driver the storage layer uses

#### Scenario: Backend swap is transparent

- **GIVEN** a wallet client connected to a Go wallet storage running the `embedded` backend
- **WHEN** the operator switches the backend to `cluster-extreme` and replays the user's event log
- **THEN** subsequent client calls SHALL return identical results to the embedded run for any deterministic method

### Requirement: `Storage` interface as backend abstraction

A single `Storage` Go interface in `pkg/storage/backend/storage.go` SHALL define every operation the wallet domain needs. Every backend driver SHALL implement this interface exactly.

The interface SHALL be expressed in domain terms (`SelectCoins`, `AppendActionLog`, `GetUTXO`, `MaterializeAction`) and SHALL NOT expose tables, columns, joins, or transactions.

#### Scenario: Driver pluggability

- **GIVEN** a Go package implementing the `Storage` interface
- **WHEN** the wallet domain layer is initialized with that driver
- **THEN** all wallet RPC behaviour SHALL work without any change to the domain layer

#### Scenario: Behavioural conformance test

- **GIVEN** the wallet behavioural test suite under `test/storage-conformance/`
- **WHEN** the suite is run against any `Storage` driver
- **THEN** all tests SHALL pass

### Requirement: Backend tiers

Four backend drivers SHALL be provided, each implementing the `Storage` interface:

| Driver name | Target deploy | Backing tech |
|-------------|---------------|--------------|
| `embedded` | 10–100 tps | BadgerDB |
| `single-node` | 100–5K tps | BadgerDB + optional ClickHouse |
| `cluster-hot` | 5K–50K tps | Aerospike + Redpanda + S3 |
| `cluster-extreme` | 50K–100K tps | Sharded Aerospike + sharded Redpanda + ClickHouse + S3 |

#### Scenario: Backend selected by config

- **GIVEN** a `config.yaml` with `storage.backend = "cluster-hot"`
- **WHEN** the wallet starts
- **THEN** the `cluster-hot` driver SHALL be loaded
- **AND** no other driver SHALL be initialized

### Requirement: Event log as canonical replay

Every state-mutating RPC call SHALL produce an event in a durable, ordered event log (Redpanda for cluster tiers; embedded log file for `embedded` and `single-node`). The KV state SHALL be a materialized projection of the event log.

#### Scenario: Crash recovery

- **GIVEN** a wallet storage node whose KV backend has lost data
- **WHEN** the node restarts with the event log intact
- **THEN** the materializer SHALL rebuild KV state by replaying events from the last checkpoint
- **AND** the rebuilt state SHALL be functionally identical to pre-crash state

#### Scenario: Multi-device replication via event log

- **GIVEN** two wallet storage nodes serving the same user
- **WHEN** one node writes events to the user's event log partition
- **THEN** the other node's materializer SHALL consume those events
- **AND** the second node's KV state SHALL converge to the first node's state

### Requirement: Bucketed UTXO selection

UTXO coin selection SHALL achieve O(log) complexity in the UTXO count, via per-user, per-basket bucket indexes keyed by satoshi range.

#### Scenario: Selection scales with UTXO count

- **GIVEN** a user with 10K UTXOs
- **AND** the same user later with 100K UTXOs
- **WHEN** coin selection runs for an arbitrary target amount
- **THEN** p99 selection latency SHALL be within 2× across both scenarios

### Requirement: Read-your-writes via cursor

The RPC contract SHALL preserve read-your-writes semantics: a write that has returned to the caller SHALL be visible on subsequent reads from the same caller.

#### Scenario: Write then read

- **GIVEN** a caller invokes `createAction` and receives a successful response
- **WHEN** the caller invokes `listOutputs` immediately afterward
- **THEN** the outputs from the just-created action SHALL appear in results
- **AND** the read SHALL block (up to a bounded timeout) on materializer catch-up if needed

### Requirement: Backpressure as 503 with Retry-After

When any of {materializer lag, event-log producer lag, backend client queue depth} exceeds a per-backend threshold, the RPC server SHALL respond with HTTP 503 and a `Retry-After` header.

#### Scenario: Materializer falls behind

- **GIVEN** the materializer is > 5 seconds behind the event log
- **WHEN** a new state-mutating RPC arrives
- **THEN** the server SHALL respond with 503
- **AND** the response SHALL include `Retry-After` with a value derived from current lag

### Requirement: Performance budgets per tier

Each backend tier SHALL meet the following budgets under steady-state load at its target tps:

| Tier | tps | p50 | p99 |
|------|-----|-----|-----|
| `embedded` | 100 | < 5ms | < 50ms |
| `single-node` | 5K | < 3ms | < 30ms |
| `cluster-hot` | 50K | < 2ms | < 10ms |
| `cluster-extreme` | 100K | < 2ms | < 15ms |

#### Scenario: Tier meets budget

- **GIVEN** a deploy of any tier at its target tps
- **WHEN** sustained load is applied for 30 minutes
- **THEN** measured p50 and p99 latencies SHALL meet the budget for that tier

## REMOVED Requirements

### Requirement: BRC-40 sync protocol

**Reason:** Option B does not implement BRC-40 sync. Multi-storage replication is achieved by event-log subscription instead. Operators who need BRC-40 sync compatibility take Option A.

**Migration:** No migration. Existing BRC-40 sync code is deleted.

### Requirement: Schema portability with `ts-stack/wallet-toolbox`

**Reason:** Option B's internal state is not a relational schema; it is a KV-shaped projection. No schema portability is offered or implied.

**Migration:** Internal data tooling that expected SQL-level introspection is unsupported.

### Requirement: SQL backend

**Reason:** SQL backends cap throughput well below Option B's 100K tps target. No SQL driver is included.

**Migration:** Operators who want SQL choose Option A.

### Requirement: Direct table access from application code

**Reason:** Option B exposes only the `Storage` interface. No application code touches tables, columns, or transactions directly.

**Migration:** All application code is rewritten against the `Storage` interface during Phase 3.
