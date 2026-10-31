package cache

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/actions-go/toolkit/core"
)

// maxCopyDepth bounds the recursion of a directory copy like @actions/io does.
const maxCopyDepth = 255

func noRel(path string) string {
	p := strings.Split(path, string(filepath.Separator))
	// In case the source is provided
	for len(p) > 0 && (p[0] == ".." || p[0] == ".") {
		p = p[1:]
	}
	return filepath.Join(p...)
}

// statSource is os.Stat reporting failures like node does ("ENOENT: ...").
func statSource(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("ENOENT: no such file or directory, stat '%s'", path)
	case errors.Is(err, fs.ErrPermission):
		return nil, fmt.Errorf("EACCES: permission denied, stat '%s'", path)
	}
	return info, err
}

// newToolFolder empties the folder of a tool version and its completion
// marker, and returns the folder.
func newToolFolder(options CacheOptions) (string, error) {
	folder := toolPath(options)
	core.Debugf("destination %s", folder)
	if err := os.RemoveAll(folder); err != nil {
		return "", err
	}
	if err := os.RemoveAll(folder + ".complete"); err != nil {
		return "", err
	}
	return folder, os.MkdirAll(folder, cachePerms)
}

// completeToolFolder marks the folder of a tool version as fully cached.
func completeToolFolder(options CacheOptions) error {
	if err := os.WriteFile(toolPath(options)+".complete", nil, 0o666); err != nil {
		return err
	}
	core.Debug("finished caching tool")
	return nil
}

// prepare validates options and logs the caching of a source.
func prepare(kind, source string, options CacheOptions) (CacheOptions, os.FileInfo, error) {
	options, err := defaultOptions(options)
	if err != nil {
		return options, nil, err
	}
	core.Debugf("Caching tool %s %s %s", options.Tool, options.Version, options.Arch)
	core.Debugf("source %s: %s", kind, source)
	info, err := statSource(source)
	return options, info, err
}

// CacheFile caches a downloaded file (GUID) and installs it
// into the tool cache with a given targetName
func CacheFile(source, target string, options CacheOptions) (string, error) {
	options, info, err := prepare("file", source, options)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("sourceFile is not a file")
	}
	folder, err := newToolFolder(options)
	if err != nil {
		return "", err
	}
	// Ensure provided arguments are namespaced to the folder
	dest := filepath.Join(folder, noRel(target))
	core.Debugf("destination file %s", dest)
	if err := ioCopy(source, dest); err != nil {
		return "", err
	}
	return folder, completeToolFolder(options)
}

// CacheDir caches a directory and installs it into the tool cacheDir
// with a given targetName
func CacheDir(source string, options CacheOptions) (string, error) {
	options, info, err := prepare("dir", source, options)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("sourceDir is not a directory")
	}
	folder, err := newToolFolder(options)
	if err != nil {
		return "", err
	}
	// copy each child item. do not move.
	children, err := os.ReadDir(source)
	if err != nil {
		return "", err
	}
	for _, child := range children {
		if err := ioCopy(filepath.Join(source, child.Name()), folder); err != nil {
			return "", err
		}
	}
	return folder, completeToolFolder(options)
}

// ioCopy copies source to dest like io.cp(source, dest, {recursive: true}) of
// @actions/io: into dest when it is an existing directory, directories
// recursively with their modes, symbolic links as links.
func ioCopy(source, dest string) error {
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		dest = filepath.Join(dest, filepath.Base(source))
	}
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("no such file or directory: %s", source)
	}
	if info.IsDir() {
		return copyTree(source, dest, 0)
	}
	return copyEntry(source, dest)
}

func copyTree(source, dest string, depth int) error {
	if depth >= maxCopyDepth {
		return nil
	}
	if err := os.MkdirAll(dest, cachePerms); err != nil {
		return err
	}
	children, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, child := range children {
		from, to := filepath.Join(source, child.Name()), filepath.Join(dest, child.Name())
		info, err := os.Lstat(from)
		if err != nil {
			return err
		}
		if info.IsDir() {
			err = copyTree(from, to, depth+1)
		} else {
			err = copyEntry(from, to)
		}
		if err != nil {
			return err
		}
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	return os.Chmod(dest, info.Mode().Perm())
}

// copyEntry copies a file with its mode, or recreates a symbolic link.
func copyEntry(source, dest string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(source)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(dest); err == nil {
			if err := os.Remove(dest); err != nil {
				return err
			}
		}
		return os.Symlink(target, dest)
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("ENOENT: no such file or directory, copyfile '%s' -> '%s'", source, dest)
	}
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(dest, info.Mode().Perm())
}
