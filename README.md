[![GoDoc](https://godoc.org/github.com/actions-go/toolkit?status.svg)](https://godoc.org/github.com/actions-go/toolkit)
[![Go CI](https://github.com/actions-go/toolkit/actions/workflows/ci.yml/badge.svg)](https://github.com/actions-go/toolkit/actions/workflows/ci.yml)

# GitHub Actions Go Toolkit

A Go port of the official [@actions/toolkit](https://github.com/actions/toolkit) JavaScript libraries (`core`, `cache`, `github`), for writing GitHub Actions in Go.

The last upstream commit that was backported is recorded in [`.actions-toolkit-sha`](.actions-toolkit-sha). The [`update-toolkit`](.claude/skills/update-toolkit/SKILL.md) Claude skill ports new upstream changes and moves that SHA forward.

```bash
go get github.com/actions-go/toolkit
```

CI runs the tests on Linux, macOS and Windows with the last four Go releases (currently 1.23, 1.24, 1.25 and 1.26; see [ci.yml](.github/workflows/ci.yml)). `go.mod` declares the oldest of them (`go 1.23.0`), and CI sets `GOTOOLCHAIN=local` so each matrix leg really runs that release. Dependencies are therefore kept at the newest versions that still build on that oldest release.

To start a new action, use the [go-action-template](https://github.com/tjamet/go-action-template).

## Packages

### [core](core)

Inputs, outputs, logging and annotations, groups, secrets, environment variables and `PATH`, saved state, job summaries, OIDC tokens, artifact declarations and platform detection. [GoDoc](https://godoc.org/github.com/actions-go/toolkit/core).

```go
package main

import "github.com/actions-go/toolkit/core"

func main() {
	// GetInput returns the value and whether the input was set.
	who, ok := core.GetInput("who-to-greet")
	if !ok {
		core.SetFailed("who-to-greet is required")
		return
	}
	core.Infof("Hello %s!", who)
	core.SetOutput("greeted", who)
	core.JobSummary.AddHeading("Greeting", 2).AddRaw("Said hello to " + who).AddEOL().Write()
}
```

Inputs are read from `INPUT_<NAME>` as in the JavaScript toolkit. If that variable is missing, `GetInput` falls back to a JSON object in `ACTION_GO_INPUTS`, which lets an action receive structured values (non-string values are re-serialised to JSON).

Also available: `GetInputOrDefault`, `GetBoolInput`, `GetMultilineInput`, `Group`, `StopCommands`/`WithoutCommands`, `SaveState`/`GetState`, `AddStepSummary`, `GetIDToken`, `DeclareArtifact` (runner 2.336.0 or later), `ToPosixPath`/`ToWin32Path`/`ToPlatformPath`, and `Platform`/`Arch`/`GetDetails`.

### [cache](cache)

Download and cache tools in the runner tool cache, for `setup-*` style actions: `DownloadTool`, `CacheFile`, `CacheDir`, `FindVersion`, `ListAllCachedVersions` and `GetCachedToolOrDownload`. Versions are matched with semver. [GoDoc](https://godoc.org/github.com/actions-go/toolkit/cache).

`CacheOptions.UseJavascriptValues` selects Node.js `os.platform()`/`os.arch()` names (`x64`, `win32`) instead of `GOOS`/`GOARCH` when building the cache path, so caches stay compatible with JavaScript actions.

Not ported yet: download retries (marked `TODO` in `DownloadTool`).

### [github](github)

A [go-github](https://github.com/google/go-github) client (v42) authenticated from `GITHUB_TOKEN`, or from the `github-token` or `token` input, plus the workflow context. [GoDoc](https://godoc.org/github.com/actions-go/toolkit/github).

- `github.GitHub` is a ready-to-use client; `github.NewClient()` builds a new one. It targets GitHub Enterprise when `GITHUB_SERVER_URL` is not `https://github.com`, and adds the `ACTIONS_ORCHESTRATION_ID` tag to the user agent.
- `github.ParseActionEnv()` returns the run context (repo, event name, SHA, ref, actor, payload and so on) from the runner environment.
- `github.DownloadSelectedRepositoryFiles` fetches a repository tarball and returns the files accepted by a `Matcher` (`MatchesOneOf` builds one from patterns).

## Development

```bash
go test ./...
```

Some `github` tests call the GitHub API; set `GITHUB_TOKEN` to avoid unauthenticated rate limits, as CI does. CI also runs `go test -run TestOutputTasks ./core/...` with `ACTIONS_OUTPUT_SET=true` to check that outputs are passed between steps.

On pushes, CI runs `gofmt -w .` and commits any formatting fixes, so run it before committing.
