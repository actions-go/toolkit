# Goal: automated Go vs JavaScript parity tests

Objective (verbatim): "Find a way to compare the behaviour of this toolkit with the reference actions/toolkit repo. We want to have the same behaviour to ensure feature parity. Your goal is to have those automated tests comparing go and javascript implementations working and merged on main."

## Approach

Differential testing. One set of JSON scenarios (env + ordered calls) is run through two drivers:

- Go driver: this module (`core` package), run in-process by `go test`.
- JS driver: the real `@actions/core` from npm, pinned in `parity/js/package.json`, run with node.

Both record stdout (workflow commands), the contents of the `GITHUB_OUTPUT`/`ENV`/`PATH`/`STATE`/`STEP_SUMMARY` files, return values and thrown errors. The test fails on any difference, except differences listed in `parity/known-divergences.json` (each with a reason), so a divergence is either fixed or written down.

## Sub-goals and KPIs

| # | Sub-goal | KPI (command) |
|---|---|---|
| 1 | Harness runs both implementations on one scenario file | `go test ./parity/...` exits 0 |
| 2 | Scenarios cover the `core` surface that exists in both | scenario count per function, printed by the test |
| 3 | Every divergence found is fixed in Go or recorded with a reason | `parity/known-divergences.json` entries all have `reason` |
| 4 | Runs in CI on every push/PR | `parity` job green on the PR |
| 5 | Merged on main | `git log origin/main` contains the commit |

## Assumptions

- npm registry reachable from sandbox and CI: validated (`npm view @actions/core version` -> 3.0.1).
- `@actions/core` loads under node 22 (it is ESM): assumed.
- Scope is `core` first; `github` and `cache` need network/service mocks and are a follow-up: decided, not validated.
- Repo workflow here is trunk-based via `git review`; the session instructions designate branch `claude/dreamy-mccarthy-smrie4`. Merge path to main: assumed to be a PR, unresolved (see Open).

## Open

- Whether to open a PR and merge it myself, or push to main. Pushing to the designated branch only is the default; creating a PR needs the user's explicit ask.

## Log

- M1 harness + 55 `core` scenarios: first run found 23 failing scenarios. Fixed in Go: trailing blank line in file commands, message over-escaping (`:` and `,` escaped in messages), random annotation property order, `AddPath` not updating `PATH`, extra warnings/debug lines that `@actions/core` does not emit. Remaining 4 are API-shape differences (see `parity/known-divergences.json`).
- Mistake worth keeping: the JS driver exited 1 with empty stderr because `core.setFailed` sets `process.exitCode`. The Go package keeps its failed status private, so exit status is not compared; that is a parity gap.
- Pre-existing, unrelated failures on main: `cache` `TestEnsureDestDir` (running as root) and `github` `TestClient` (needs `GITHUB_TOKEN`).
- Not covered yet: `summary`, `getIDToken`, `group`, `platform`, `github` and `cache` packages.

## Running locally

```bash
npm ci --prefix parity/js
go test -v ./parity/...
```
