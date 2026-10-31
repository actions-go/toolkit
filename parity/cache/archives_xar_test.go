package cacheparity

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// buildXar writes the files below a temporary directory and archives its
// top-level entries with the xar binary (macOS ships it), which is the only
// thing that makes xar archives. Only files and directories are supported.
func (a archiveSpec) buildXar() ([]byte, error) {
	work, err := os.MkdirTemp("", "parity-xar-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	tree := filepath.Join(work, "tree")
	top := map[string]bool{}
	var tops []string
	for _, f := range a.Files {
		name := strings.TrimSuffix(f.Name, "/")
		p := filepath.Join(tree, name)
		switch f.Type {
		case "", "file":
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(p, []byte(f.Content), 0o644); err != nil {
				return nil, err
			}
			if err := os.Chmod(p, f.mode(0o644)); err != nil {
				return nil, err
			}
		case "dir":
			if err := os.MkdirAll(p, 0o755); err != nil {
				return nil, err
			}
			if err := os.Chmod(p, f.mode(0o755)); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("member type %q is not supported in xar fixtures", f.Type)
		}
		if first := strings.Split(name, "/")[0]; !top[first] {
			top[first] = true
			tops = append(tops, first)
		}
	}
	out := filepath.Join(work, "a.xar")
	cmd := exec.Command("xar", append([]string{"-c", "-f", out}, tops...)...)
	cmd.Dir = tree
	if msg, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("xar -c: %v: %s", err, msg)
	}
	return os.ReadFile(out)
}
