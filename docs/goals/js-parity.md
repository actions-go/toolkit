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

## Phase 2: cover everything (goal: "suite merged on main should cover everything")

M1 (merged, #94) covered the simple `core` API. Phase 2 sub-goals, built in parallel in separate worktrees with disjoint files:

| Area | Reference | Location |
|---|---|---|
| rest of `core` (summary, group, platform, getIDToken, stop/startCommands, edge cases) | @actions/core 3.0.1 | `parity/` |
| `github` (Context, user agent, client defaults) | @actions/github 9.1.1 | `parity/github/` |
| `cache` (port of tool-cache) | @actions/tool-cache 4.0.0 | `parity/cache/` |

Out of scope by construction: Go-only helpers with no JS equivalent (`DownloadSelectedRepositoryFiles`, `MatchesOneOf`, `core/artifacts.go`, actions-go JSON inputs). The package name `cache` is tool-cache, not @actions/cache.

## What phase 2 taught (not visible in the diffs)

Results: `core` 291 scenarios, `github` 70, `cache` 217. Each comparison found real Go bugs the existing unit tests had encoded as expected behaviour (the old platform tests asserted constants against themselves).

- **The OS matrix found a bug class Linux cannot show.** Go ended commands, file commands and `Info` with `\n` (`Println`); the reference uses `os.EOL`. 170 of 291 `core` scenarios failed on the first Windows run, and `github` printed one `\n` message too. Fixed by writing the `EOF` constant. Unit tests that hard-coded `\n` (`TestIssueCommand`, summary, artifacts) then failed on Windows: after a line-ending change, grep the tests for `\n` too.
- **Windows-only scenario skipped without analysis:** `platform getDetails without PATH` (`skipOS` in the scenario, reason printed). The cause is probably how PowerShell is found without PATH; nobody looked.
- **Decision kept on purpose:** `AddPath` with no `PATH` gives `<dir>` in Go; the reference gives `<dir>:undefined`. Documented in `parity/known-divergences.json`.
- **Public-API changes the comparison forced:** `CacheFile` on a directory now errors; the `github` client sends `Authorization: token` instead of `Bearer`; `GITHUB_REPOSITORY` wins over `payload.repository`; multi-line file commands use a random delimiter (the constant one let a value forge extra outputs).
- **Harness traps.** `core.setFailed` sets `process.exitCode`, so the JS driver exited 1 with empty stderr until the exit code was recorded (`failed`) and reset per scenario. A harness reporting nothing looked like a harness bug, not a reference behaviour. `stopCommands`/`startCommands` are not exported by `@actions/core` 3.0.1; the driver reaches `lib/command.js` by path, which ties it to that version layout.
- **Delegation traps.** Worktree agents were created from the branch's remote state, not my local HEAD, so two reported the npm dependency missing even though I had committed it; commit dependencies before spawning, and push them. Agents must not share a file: one package dir plus one driver file per area kept merges trivial.
- **GitHub traps.** After one PR merged, GitHub rebased the other open branches onto main: pushes failed with stale info, merges with 409 "Head branch was modified", and `expectedHeadSha` went stale. Re-read the PR head before merging. A `check_suite.completed` event for an old head says nothing about the new one. Force-push to a branch is denied by policy here; use a fresh branch and a new PR. `mergeable_state: blocked` is shown even when the merge then succeeds.
- **Cost trap.** `get_job_logs` on a Go test job returns every PASS line; `tail_lines` of a few hundred still costs tens of thousands of tokens. The FAIL list is near the end only for small suites. Prefer logging failures only, or a `-run` filter.
- **CI queue.** Runner capacity was the slow part; most "waiting" was queued jobs, not failures.

Not covered, by design: `extract7z`/`extractXar` (need external binaries), `downloadTool` auth/headers (no Go API). Tar, zip and the manifest functions are being added in a separate PR.

## Open at the end of the session (hand-off)

- **PR #103 (tar/zip/manifest) is red on CI, not merged.** All `getManifestFromRepo` scenarios fail when the proxy environment variables are unset: the reference answers `Protocol "http:" not supported. Expected "https:"` and Go returns the manifest. They pass in the agent sandbox only because `HTTPS_PROXY` is set there, which changes the agent http-client builds. Reproduce with `env -u HTTPS_PROXY -u https_proxy -u HTTP_PROXY -u http_proxy -u ALL_PROXY -u all_proxy PARITY_REQUIRED=1 go test ./parity/cache/`. The `https.request` patch in `parity/js/toolcache-driver.mjs` (it redirects api.github.com to the local server) is the suspect; making the options plain `http:` was tried and did not fix it. Next step: log the options the reference passes to `https.request` without a proxy, or serve the local server over https with a throwaway certificate and `NODE_EXTRA_CA_CERTS` instead of patching.
- **PR #102 (exit status, OS matrix, Windows EOL fixes)**: Windows failed in `parity/cache` on directory modes (`0666` vs `0777`); fixed in `9cfae17` (modes not compared on Windows). That run, and the macOS/Windows runs of #103's tar/zip scenarios, had not reported when the session ended.
- Lesson: run the parity suites once with the proxy variables unset before pushing; CI has none and the sandbox does.

## Running locally

```bash
npm ci --prefix parity/js
go test -v ./parity/...
```
