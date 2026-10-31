# Goal: merge open dependabot PRs, all dependencies current on main

Objective (verbatim): check all dependabot PRs. rebase and solve conflicts. merge remaining dependabot PRs and get all dependencies up to date in main.

## Sub-goals and KPIs
| # | Sub-goal | KPI (command) | Target |
|---|---|---|---|
| 1 | No open dependabot PRs | `list_pull_requests state=open` | 0 (#85, #87, #88 at start) |
| 2 | Go modules current | `go list -m -u all` | no upgrades listed |
| 3 | Actions current | `.github/workflows/ci.yml` pins vs latest tags | all latest |
| 4 | Checks green | `scripts/verify` | exit 0 |

## Plan
1. Rebase #85, #87, #88 onto main (main moved: #89 merged), resolve go.mod/go.sum conflicts, merge each once CI is green.
2. Run `go list -m -u all`; bump what is left (indirect deps included) in one commit.
3. Bump any remaining workflow pins.

## Assumptions
- Merging dependabot PRs is authorised by the objective. `assumed`
- CI needs a GITHUB_TOKEN for cache/ and github/ tests; those may not run locally. `assumed`

## Policy change mid-run
The human set the support window to the last 4 Go releases (1.23-1.26). That caps every dependency at the newest version whose go.mod needs Go <= 1.23, so "latest" below means "latest allowed". My first pass assumed 3 releases and was redone.

## Log
- 2026-10-31 #85 (setup-go 5.0.0) and #87 (x/crypto) merged after green CI. `@dependabot` comments written by Claude never fired (the tool rewrote the mention); the human's did. #87 was recreated by Dependabot at 0.52.0.
- #88 (x/oauth2 0.16) merged by the human before #100 landed.
- #100 merged: go 1.23.0 in go.mod, CI `GOTOOLCHAIN=local`, go-github v42 -> v74, x/oauth2 0.30, semver 3.5, testify 1.12.1. Cost: v74 sends `X-GitHub-Api-Version`, which failed the github parity test until that header was ignored there.
- A rebase onto main failed silently once: `scripts/verify | tail` hid the exit code, and main had gained a v42 import in `parity/github`. Run verify without a pipe.

## KPI outcome
1. Open dependabot PRs: 0 (`list_pull_requests`, 2026-10-31).
2. `go list -m -u all` on Go 1.23: only x/oauth2 0.37 (needs Go 1.26, declined by policy) plus two modules that appear in the graph only (`compute/metadata`, `objx`). Missed by design until 1.23 leaves the window.
3. Workflow pins: setup-go was 4.1.0 in the parity job and 5.0.0 elsewhere; unified at 5.0.0. Whether 5.0.0 is the latest tag was not checked.
4. `scripts/verify`: exit 0 on main.
