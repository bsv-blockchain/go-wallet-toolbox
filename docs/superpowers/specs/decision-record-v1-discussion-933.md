# Decision Record v1 — Plan of Record

**Source:** [GitHub Discussion #933](https://github.com/bsv-blockchain/go-wallet-toolbox/discussions/933)  
**Frozen:** 2026-07-16 (quorum round 5/5, 0 BLOCK)  
**Status:** Plan of record for robustness + performance work

---

## Policy (locked)

| # | Decision | Value |
|---|----------|--------|
| P1 | Accepted | Multi-source knowledge: SSE `SEEN_ON_MULTIPLE_NODES` preferred; else multi-provider `GetStatusForTxIDs` agreement; metric `acceptance_degraded_no_sse` |
| P2 | Confirmed | Merkle / block announcement + `BlocksDelay` |
| P3 | HighAssurance | Default **ON** |
| P4 | DS/invalid input release | Quarantine; **no auto-release** under HA; explicit release + evidence only. **AbortAction is an input-release path** → evidence-gated |
| P5 | ServiceError / thin success → change FREE | **No** under HA |
| P6 | Reorg | Demote Tx+UTXO to **unproven/unconfirmed**, outputs **remain spendable** at unproven tier; durable event; auto re-proof; no human page. Competing evidence / failed re-proof → **quarantine** (P4) |
| P7 | Topology | Monitor + services on **every instance**; DB leases/locks for correctness |
| P8 | Option B / 140k | **Track P** parallel; merge only if W6 + SLO gates pass; does not block Track S |
| P9 | `includeSending` under HA | **Off** |

## Opens (locked)

| ID | Resolution |
|----|------------|
| O1 | Multi-provider status agreement suffices; SSE preferred; degraded metric |
| O2 | W3a locks+leases+cursor → W3b durable outbox |
| O3 | Per-user advisory lock optional, default on under HA |

## Track S — Safety (main, sequential)

| Wave | Scope |
|------|--------|
| W0 | Postgres CI + concurrency harness |
| W1 | CAS + truthfulness + Abort evidence gate + UserUTXO BasketName index tag fix |
| W2 | Fund rules + HA (ServiceError not spendable; DS quarantine; reorg demote; CompetingEvidence) |
| W3a | Stable job locks, row leases, resumable cursors, clean watermark |
| W3b | Durable event outbox |
| W4 | Lifecycle phase+version; UserUTXO projection or versioned |

## Track P — Performance (parallel)

- Document topology: N workers × rate, hardware, spike duration/ramp
- Bounded O(k) UTXO selection; async broadcast; contention retry/queue
- Hot-wallet pre-fragmentation; Option B only under gates

## W6 hard gates

- Concurrent CreateAction correctness
- Crash between PostBEEF and apply → never freed inputs
- Zero application fund/state errors (incl. contention misreported as insufficient funds)
- 140k spike capacity on declared topology; 99.9% soak

## Implementation start order

1. W0 Postgres CI harness  
2. W1: provided-input RowsAffected → UnFail → Abort gate → status CAS → index tag → predicates → SendWaiting  
3. W2 ServiceError not spendable  
4. W3a lock key + leases  
5. Track P benchmark skeleton after W0  

## Quorum

- Product answers Q1–Q4 + 140k SLO incorporated  
- 3 agent ACKs on v0.1; 2 explicit ACKs on v1; **0 BLOCK**  
- Discussion plan-churn closed unless a locked clause is invalidated by new evidence  
