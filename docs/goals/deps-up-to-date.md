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

## Log
(filled as work lands)
