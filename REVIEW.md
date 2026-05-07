# Review: 428eb307

Verdict: needs work.

## Finding

Medium: tests do not prove the new worker-plane behavior.

`internal/worker/spawn.go`, `internal/review/review.go`, and `internal/planreview/review.go` now pass `smallModel=true`, but the tests only check that the command contains `lenos --agent ...` and `ttal context`. They never assert `--small-model`, so a regression that changes those call sites back to `false` would likely still pass.

Please add positive assertions for `--small-model` in the lenos branch tests:

- `internal/review/review_test.go`
- `internal/planreview/review_test.go`
- ideally one `BuildLenosCommand(... smallModel=true ...)` unit test in `internal/launchcmd/launchcmd_test.go`

## Notes

I do not see a functional bug in the implementation itself. Manager-plane `spawnAgentSession` passes `smallModel=false`; worker, PR reviewer, and plan reviewer paths pass `true`.

Verification run passed:

```bash
go test ./internal/launchcmd ./internal/worker ./internal/review ./internal/planreview ./internal/daemon
```
