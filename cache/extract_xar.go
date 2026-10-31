package cache

import (
	"errors"

	"github.com/actions-go/toolkit/core"
)

// ExtractXar extracts a xar compatible archive into dest, a new folder below
// RUNNER_TEMP when dest is empty, and returns the destination, like extractXar
// of @actions/tool-cache. It is only supported on macOS, where it runs the
// xar found in PATH as "xar <flags> -x -C <dest> -f <file>" (with -v when
// debugging is on), repeated arguments removed.
func ExtractXar(file, dest string, flags ...string) (string, error) {
	if hostOS != "darwin" {
		return "", errors.New("extractXar() not supported on current OS")
	}
	if file == "" {
		return "", errors.New(`parameter "file" is required`)
	}
	dest, err := createExtractFolder(dest)
	if err != nil {
		return "", err
	}
	args := append(append([]string{}, flags...), "-x", "-C", dest, "-f", file)
	if core.IsDebug() {
		args = append(args, "-v")
	}
	xarPath, err := which("xar")
	if err != nil {
		return "", err
	}
	if err := runTool(xarPath, unique(args), "", false); err != nil {
		return "", err
	}
	return dest, nil
}

// unique removes the repeated values, keeping the first of each in order.
func unique(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
