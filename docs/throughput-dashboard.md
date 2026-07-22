# Throughput Demo Dashboard (Mainnet)

Demo control plane for observing **throughput-mode** wallet infrastructure:

- Local Postgres + `infra_throughput` with `utxo_management.strategy: throughput`
- Operator wallet client (holds keys) + **FuelKeeper**
- Controllable createAction **event stream** with per-action OP_RETURN
  `sha256(iteration ‖ RFC3339Nano timestamp)`
- Web UI: start/stop stream, TPS chart, fuel/reserve/default balances, top-up feed
- Browser **`@bsv/sdk` WalletClient** funding into the operator deposit address

> **Mainnet.** Real BSV and fees. Dashboard defaults to `BSV_NETWORK=main` and binds the UI to localhost.

Related:

- Throughput UTXO design: [`examples/throughput_mode/throughput-mode.md`](../examples/throughput_mode/throughput-mode.md)
- Fixed-payload live-test loadgen stack: [`docs/throughput-docker.md`](throughput-docker.md)

## Mainnet profile

Matches the live-test OP_RETURN shape; fee and denomination must stay aligned with
`config.DemoThroughput()` and `infra-config-docker-throughput-mainnet.yaml`.

| Setting | Value |
|---|---|
| Network | `main` |
| UTXO strategy | `throughput` |
| Fee model | `sat/kb` value `100` |
| Expected tx size | `200` bytes |
| Expected output satoshis | `0` (data-only OP_RETURN) |
| Target TPS (server pool sizing) | `1000` |
| Derived fuel denomination | **20 sats** |
| Dashboard default stream TPS | `10` (raise only when funded) |

## Architecture

```
Browser (http://127.0.0.1:8200)
  └─ cmd/throughput_dashboard
       ├─ FuelKeeper (default → reserve → fuel)
       ├─ StreamController (rate-limited createAction)
       └─ storage client ──► infra_throughput (:8101 host / :8100 compose)
                                  └─ Postgres
```

The storage server never holds operator keys. Fan-out top-ups are signed in the dashboard process.

| Service | Role | Host access |
|---|---|---|
| `db` | Postgres 17 (`db-data-throughput-dashboard`) | `127.0.0.1:5433` |
| `infra` | `cmd/infra_throughput` + mainnet throughput yaml | `http://127.0.0.1:8101` |
| `dashboard` | UI + FuelKeeper + stream control | `http://127.0.0.1:8200` |

## Operator runbook

Follow this order: **fund → FuelKeeper → start stream**.

### 1. Prerequisites

1. Docker Compose v2 and a funded **operator** wallet private key (hex). Never commit keys.
2. Optional but recommended for reliable mainnet broadcast / internalize:
   - `INFRA_WALLET_SERVICES_ARC_TOKEN` — mainnet ARC token
   - `INFRA_WALLET_SERVICES_WHATS_ON_CHAIN_API_KEY` — if you use WOC-backed paths

### 2. Start the stack

```bash
export PRIVATE_KEY=<hex-private-key>
# Recommended for reliable mainnet broadcast:
# export INFRA_WALLET_SERVICES_ARC_TOKEN=<mainnet-arc-token>
# export INFRA_WALLET_SERVICES_WHATS_ON_CHAIN_API_KEY=<optional>

docker compose -f docker-compose.throughput-dashboard.yaml up --build
```

Open **http://127.0.0.1:8200**. Compose fails fast if `PRIVATE_KEY` is unset.

### 3. Fund the operator (default basket)

1. Install / unlock a MetaNet-compatible browser wallet that implements `@bsv/sdk` `WalletClient`.
2. On the **Fund operator** panel, set satoshis (default 100 000) and click **Pay with WalletClient**.
3. The UI pays the operator BRC-29 deposit address, then `POST /api/funding/internalize` credits the **default** basket.
4. Alternate path: pay the deposit address externally, then internalize via atomic tx hex or `{ txid, output_index }` on the funding API.

### 4. Wait for FuelKeeper

FuelKeeper runs as soon as the dashboard process starts (every ~10s when below low water):

1. Fans **default → reserve** chunks
2. Mints exact-denomination **fuel** UTXOs for createAction

Watch the UI balances and top-up feed. Until fuel is available, createActions may fall back to slower default-basket funding or fail with not-enough-funds — both are counted in the UI.

Do **not** start a high-rate stream until fuel inventory is rising.

### 5. Start the stream

1. Confirm fuel/reserve gauges look healthy.
2. Start at low TPS (compose default is `10`).
3. Click **Start stream** (or `POST /api/stream/start` with optional `{ "tps", "workers" }`).
4. Confirm TPS chart and event feed move; stop anytime with **Stop stream** (FuelKeeper keeps running).

Each createAction has a single 0-sat output:

```
OP_RETURN <32-byte sha256( strconv(iteration) + time.RFC3339Nano )>
```

`AcceptDelayedBroadcast` is enabled for high-rate demos.

### 6. Stop / tear down

```bash
# Ctrl-C if attached, or:
docker compose -f docker-compose.throughput-dashboard.yaml down

# Also drop the Postgres volume (destroys demo DB state):
docker compose -f docker-compose.throughput-dashboard.yaml down -v
```

## Local run (infra already up)

If the throughput infra is already listening on `8101`:

```bash
export PRIVATE_KEY=<hex>
export SERVER_URL=http://127.0.0.1:8101
export BSV_NETWORK=main
go run ./cmd/throughput_dashboard
```

## Environment (dashboard)

| Variable | Default | Notes |
|---|---|---|
| `PRIVATE_KEY` | *(required)* | Operator hex key |
| `SERVER_URL` | `http://127.0.0.1:8101` | Compose: `http://infra:8100` |
| `BSV_NETWORK` | `main` | Mainnet-first |
| `HTTP_ADDR` | `127.0.0.1:8200` | Compose binds `0.0.0.0:8200` published to localhost only |
| `TPS` | `10` | Safer mainnet default (raise when funded) |
| `WORKERS` | `8` | Concurrent createActions |
| `SAMPLE_SECONDS` | `1` | Gauge / TPS sample interval |
| `ORIGINATOR` | `throughput-dashboard.local` | createAction originator |

Infra (compose) also accepts:

| Variable | Notes |
|---|---|
| `INFRA_WALLET_SERVICES_ARC_TOKEN` | Overrides mainnet ARC token in yaml |
| `INFRA_WALLET_SERVICES_WHATS_ON_CHAIN_API_KEY` | Optional WOC API key |
| `POSTGRES_PASSWORD` | Defaults to `postgres` (local compose only) |

## Safety checklist

- [ ] You intend to use **mainnet** and accept fee burn (~denomination sats per successful action plus fan-out overhead).
- [ ] `PRIVATE_KEY` is funded and not checked into git.
- [ ] UI is only exposed on localhost (compose publishes `127.0.0.1:8200`).
- [ ] Storage RPC on the host is localhost-only (`127.0.0.1:8101`); Postgres is `127.0.0.1:5433`.
- [ ] Start the stream at low TPS first; confirm fuel inventory is rising after funding.
- [ ] ARC / indexer credentials are valid for mainnet if you need reliable broadcast and internalize-by-txid.
- [ ] You know how to stop the stream and `docker compose ... down` before leaving the demo unattended.

## API sketch

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/status` | Network, last tick, recent events |
| `POST` | `/api/stream/start` | `{ "tps"?, "workers"? }` |
| `POST` | `/api/stream/stop` | Stop stream (FuelKeeper keeps running) |
| `GET` | `/api/events` | SSE: `tick`, `topup` |
| `GET` | `/api/funding` | Deposit address + locking script hex |
| `POST` | `/api/funding/internalize` | `{ atomic_tx_hex? , txid? , output_index? }` |

## Packaging files

| File | Purpose |
|---|---|
| `Dockerfile.dashboard` | Builds `infra_throughput` + `throughput_dashboard` |
| `docker-compose.throughput-dashboard.yaml` | db + infra + dashboard |
| `infra-config-docker-throughput-mainnet.yaml` | Mainnet throughput server config |

Validate compose interpolation without starting containers:

```bash
PRIVATE_KEY=00 docker compose -f docker-compose.throughput-dashboard.yaml config
```
