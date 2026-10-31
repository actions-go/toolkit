# toolkit

Go port of https://github.com/actions/toolkit.

## Supported Go versions

The last 4 Go releases (1.23, 1.24, 1.25, 1.26 as of Oct 2026).

- `go.mod` `go` directive = the oldest supported release. When a new Go release ships, drop the oldest from the `build` matrix in `.github/workflows/ci.yml` and raise the directive in the same commit; update the README line too.
- CI uses `GOTOOLCHAIN=local`. Without it, a `go.mod` newer than the runner's Go silently downloads a newer toolchain and the matrix tests nothing older (this hid the problem on 1.23/1.24 legs until Oct 2026).
- A dependency whose `go.mod` requires a newer Go than the oldest supported release must not be adopted yet, even if it is the latest. Check with `go get <mod>@<ver>` under `GOTOOLCHAIN=go1.23.<n>`: it errors "requires go >= X". Dependabot PRs raising the `go` directive get the same treatment: rebase or close them.

## Checks

`scripts/verify` runs gofmt, build, vet and tests for `core/` and `parity/`. `cache/` and `github/` tests need `GITHUB_TOKEN` and network (`go test ./github/ -skip 'TestClient$'` runs the rest). `TestEnsureDestDir` in `cache` fails as root.
