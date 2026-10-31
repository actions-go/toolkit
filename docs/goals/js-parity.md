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

## State at the end (all merged to main)

| Area | Reference | Scenarios | Known divergences | Runs on |
|---|---|---|---|---|
| `core` | @actions/core 3.0.1 | 291 | 9 | Linux, macOS, Windows |
| `github` | @actions/github 9.1.1 | 70 | 7 | Linux, macOS, Windows |
| `cache` | @actions/tool-cache 4.0.0 | 363 | 16 | Linux, macOS, Windows (with the skips below) |

Narrowed on purpose after the first macOS and Windows runs:
- On Windows the `cache` suite skips the scenarios that compare Unix permission bits or symbolic links (`requires: ["unix"]`, 8 scenarios: Windows has no mode bits, symlink creation needs privileges, and `io.cp` recreates links as junctions with absolute targets) and one that needs an OS version (`requires: ["osversion"]`: the reference reports none on Windows). Path separators are compared as `/` there.
- tar and zip extraction scenarios (`requires: ["posix"]`) run on Linux only. The Go extractors reproduce GNU tar and Info-ZIP unzip; macOS ships bsdtar and another unzip build (other listings, differently normalised Unicode names). macOS still runs the manifest, version and cache scenarios.
- `extract7z` is supported by the reference on Windows only and `extractXar` on macOS only: Linux compares just the unsupported-OS errors, the xar scenarios run on macOS, and no 7z scenario runs anywhere (the reference needs its bundled 7zdec on Windows, which the port does not bundle).

`platform getDetails without PATH` runs on all three systems now. The Windows difference was not about PATH: both sides fail with "Unable to locate executable file: powershell", but Go printed the Unix hint ("Also check the file mode...") where `@actions/io` prints the Windows one ("Also verify the file has a valid extension..."). Fixed in `core/platform.go` and `cache/extract.go`. The same Windows run also showed that `CacheFile`'s `..` hardening split targets on the OS separator only and did not work on Windows.

Accepted gap (decided with the user): how an executable is looked up. The error text for a missing executable matches `@actions/io`, but the lookup itself uses Go's `exec.LookPath` (in `core/platform.go` and `cache/extract.go`) and differs from `findInPath` / `tryGetExecutablePath`:
- Unix: `@actions/io` accepts a file only if an execute bit applies to the current user (other; group with a matching gid; owner with a matching uid). `LookPath` accepts any execute bit.
- Windows: `@actions/io` tries the bare name and then the name plus each `PATHEXT` entry, and tries no extension when `PATHEXT` is unset; it restores the file's real letter case. `LookPath` has a built-in default extension list and does not restore case.
- A name with a path separator that is not an absolute path: `@actions/io` finds nothing; `LookPath` tries it relative to the current directory.
- Empty `PATH` entries: skipped by `@actions/io`; `LookPath` treats them as the current directory.
- The current directory is never searched by `@actions/io`; `LookPath` on Windows has its own rule for it.
- The returned path is the `PATH` directory joined with the name; our `which` in `cache` makes it absolute.
None of the parity scenarios exercises these cases (they cover the error text and an unset `PATH`). If this ever matters: port `findInPath` into one internal function used by both packages and add scenarios that call `@actions/io`'s `which` directly; Windows behaviour can only be proven on the Windows CI leg.

Not covered, by design: xz/zstd tar archives (no standard library decoder; Go returns an explicit "unsupported" error), `downloadTool` auth/headers (no Go API), and the Go-only helpers (`DownloadSelectedRepositoryFiles`, `MatchesOneOf`, `core/artifacts.go`).

Tar, zip and manifest hang trap: a reference call that never finishes (a tar waiting on stdin under bsdtar) hung the macOS job for the full 10 minute `go test` timeout with no scenario named. The cache driver now fails any reference call after 60 seconds, naming the scenario.

## Extract7z and ExtractXar (third round)

- The reference limits both: `extract7z` asserts `IS_WINDOWS` and `extractXar` asserts `IS_MAC`, before it looks at its arguments. Linux can only compare the unsupported-OS errors and "no destination folder is created"; the xar scenarios (`requires: ["darwin", "xar"]`) run on macOS only, and no 7z scenario runs anywhere until the cache suite runs on Windows.
- `Extract7z` without a path looks for `7z`/`7zr` in PATH; the reference runs a bundled PowerShell script (`Invoke-7zdec.ps1`) and the port bundles nothing.
- Scenarios that could not run in the sandbox (no macOS, no `xar`) were only proven by CI. Expect the first run of any such scenario to find something; this one found two things:
  - the node driver runs every scenario in one process, so a scenario that sets `PATH` (the "no xar in PATH" ones) broke the later `_getOsVersion` scenario ("sw_vers: No such file"); the driver now restores its starting environment before each scenario;
  - `@actions/exec` writes a failing tool's stderr into the same log stream as stdout, so the Go port now sends the child's stderr to stdout too.
- A mistake worth remembering: `scripts/verify | tail -3 && git commit` committed a failing tree, because the pipeline's exit status is `tail`'s. The driver change used a `const` declared after the loop that calls it (temporal dead zone). Redirect the output to a file and test the exit status, then read the file.

## Running locally

```bash
npm ci --prefix parity/js
go test -v ./parity/...
```
