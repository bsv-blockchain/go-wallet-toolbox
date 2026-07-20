# ts-stack pin

This change is anchored to the TS schema at a specific commit. All conformance work targets this commit until a refresh is performed.

## Pinned commit

- Repo: `bsv-blockchain/ts-stack`
- Branch: `main`
- Commit: `7a840ff97e1f685f778210818933e6da0dac22c2` (HEAD of `main` at Wave 0 start)
- Path: `packages/wallet/wallet-toolbox/src/storage/schema/KnexMigrations.ts`
- Date pinned: 2026-07-06
- Local checkout used for extraction: `/Users/pawellewandowski/gignative/bitcoin-sv/toolbox/ts-stack`

## Schema-pin vs conformance-vector-pin (MUST reconcile in Wave 0)

The schema is pinned at `7a840ff97e1f685f778210818933e6da0dac22c2` (above). The vendored conformance vectors are pinned at a **different, older** commit:

- `conformance/SOURCE` → `upstream_sha=1920a9c1b34010ff7ede34e424f0fca19b2ed3e6` (`fetched_at=2026-05-14`).

These must agree, or conformance will validate Go against a schema that differs from the one we built to. **Wave 0 action:** run `./conformance/scripts/refresh-vectors.sh 7a840ff97e1f685f778210818933e6da0dac22c2` and triage any new vector deltas. If a refresh would pull in schema changes beyond what this change implements, halt and reassess (per refresh policy below) instead of silently re-pinning.

## Pin policy

- Pin is `main` HEAD at Wave 0 start (`7a840ff97e1f685f778210818933e6da0dac22c2`).
- One refresh per wave is permitted to absorb upstream schema deltas.
- Final refresh in Wave 4 before archive.
- If a refresh introduces a breaking diff, halt the wave and reassess the proposal.

## Refresh procedure

1. Check `ts-stack` main for schema changes since the pinned commit.
2. If changes are non-trivial, open a sub-change against this OpenSpec change.
3. If changes are minor, update the pin commit + `conformance/SOURCE` and re-run the Wave 3 conformance gate.

## Refresh log

| Date       | Commit       | Reason       |
|------------|--------------|--------------|
| 2026-06-08 | `179d88c61`  | Initial pin  |
| 2026-07-06 | `7a840ff97e1f685f778210818933e6da0dac22c2` | Refresh — schema unchanged (KnexMigrations.ts identical) |
