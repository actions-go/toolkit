# Changelog

All notable changes are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project follows [Semantic Versioning](https://semver.org/).

## Versioning

Upstream `@actions/core` is at 3.x, but a Go module at v2 or later must change its import path (`.../toolkit/v3`). That would break every importer, so this module stays on `v0.x` and uses the minor number to track the upstream major of `@actions/core`: `v0.3.x` matches `@actions/core` 3.x. Patch releases carry fixes and additions. Move to `v1.0.0` once the API is declared stable.

Versions of the references the parity suite compares against (see [parity](parity)): `@actions/core` 3.0.1, `@actions/github` 9.1.1, `@actions/tool-cache` 4.0.0.

## [0.3.0] - 2026-11-02

First tagged release. It covers everything on `main` up to 5155863.

### Added

- `cache`: `ExtractTar`, `ExtractZip`, `Extract7z` and `ExtractXar`, like `@actions/tool-cache`.
- `cache`: tool manifests are read like `@actions/tool-cache`.
- `parity`: a test suite that runs the same scenarios against the Go packages and the JavaScript references and compares results, on Linux, macOS and Windows. Known differences are listed in `parity/known-divergences.json`.
- Support for the last four Go releases (1.23 to 1.26), with CI running each with `GOTOOLCHAIN=local`.

### Changed

- `go.mod` declares `go 1.23.0`, and all dependencies were raised to the newest versions that build on it.
- `RUNNER_TOOL_CACHE` and `RUNNER_TEMP` are read at each call, not once at start-up.
- Workflow commands and file commands end with the platform EOL (`\r\n` on Windows).

### Fixed

- `core`: `GetInput` trims values and multi-line inputs keep blank lines, like `@actions/core`.
- `core`: multi-line file command values use a random delimiter.
- `core`: `GetIDToken` sends the request and reports errors like `@actions/core`.
- `core`: platform names, empty command names and missing tools are reported like `@actions/core`.
- `core`: summary HTML attributes are rendered in a stable order.
- `core`: the missing event file message ends with the platform EOL.
- `cache`: `DownloadTool` behaves like `@actions/tool-cache`.
- `cache`: tar extraction keeps the mode `tar` gives a cut file when no archive permissions apply.
- `cache`: the `xar` and `7z` stderr is forwarded to the step log.
- `cache`: missing executables and escaping targets on Windows match the reference.
- Dependency bumps for `golang.org/x/crypto` (0.52.0) and `golang.org/x/oauth2` (0.16.0), and the GitHub Actions used in CI.

### Known gaps

- `cache`: download retries are not ported (`TODO` in `DownloadTool`).
- How executables are looked up differs from the reference; see `docs/goals/js-parity.md`.

[0.3.0]: https://github.com/actions-go/toolkit/releases/tag/v0.3.0
