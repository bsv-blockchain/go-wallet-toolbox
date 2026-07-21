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

## Quick start (Docker)

```bash
# Operator wallet private key (hex) — never commit this
export PRIVATE_KEY=<hex-private-key>

# Recommended for reliable mainnet broadcast:
# export INFRA_WALLET_SERVICES_ARC_TOKEN=<mainnet-arc-token>
# export INFRA_WALLET_SERVICES_WHATS_ON_CHAIN_API_KEY=<optional>

docker compose -f docker-compose.throughput-dashboard.yaml up --build
```

Open **http://127.0.0.1:8200**

| Service | Host access |
|---|---|
| Dashboard UI | `http://127.0.0.1:8200` |
| Storage RPC | `http://127.0.0.1:8101` |
| Postgres | `127.0.0.1:5433` |

## Local run (infra already up)

If the throughput infra is already listening on `8101`:

```bash
export PRIVATE_KEY=<hex>
export SERVER_URL=http://127.0.0.1:8101
export BSV_NETWORK=main
go run ./cmd/throughput_dashboard
```

## Funding with WalletClient

1. Install / unlock a MetaNet-compatible browser wallet that implements `@bsv/sdk` `WalletClient`.
2. On the **Fund operator** panel, set satoshis (default 100 000) and click **Pay with WalletClient**.
3. The UI pays the operator BRC-29 deposit address, then `POST /api/funding/internalize` credits the **default** basket.
4. FuelKeeper (every ~10s when below low water) fans default → reserve chunks → exact-denomination fuel.

Until fuel is available, createActions may fall back to slower default-basket funding or fail with not-enough-funds — both are counted in the UI.

## Stream payload

Each createAction has a single 0-sat output:

```
OP_RETURN <32-byte sha256( strconv(iteration) + time.RFC3339Nano )>
```

`AcceptDelayedBroadcast` is enabled for high-rate demos.

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

## Safety checklist

- [ ] You intend to use **mainnet** and accept fee burn (~denomination sats per successful action plus fan-out overhead).
- [ ] `PRIVATE_KEY` is funded and not checked into git.
- [ ] UI is only exposed on localhost (compose publishes `127.0.0.1:8200`).
- [ ] Start the stream at low TPS first; confirm fuel inventory is rising after funding.
- [ ] ARC / indexer credentials are valid for mainnet if you need reliable broadcast and internalize-by-txid.

## API sketch

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/status` | Network, last tick, recent events |
| `POST` | `/api/stream/start` | `{ "tps"?, "workers"? }` |
| `POST` | `/api/stream/stop` | Stop stream (FuelKeeper keeps running) |
| `GET` | `/api/events` | SSE: `tick`, `topup` |
| `GET` | `/api/funding` | Deposit address + locking script hex |
| `POST` | `/api/funding/internalize` | `{ atomic_tx_hex? , txid? , output_index? }` |

## Denomination

Dashboard and server share the live-test OP_RETURN profile:

- `expected_tx_size_bytes: 200`, fee `100 sat/kb`, `expected_output_satoshis: 0`
- Derived denomination **20 sats** (must match FuelKeeper leaf shape)

See `config.DemoThroughput()` and `infra-config-docker-throughput-mainnet.yaml`.
