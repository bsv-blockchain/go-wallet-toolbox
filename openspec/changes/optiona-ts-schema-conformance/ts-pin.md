# ts-stack pin

This change is anchored to the TS schema at a specific commit. All conformance work targets this commit until a refresh is performed.

## Pinned commit

- Repo: `bsv-blockchain/ts-stack`
- Branch: `main`
- Commit: TBD — pin HEAD of `main` at Phase 0 start
- Path: `packages/wallet/wallet-toolbox/src/storage/schema/KnexMigrations.ts`
- Date pinned: TBD

## Pin policy

- Pin is `main` HEAD at Phase 0 start.
- One refresh per phase is permitted to absorb upstream schema deltas.
- Final refresh in Phase 14 before archive.
- If a refresh introduces breaking diff, halt phase and reassess proposal.

## Refresh procedure

1. Check `ts-stack` main branch for schema changes since the pinned commit.
2. If changes are non-trivial, open a sub-change against this OpenSpec change.
3. If changes are minor, update the pin commit and re-run Phase 14 validation.

## Refresh log

| Date | Commit | Reason |
|------|--------|--------|
| TBD | TBD | Initial pin |
