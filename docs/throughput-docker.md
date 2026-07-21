# Throughput Docker Live-Test Stack

Two Docker Compose stacks live in this repo:

| Stack | Compose file | UTXO strategy | Host ports |
|---|---|---|---|
| **Standard (privacy)** | `docker-compose.yaml` | `privacy` | HTTP `8100`, Postgres `5432` |
| **Throughput live-test** | `docker-compose.throughput.yaml` | `throughput` | HTTP `8101`, Postgres `5433` |

Host ports differ so both stacks can run side-by-side.

## Standard stack (privacy)

Default development stack. Uses `Dockerfile` + `infra-config-docker.yaml`
with explicit `utxo_management.strategy: privacy`.

```bash
docker compose up -d --build
```

- Storage server: `http://localhost:8100`
- Postgres: `127.0.0.1:5432`

## Throughput stack (live-test)

Sized for a **1000 TPS** OP_RETURN live test:

| Setting | Value |
|---|---|
| Fee model | `sat/kb` value `100` |
| Expected tx size | `200` bytes |
| Expected output satoshis | `0` (data-only OP_RETURN) |
| Target TPS | `1000` |
| OP_RETURN payload | `the proof is in the pudding` |

### Prerequisites

1. **Funded wallet private key** — loadgen signs createActions; set via `PRIVATE_KEY` (hex). Never commit keys.
2. **Optional faucet bootstrap** — set `FAUCET_TXID` so loadgen can internalize a faucet payment into the wallet before FuelKeeper warm-up.
3. **FuelKeeper warm-up** — loadgen starts FuelKeeper and waits `WARMUP_SECONDS` (default `5`) before the rate loop. Sustained 1000 TPS needs a funded reserve and time for fuel pool top-up; increase warm-up or pre-fund if the pool is empty.

### Run

```bash
export PRIVATE_KEY=<hex-private-key>
# optional:
# export FAUCET_TXID=<txid>
# export TPS=1000 WORKERS=64 WARMUP_SECONDS=30 DURATION_SECONDS=60

docker compose -f docker-compose.throughput.yaml up --build
```

Services:

| Service | Role |
|---|---|
| `db` | Postgres 17 (volume `db-data-throughput`) |
| `infra` | `cmd/infra_throughput` — loads `infra-config-throughput.yaml` |
| `loadgen` | `cmd/throughput_loadgen` — rate-limited createAction loop |

- Storage server (host): `http://localhost:8101`
- Postgres (host): `127.0.0.1:5433`
- Inside the compose network, loadgen talks to `http://infra:8100`

`loadgen` exits when `DURATION_SECONDS` elapses (or on SIGINT/SIGTERM). Use
`DURATION_SECONDS=0` (default) to run until interrupted. `restart: "no"` keeps
compose from restarting a finished or failed loadgen.

### Loadgen environment

| Variable | Default | Notes |
|---|---|---|
| `PRIVATE_KEY` | *(required)* | Wallet key (hex) |
| `SERVER_URL` | `http://infra:8100` | Storage server URL |
| `BSV_NETWORK` | `test` | Network name |
| `TPS` | `1000` | Target createActions per second |
| `WORKERS` | `64` | Concurrent workers |
| `FAUCET_TXID` | empty | Optional faucet tx to internalize |
| `WARMUP_SECONDS` | `5` | FuelKeeper warm-up before load |
| `DURATION_SECONDS` | `0` | `0` = run until stop |

### Notes

- Loadgen issues createActions with a single OP_RETURN output whose data is
  exactly `the proof is in the pudding`.
- FuelKeeper runs in the loadgen process (client-side signing); the server
  never holds operator keys.
- For sustained 1000 TPS, ensure the wallet is funded and allow enough warm-up
  for fan-out into the `fuel` basket before measuring steady-state rate.
- Throughput mode design: `examples/throughput_mode/throughput-mode.md` and
  `docs/superpowers/specs/2026-07-20-throughput-docker-live-test-design.md`.
- **Mainnet demo dashboard** (start/stop stream UI, WalletClient funding):  
  [`docs/throughput-dashboard.md`](throughput-dashboard.md) and  
  `docker compose -f docker-compose.throughput-dashboard.yaml up --build`.
