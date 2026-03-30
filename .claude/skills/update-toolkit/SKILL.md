You are executing the `update-toolkit` skill. Your job is to backport new features from the upstream `actions/toolkit` JavaScript repository into this Go port (`actions-go/toolkit`), then update `.actions-toolkit-sha` and ensure all tests pass.

## Overview

This repository is a Go port of GitHub's [actions/toolkit](https://github.com/actions/toolkit) JavaScript library. The file `.actions-toolkit-sha` contains the last upstream commit SHA that was fully backported.

## Step 1: Determine what changed upstream

Run these commands to find new upstream changes:

```bash
# Get the currently tracked SHA
TRACKED_SHA=$(cat .actions-toolkit-sha)
echo "Currently tracked: $TRACKED_SHA"

# Clone or update upstream (shallow first for speed)
if [ ! -d /tmp/actions-toolkit-upstream/.git ]; then
  git clone --depth=50 https://github.com/actions/toolkit.git /tmp/actions-toolkit-upstream
else
  git -C /tmp/actions-toolkit-upstream fetch --depth=50
fi

# Get the upstream HEAD
UPSTREAM_HEAD=$(git -C /tmp/actions-toolkit-upstream rev-parse origin/HEAD 2>/dev/null || git -C /tmp/actions-toolkit-upstream rev-parse HEAD)
echo "Upstream HEAD: $UPSTREAM_HEAD"

# Check if we need to unshallow to access the tracked SHA
if ! git -C /tmp/actions-toolkit-upstream cat-file -e "$TRACKED_SHA" 2>/dev/null; then
  echo "Fetching full history to access tracked SHA..."
  git -C /tmp/actions-toolkit-upstream fetch --unshallow
fi

# List commits that changed relevant packages since our tracked SHA
git -C /tmp/actions-toolkit-upstream log --oneline "$TRACKED_SHA".."$UPSTREAM_HEAD" \
  -- packages/core/ packages/github/ packages/cache/ 2>&1
```

If there are **no new commits**, print "Already up to date at $TRACKED_SHA" and stop.

## Step 2: Analyse what changed

For each relevant commit, examine the diff to understand what new functionality was added:

```bash
git -C /tmp/actions-toolkit-upstream diff "$TRACKED_SHA".."$UPSTREAM_HEAD" \
  -- packages/core/src/ packages/github/src/ 2>&1
```

Focus on:
- **New exported functions** in `packages/core/src/core.ts`
- **New source files** (e.g. `packages/core/src/platform.ts`)
- **New constants or types** exported from any package
- **New context fields** in `packages/github/src/context.ts`
- Changes to **`packages/cache/`** that affect the tool-cache API

**Ignore**: TypeScript-specific changes (types, generics syntax), build tooling, dependency bumps, tests.

## Step 3: Port each change to Go

For each meaningful upstream change, implement the equivalent in Go following the existing patterns:

### Mapping rules

| JS/TS concept | Go equivalent |
|---|---|
| `export function foo(...)` | `func Foo(...)` in the relevant `.go` file |
| `export const isWindows = ...` | `var IsWindows = ...` |
| `export interface Bar { ... }` | `type Bar struct { ... }` |
| New source file `packages/core/src/baz.ts` | New file `core/baz.go` |
| `os.platform()` | `runtime.GOOS` |
| `os.arch()` | `runtime.GOARCH` (mapped to JS names where needed) |
| `exec.getExecOutput(cmd)` | `exec.Command(cmd).Output()` |
| New field on GitHub context | New field on `ActionContext` in `github/context.go` |
| `process.env['VAR']` | `os.Getenv("VAR")` |

### File placement

| Upstream file | Go file |
|---|---|
| `packages/core/src/core.ts` | `core/core.go` |
| `packages/core/src/command.ts` | `core/command.go` |
| `packages/core/src/summary.ts` | `core/summary.go` |
| `packages/core/src/oidc-utils.ts` | `core/oidc.go` |
| `packages/core/src/path-utils.ts` | `core/path.go` |
| `packages/core/src/platform.ts` | `core/platform.go` |
| `packages/github/src/context.ts` | `github/context.go` |

### Style rules

- Follow Go naming conventions (exported = PascalCase, unexported = camelCase)
- Match the existing code style in the file you are editing
- OS-specific behaviour: check existing `_unix.go`/`_windows.go` split for precedent
- Keep injected variables (like `var runCommand = ...`) for testability
- Do not add features beyond what was added upstream

## Step 4: Write tests for every ported feature

For each new function or type, add tests in the corresponding `_test.go` file:

- Place tests in the same package (internal white-box tests)
- Use `github.com/stretchr/testify/assert` and `require`
- Stub OS-command execution via the `runCommand` variable when applicable
- For platform-specific tests, use `t.Skip()` with a clear message

Run tests after each ported feature to validate:

```bash
go test ./... 2>&1
```

## Step 5: Update `.actions-toolkit-sha`

Once all changes are ported and all tests pass, update the tracked SHA:

```bash
echo -n "$UPSTREAM_HEAD" > .actions-toolkit-sha
```

Verify the file:

```bash
cat .actions-toolkit-sha
```

## Step 6: Run the full test suite one final time

```bash
go test ./... 2>&1
```

All tests must pass before finishing. If any test fails, investigate and fix before proceeding.

## Notes

- The `jsArch()` helper in `core/platform.go` maps Go arch names to Node.js `os.arch()` equivalents (e.g. `amd64` → `x64`, `386` → `x32`).
- The `cache` package in this repo is a *tool cache* (equivalent to `@actions/tool-cache`), not the CI cache (`@actions/cache`) — do not confuse them.
- When in doubt about Go equivalent of a JS feature, look at existing Go files in the same package for precedent.
