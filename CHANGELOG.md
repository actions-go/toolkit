# Changelog

This project follows [semantic versioning](https://semver.org). The module has no `/v3` path suffix, so Go rejects a `v3.x.y` tag; versions stay `v0.x.y`. The minor number is chosen to line up with the upstream line the code is tested against: `v0.3.x` tracks `@actions/core` 3.0.x and `@actions/tool-cache` 4.0.x.

## v0.3.0 - 2026-11-09

First tagged release. Before it, consumers could only pin pseudo-versions.

Tested against (see [docs/goals/js-parity.md](docs/goals/js-parity.md)):

| Package | Upstream reference | Parity scenarios |
|---|---|---|
| `core` | `@actions/core` 3.0.1 | 291, on Linux, macOS, Windows |
| `cache` | `@actions/tool-cache` 4.0.0 | 363, on Linux, macOS, Windows (some skipped, see parity doc) |
| `github` | `@actions/github` 9.1.1 | compared in `parity/github` |

Last upstream commit ported: see `.actions-toolkit-sha`.

### Breaking changes

Behaviour now matches the JavaScript reference where it used to differ. Code that relied on the old behaviour changes.

- Go 1.23 is the minimum (`go 1.23.0`), and every dependency was raised to the newest version that builds on it. CI covers Go 1.23 to 1.26 on Linux, macOS and Windows.
- `GetInput` trims whitespace and multiline inputs keep blank lines, like `@actions/core`.
- Commands and file commands end with the platform EOL (`\r\n` on Windows).
- A missing event file message ends with the platform EOL.
- `GetIDToken` now sends the request and returns errors instead of swallowing them.
- Platform names, empty command names and missing-tool errors use the reference wording.
- Summary HTML attributes render in a stable order with the reference messages.
- `cache` downloads tools like `@actions/tool-cache`.
- On Windows, missing executables and escaping targets behave as in the reference.

### Added

- `cache`: `ExtractTar`, `ExtractZip`, `Extract7z` and `ExtractXar`, with output from `xar` and `7z` forwarded into the step log.
- `cache`: tool manifest reading (`FindFromManifest` and related helpers).
- `parity/`: a harness that runs the same scenarios through the Go port and the JavaScript package and diffs the results, plus a CI job that runs it on every OS.

### Fixed

- Multi-line file command values use a random delimiter, so a value can no longer terminate itself early.
- `tar` extraction keeps the mode of a cut file instead of applying the archive permissions.
- `RUNNER_TOOL_CACHE` and `RUNNER_TEMP` are read at each call, not at package load.

### Known gaps

- Executable lookup differs from the reference in one accepted case, recorded in [docs/goals/js-parity.md](docs/goals/js-parity.md).
- `tar` and `zip` extraction is compared with the reference on Linux only. macOS ships bsdtar and a different unzip build.
- On Windows the `cache` parity suite skips scenarios that need Unix mode bits, symlinks or an OS version.

### Maintenance

- CI: matrix reduced from 12 to 6 jobs; actions bumped to `checkout` 7.0.1, `setup-go` 7.0.0 and `setup-node` 7.
- Open Dependabot PRs for `testify` 1.7.0 and `oauth2` 0.37.0 are not part of this release. The `go` directive stays at 1.23.0 until Go 1.23 leaves the support window.
