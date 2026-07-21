# Task 1 — Stream package harden

## Owns exclusively
`cmd/throughput_dashboard/internal/stream/**` only. Do not edit other packages.

## Goal
Harden the controllable createAction event stream: hashed OP_RETURN + Start/Stop controller + solid tests.

## Requirements
1. `HashPayload(iteration, ts)` returns `sha256(strconv.FormatUint(iteration,10) + ts.UTC().Format(time.RFC3339Nano))` (32 bytes).
2. `OpReturnLockingScriptForHash` / `OpReturnLockingScriptForIteration` build OP_RETURN locking scripts pushing the hash.
3. `Controller` with Start/Stop/Running/Stats; rate-limited producer; worker pool; each action unique hash/iteration.
4. `CreateAction` options: `AcceptDelayedBroadcast: true`, satoshis 0, single OP_RETURN output.
5. Tests: hash determinism; OP_RETURN contains hash; start/stop; failure counting. **Do not** require Succeeded==Attempted after Stop (cancel may fail in-flight).
6. `go test ./cmd/throughput_dashboard/internal/stream/... -count=1` must PASS.

## Baseline
Code already exists from scaffold commit `4758c367`. Review, fix gaps, improve tests, commit improvements.

## Commit
`feat(dashboard): harden stream controller and hashed opreturn`

## Report
Write full report to `.superpowers/sdd/task-1-report.md` in the worktree.
Return status DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT, commits, test summary.
