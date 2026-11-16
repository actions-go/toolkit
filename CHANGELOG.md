# Changelog

All notable changes to this module. Versions follow [semver](https://semver.org).

## Versioning against @actions/toolkit

The upstream packages are versioned separately (as of 2026-11-16: `@actions/core` 3.1.0, `@actions/github` 9.1.1, `@actions/tool-cache` 4.0.0). This module ships all three ports under one version, so it follows upstream as far as Go allows:

- **Major:** Go requires a `/vN` module path suffix from v2 on, which would change every import path. The major stays at 1, the highest that keeps `github.com/actions-go/toolkit`.
- **Minor:** tracks the minor of `@actions/core`, the reference the port was last compared against (3.0.1, pinned in `parity/js/package.json`). Hence 1.0.
- **Patch:** fixes and backports that do not change the API.

The upstream commit last backported is in [`.actions-toolkit-sha`](.actions-toolkit-sha).

## [1.0.0] - 2026-11-16

First tagged release. Before it, the module was only usable through pseudo-versions.

### Added

- `cache`: `ExtractTar` and `ExtractZip` (GNU tar and Info-ZIP `unzip` behaviour, including modes, symlinks, listings and failure codes), `Extract7z` and `ExtractXar` (OS gate, argument order, echoed command line and failure message as in `@actions/tool-cache`).
- `cache`: `GetManifestFromRepo`, `FindFromManifest`, `IsExplicitVersion`, `EvaluateVersions` and `OSVersion`, for picking a tool release from a `versions-manifest.json`. `ManifestAPIURL` redirects the API host (the reference hard codes `api.github.com`).
- `parity/`: a suite that runs the same scenarios against the Go packages and the pinned JavaScript packages (`@actions/core` 3.0.1, `@actions/github` 9.1.1, `@actions/tool-cache` 4.0.0) and compares the results, on Linux, macOS and Windows. Known differences are listed in `parity/known-divergences.json`.

### Changed

- Supported Go versions are now the last four releases: 1.23 to 1.26. `go.mod` declares `go 1.23.0`, and CI sets `GOTOOLCHAIN=local` so each matrix leg runs its own release.
- **Dependencies:** `go-github` v42 to v74, `golang.org/x/oauth2` 0.30, `semver/v3` 3.5, `testify` 1.12.1. `go-github` v74 sends an `X-GitHub-Api-Version` header that v42 did not; code that asserts on request headers sees the difference. `x/oauth2` 0.37 is held back because it needs Go 1.26.
- `cache`: `RUNNER_TOOL_CACHE` and `RUNNER_TEMP` are read at each call instead of once at start-up.
- `DownloadTool` follows `@actions/tool-cache` (see its parity scenarios for the details).
- Commands and file commands end with the platform EOL (`\r\n` on Windows), as `@actions/core` does.

### Fixed

- `core`: inputs are trimmed and blank lines in multi-line inputs are kept.
- `core`: multi-line values written to the file commands use a random delimiter.
- `core`: `GetIDToken` sends the request and reports errors with the reference messages.
- `core`: platform names, empty command names and missing tools are reported as in `@actions/core`.
- `core`: job summary HTML attributes are rendered in a stable order.
- `core`/`cache` on Windows: a missing executable gets the `@actions/io` hint ("verify the file has a valid extension").
- `cache` on Windows: `CacheFile` strips `../` from its target, which previously could escape the tool folder.
- `cache`: `ExtractTar` keeps the umask-reduced mode on a file cut off by a truncated archive when run as a regular user.
- `cache`: the stderr of `xar` and `7z` goes to the step log, as `@actions/exec` does.
- `core`: the missing-event-file message ends with the platform EOL.

### Known gaps

- `DownloadTool` does not retry (marked `TODO`).
- `Extract7z` without a path looks up `7z` then `7zr` in `PATH`; the reference runs a bundled `7zdec.exe` through PowerShell, and nothing is bundled here.
- Executable lookup differs slightly from `@actions/io`; recorded in `docs/goals/js-parity.md`.
- `GetManifestFromRepo` and `EvaluateVersions` diverge from the reference on malformed input (a non-array manifest, an invalid version in the list).
- tar and zip extraction are compared against the reference on Linux only.

### Maintenance

- CI actions bumped: `actions/checkout` 7.0.1, `actions/setup-go` 7.0.0, `actions/setup-node` 7.
