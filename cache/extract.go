package cache

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/actions-go/toolkit/core"
	"github.com/google/uuid"
)

// createExtractFolder makes the folder an archive is extracted into, a new
// folder below RUNNER_TEMP when dest is empty, like _createExtractFolder of
// @actions/tool-cache.
func createExtractFolder(dest string) (string, error) {
	if dest == "" {
		dest = filepath.Join(tempDir(), uuid.New().String())
	}
	if err := os.MkdirAll(dest, cachePerms); err != nil {
		return "", err
	}
	return dest, nil
}

// deferredDir is a directory whose mode and time are set once its content is
// extracted, so that a read only directory can be filled.
type deferredDir struct {
	path  string
	mode  os.FileMode
	mtime time.Time
}

// applyDirectories sets the mode and time of the directories, deepest first.
func applyDirectories(dirs []deferredDir) {
	sort.SliceStable(dirs, func(i, j int) bool { return len(dirs[i].path) > len(dirs[j].path) })
	for _, d := range dirs {
		_ = os.Chmod(d.path, d.mode)
		if !d.mtime.IsZero() {
			_ = os.Chtimes(d.path, d.mtime, d.mtime)
		}
	}
}

// which finds an executable in PATH like io.which(tool, true) of @actions/io
// does.
func which(tool string) (string, error) {
	path, err := exec.LookPath(tool)
	if err != nil {
		hint := "Also check the file mode to verify the file is executable."
		if core.IsWindows {
			hint = "Also verify the file has a valid extension for an executable file."
		}
		return "", fmt.Errorf("Unable to locate executable file: %s. Please verify either the file path exists or the file can be found within a directory specified by the PATH environment variable. %s", tool, hint)
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return path, nil
}

// echoCommand prints the command line like @actions/exec does before it runs
// a tool. The extraction itself is done in process, but a workflow log that
// reads the same as the one of the reference is easier to compare and to
// debug.
func echoCommand(tool string, args []string) {
	core.Info("[command]" + tool + " " + strings.Join(args, " "))
}

// processError is the error @actions/exec throws when a tool exits with a
// failure code.
func processError(tool string, code int) error {
	return fmt.Errorf("The process '%s' failed with exit code %d", tool, code)
}

// extractMember is the part of an extraction that is common to tar and zip:
// where members may be written.
type extractRoot struct {
	// dir is the destination as given, resolved the symbolic links of its
	// parents so that a member can be checked against it.
	dir string
}

func newExtractRoot(dest string) (extractRoot, error) {
	resolved, err := filepath.EvalSymlinks(dest)
	if err != nil {
		return extractRoot{}, err
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return extractRoot{}, err
	}
	return extractRoot{dir: abs}, nil
}

var errOutside = errors.New("outside of the destination")

// mkdirParents creates the directories of rel (a slash free of ".." and
// absolute prefixes, below the destination) that do not exist yet and returns
// the resolved path of the last one. A symbolic link met on the way is followed
// only when it stays inside the destination: an archive cannot make a later
// member land outside of it.
func (r extractRoot) mkdirParents(rel string) (string, error) {
	cur := r.dir
	if rel == "" || rel == "." {
		return cur, nil
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		next := filepath.Join(cur, part)
		info, err := os.Lstat(next)
		switch {
		case errors.Is(err, os.ErrNotExist):
			if err := os.Mkdir(next, cachePerms); err != nil {
				return "", err
			}
		case err != nil:
			return "", err
		case info.Mode()&os.ModeSymlink != 0:
			resolved, err := filepath.EvalSymlinks(next)
			if err != nil {
				return "", err
			}
			if !isInside(r.dir, resolved) {
				return "", errOutside
			}
			if st, err := os.Stat(resolved); err != nil || !st.IsDir() {
				return "", fmt.Errorf("%s is not a directory", next)
			}
			next = resolved
		case !info.IsDir():
			return "", fmt.Errorf("%s is not a directory", next)
		}
		cur = next
	}
	return cur, nil
}

// isInside reports whether path is dir or below it.
func isInside(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

// safeLink reports whether a symbolic link in the directory parent (as
// returned by parent) pointing at target stays inside the destination.
// Absolute targets never do: they point at the file system of the machine, not
// at the archive.
func (r extractRoot) safeLink(parent, target string) bool {
	if filepath.IsAbs(target) || strings.HasPrefix(target, "/") || strings.HasPrefix(target, `\`) {
		return false
	}
	return isInside(r.dir, filepath.Join(parent, target))
}
