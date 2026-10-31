# Changelog

Versions follow [semver](https://semver.org). The module path has no `/vN` suffix, so Go only allows `v0` and `v1`. The major version therefore cannot follow `@actions/core` (3.x). The minor version tracks it: `v1.1.0` is aligned with `@actions/core` 3.1.0, the upstream state recorded in [`.actions-toolkit-sha`](.actions-toolkit-sha) (`0c3a9c3`). Matching the upstream major would need a `/v3` import path, which breaks every importer.

## v1.1.0 - 2026-10-31

First tagged release. Earlier work was consumed by commit SHA. Aligned with `@actions/core` 3.1.0, `@actions/github` 9.1.1 and `@actions/tool-cache` 4.0.0 (the versions the parity suite compares against).

### Added

- `core`: `DeclareArtifact`, `GetIDToken` (OIDC), job summaries (`JobSummary`, `AddStepSummary`), `Platform`/`Arch`/`GetDetails`, `ToPosixPath`/`ToWin32Path`/`ToPlatformPath`, `StopCommands`/`WithoutCommands`, `GetBoolInput`, `GetMultilineInput`, `GetInputOrDefault`, and a JSON `ACTION_GO_INPUTS` fallback for inputs.
- `github`: `RefName` and `RefType` on the action context, the `ACTIONS_ORCHESTRATION_ID` user-agent tag, GitHub Enterprise server URL support, and `DownloadSelectedRepositoryFiles`.
- `cache`: tool-cache split into download, find and store, matching `@actions/tool-cache`; `RUNNER_TOOL_CACHE` and `RUNNER_TEMP` are read on every call.
- A parity suite (`parity/`) that runs the Go packages and the reference JavaScript packages on the same scenarios and compares the results, on Linux, macOS and Windows. `scripts/verify` runs the fast offline checks.
- [README](README.md) rewritten for the Go toolkit; goal files under `docs/goals/` record the parity and dependency work and what they taught.

### Changed (behaviour now matches `@actions/core`)

- Commands and file commands end with the platform EOL; the missing-event-file message does too.
- Multi-line file command values use a random delimiter.
- `GetInput` trims values; multi-line inputs keep blank lines.
- `GetIDToken` sends the request and returns errors.
- Platform names, empty command names and missing tools are reported with the reference messages.
- Summary HTML attributes render in a stable order.
- `DownloadTool` behaves like `@actions/tool-cache`.

### Dependencies and build

- Minimum Go is 1.23 (`go 1.23.0`); CI covers the last four releases (1.23 to 1.26) with `GOTOOLCHAIN=local`, shrunk to 6 build jobs.
- Dependencies updated to the newest versions that build on Go 1.23, including `golang.org/x/crypto` 0.52.0 and `golang.org/x/net` (security fixes), `golang.org/x/oauth2` and `google/uuid`.

### Known gaps

- `cache.DownloadTool` does not retry downloads yet.
- `TestClient` in `github` needs the live GitHub API and `GITHUB_TOKEN`; `TestEnsureDestDir` in `cache` fails when run as root.
