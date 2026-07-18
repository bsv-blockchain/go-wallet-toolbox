# Throughput Mode — Operator Runbook

> **Status:** This example tracks the design proposal in
> [`plans/high-throughput-utxo-management.md`](../../plans/high-throughput-utxo-management.md)
> (PR #936). The Go file is excluded from builds via the `throughput_example`
> build tag until the `utxo_management` configuration lands (Phases 1–3 of the
> proposal); this runbook documents the intended operator flow so it can be
> reviewed alongside the design.

This example shows how to set up and run a wallet server for sustained
high-volume, single-operator workloads — the motivating profile is
repo-market settlement: ~10 createActions/s on average, with two diurnal
peaks (market open/close) reaching ~160,000 outputs/s as packed many-output
actions.

## Overview

1. **Configure** `utxo_management.strategy: throughput`: a pool of
   exact-denomination "fuel" UTXOs sized to your typical action shape, held in
   a dedicated `fuel` basket.
2. **Fund** the `reserve` basket. The `pool_top_up` monitor task fans reserve
   UTXOs out into fuel (reserve → chunks → 240-sat outputs) and keeps the pool
   between its low/high water marks. Fuel matures to block inclusion in ~5
   minutes and is then claimable under the default `prefer_mined` policy.
3. **Issue packed actions.** There is no batch endpoint — actions are atomic
   per txid. Batch at the application layer by defining many outputs per
   action: a typical 15-output action funds with **one** 240-sat fuel claim;
   a 1,000-output peak action multi-claims automatically.
4. **Monitor externally.** The wallet emits OpenTelemetry metrics and traces
   over one OTLP endpoint; your monitoring stack owns thresholds and paging.
   There is no in-process alerting.

## Sizing your configuration

| Input | Repo-market value | Notes |
|---|---|---|
| Typical action | 15 outputs ≈ 668 B signed | includes the single fuel input (148 B) |
| Denomination | **240 sat** | 67-sat fee + ~170 sat of output value; must exceed the ~15-sat marginal input fee (throughput-mode dust rule) |
| Peak fuel-claim rate | ~10,000/s | 160 actions/s × ~64 claims for 1,000-output actions |
| Pool target | ~18 M | one 15-min peak ≈ 9 M fuel; two daily peaks with headroom; refilled between peaks |
| Fan-out round | ≥ `target_tps × interval × 1.2` outputs | defaults: 100 outputs × 12,000 leaf txs per 10 s round |

Derivation rules, validation identities, and the worked math are in the
proposal §4 and §6.

## Reserve funding

Deposit large UTXOs into the `reserve` basket with an ordinary
basket-insertion `internalizeAction` (see
[internalize_tx_from_faucet](../wallet_examples/internalize_tx_from_faucet/internalize_tx_from_faucet.md)
for the mechanics — the only difference is `basket: "reserve"`). Only the
top-up task spends reserve; the funding paths cannot touch it.

Plan reserve depth against burn: at the repo profile, average fuel burn is
~2.1 BSV/day, plus ~15% fan-out fee overhead and burst burn during the two
peaks. The `wallet.utxo.reserve.runway_seconds` gauge reports how long the
reserve lasts at the configured rated load.

## Observability

### Collector

Point `tracing.dialAddr` (and the metrics exporter, which shares the
endpoint) at an OpenTelemetry Collector:

```yaml
# otel-collector.yaml (minimal)
receivers:
  otlp:
    protocols:
      grpc:            # :4317
exporters:
  prometheus:
    endpoint: 0.0.0.0:9464
service:
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [prometheus]
    traces:
      receivers: [otlp]
      exporters: []    # add your tracing backend (Tempo/Jaeger/…)
```

### Key instruments

| Instrument | Type | Meaning |
|---|---|---|
| `wallet.utxo.pool.spendable{basket,status,denomination}` | gauge | claimable fuel, per status tier |
| `wallet.utxo.pool.immature` | gauge | minted, awaiting block inclusion |
| `wallet.utxo.pool.stale` | gauge | old-denomination fuel awaiting consolidation |
| `wallet.utxo.pool.runway_seconds` | gauge | seconds until pool exhaustion at rated load |
| `wallet.utxo.reserve.balance_satoshis` / `wallet.utxo.reserve.runway_seconds` | gauge | reserve depth / time until empty |
| `wallet.funder.claims{result}` | counter | `exact_match` / `multi_claim` / `fallback` |
| `wallet.funder.not_enough_funds` | counter | funding failures at the API surface |
| `wallet.topup.outputs_minted{denomination}` / `wallet.topup.rounds{result}` | counter | replenishment progress |
| `wallet.topup.consecutive_failures` | gauge | rounds failed in a row |
| `wallet.funder.fund_duration` / `wallet.topup.round_duration` | histogram | latency profiles |

### Low-funds alert rules (Prometheus)

```yaml
groups:
  - name: wallet-throughput
    rules:
      - alert: PoolRunwayLow
        expr: wallet_utxo_pool_runway_seconds < 900
        for: 1m
        labels: { severity: critical }
        annotations:
          summary: "Fuel pool below 15 minutes of runway at rated load"
      - alert: ReserveRunwayLow
        expr: wallet_utxo_reserve_runway_seconds < 86400
        for: 5m
        labels: { severity: warning }
        annotations:
          summary: "Reserve funds below 1 day of fee burn — deposit soon"
      - alert: TopUpFailing
        expr: wallet_topup_consecutive_failures >= 3
        labels: { severity: critical }
        annotations:
          summary: "pool_top_up has failed 3+ consecutive rounds"
      - alert: FundingErrors
        expr: increase(wallet_funder_not_enough_funds[1m]) > 0
        labels: { severity: critical }
        annotations:
          summary: "createAction failing with not-enough-funds"
      - alert: StaleFuelAccumulating
        expr: wallet_utxo_pool_stale > 1000000
        for: 30m
        labels: { severity: warning }
        annotations:
          summary: "Stale-denomination fuel not being consolidated"
```

Route these through Alertmanager to whatever pages you (Slack, PagerDuty,
SMS gateway) — notification transport is deliberately outside the wallet.

## Running

```bash
# once the feature lands, drop the build tag and:
go run -tags throughput_example ./examples/throughput_mode
```

The server boots, creates the `fuel`/`reserve` baskets, and the top-up task
starts filling the pool as soon as the reserve is funded. Until the first
fuel matures (~5 min), createActions fund via the generic fallback path
against the `default` basket — slower but correct.

## Safety properties worth knowing

- A drained pool degrades to the fallback path, never to wrong behavior; the
  runway gauges give you the paging lead time to prevent it.
- Rolling back to `strategy: privacy` is a zero-cost basket relabel — no
  on-chain sweep required.
- Throughput mode makes the operator's transaction graph fully linkable by
  design. It is for single-operator infrastructure funds, never custodial
  end-user funds.
