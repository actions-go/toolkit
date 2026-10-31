package cache

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/actions-go/toolkit/core"
)

// Exit codes of unzip, see its man page.
const (
	unzipWarning     = 1
	unzipBadMember   = 2
	unzipNoFile      = 9
	unzipColumnWidth = 22
)

// ExtractZip extracts a zip archive into dest, a new folder below RUNNER_TEMP
// when dest is empty, and returns the destination, like extractZip of
// @actions/tool-cache does on Linux and macOS (it runs "unzip -o").
//
// Existing files are overwritten. Members are written the way unzip does:
// modes and times are restored, a leading "/" and ".." parts of a name are
// removed, and symbolic links are created after all the files, so that no
// member is written through them. An archive without members is a warning,
// which unzip reports with a failure code, so it is an error here too.
//
// Unlike unzip, a symbolic link pointing outside of dest is not created, and
// nothing is ever written through a symbolic link that leaves dest, including
// one that was in dest before the extraction.
//
// Names with characters outside of ASCII are written as the archive says:
// unzip turns them into #Uxxxx escapes unless the locale is UTF-8.
//
// With RUNNER_DEBUG=1 the members are listed in the format of unzip.
func ExtractZip(file, dest string) (string, error) {
	if file == "" {
		return "", errors.New("parameter 'file' is required")
	}
	dest, err := createExtractFolder(dest)
	if err != nil {
		return "", err
	}
	unzipPath, err := which("unzip")
	if err != nil {
		return "", err
	}
	args := []string{"-o", file}
	if !core.IsDebug() {
		args = []string{"-o", "-q", file}
	}
	echoCommand(unzipPath, args)

	code, err := unzip(file, dest, core.IsDebug())
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", processError(unzipPath, code)
	}
	return dest, nil
}

// unzipper extracts one archive.
type unzipper struct {
	root    extractRoot
	umask   os.FileMode
	listing bool
	// code is the exit code of unzip: the highest of the problems met
	code int
	// links are the symbolic links to create at the end, pending the names of
	// them as the paths they will have
	links   []*zip.File
	pending map[string]bool
	dirs    []deferredDir
}

func (u *unzipper) fail(code int, format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "unzip: "+format+"\n", args...)
	if code > u.code {
		u.code = code
	}
}

// unzip extracts file below dest and returns the exit code of unzip.
func unzip(file, dest string, listing bool) (int, error) {
	zr, name, err := openZip(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unzip:  cannot find or open %s, %s.zip or %s.ZIP.\n", file, file, file)
		return unzipNoFile, nil
	}
	defer zr.Close()
	root, err := newExtractRoot(dest)
	if err != nil {
		return 0, err
	}
	u := &unzipper{root: root, umask: umask(), listing: listing, pending: map[string]bool{}}
	if listing {
		core.Info("Archive:  " + name)
	}
	if len(zr.File) == 0 {
		u.fail(unzipWarning, "zipfile is empty")
	}
	for _, f := range zr.File {
		if err := u.extract(f); err != nil {
			u.fail(unzipWarning, "%s: %v", f.Name, err)
		}
	}
	u.finishLinks()
	applyDirectories(u.dirs)
	return u.code, nil
}

// openZip opens file, or file.zip, or file.ZIP like unzip does.
func openZip(file string) (*zip.ReadCloser, string, error) {
	var err error
	for _, name := range []string{file, file + ".zip", file + ".ZIP"} {
		var zr *zip.ReadCloser
		if zr, err = zip.OpenReader(name); err == nil {
			return zr, name, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			break
		}
	}
	return nil, "", err
}

// zipName cleans the name of a member the way unzip does: leading slashes,
// "." and ".." parts are dropped.
func zipName(name string) (rel string, dir bool) {
	var parts []string
	for _, p := range strings.Split(name, "/") {
		if p != "" && p != "." && p != ".." {
			parts = append(parts, p)
		}
	}
	return filepath.Join(parts...), strings.HasSuffix(name, "/")
}

// throughPendingLink reports whether rel is below a symbolic link that is not
// created yet: unzip leaves a file in its place, and so cannot extract below.
func (u *unzipper) throughPendingLink(rel string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i := 1; i < len(parts); i++ {
		if u.pending[strings.Join(parts[:i], "/")] {
			return true
		}
	}
	return false
}

func (u *unzipper) extract(f *zip.File) error {
	rel, isDir := zipName(f.Name)
	if strings.HasPrefix(f.Name, "/") {
		u.fail(unzipWarning, "warning:  stripped absolute path spec from %s", f.Name)
	}
	if rel == "" {
		return nil
	}
	mode := f.Mode()
	if mode&os.ModeSymlink == 0 && u.throughPendingLink(rel) {
		u.fail(unzipBadMember, "checkdir error:  %s exists but is not directory\n                 unable to process %s.", filepath.Dir(rel), f.Name)
		return nil
	}
	switch {
	case isDir || mode.IsDir():
		return u.directory(rel, f)
	case mode&os.ModeSymlink != 0:
		return u.queueLink(rel, f)
	}
	return u.file(rel, f)
}

// mode is the mode a member ends with: the permissions stored for a member
// that was zipped on Unix, the default minus the umask otherwise.
func (u *unzipper) mode(f *zip.File, dflt os.FileMode) os.FileMode {
	if f.CreatorVersion>>8 != 3 { // not created on Unix
		return dflt &^ u.umask
	}
	m := f.Mode()
	out := m.Perm()
	if m&os.ModeSetuid != 0 {
		out |= os.ModeSetuid
	}
	if m&os.ModeSetgid != 0 {
		out |= os.ModeSetgid
	}
	if m&os.ModeSticky != 0 {
		out |= os.ModeSticky
	}
	return out
}

// list prints a line of the listing: the verb right aligned on 11 columns,
// then the name on 22 columns and two blanks.
func (u *unzipper) list(verb, name, suffix string) {
	if !u.listing {
		return
	}
	core.Info(fmt.Sprintf("%11s: %-*s  %s", verb, unzipColumnWidth, name, suffix))
}

func (u *unzipper) directory(rel string, f *zip.File) error {
	parent, err := u.root.mkdirParents(filepath.Dir(rel))
	if err != nil {
		return err
	}
	path := filepath.Join(parent, filepath.Base(rel))
	created := false
	if info, err := os.Lstat(path); err != nil {
		if err := os.Mkdir(path, cachePerms); err != nil {
			return err
		}
		created = true
	} else if !info.IsDir() {
		return fmt.Errorf("%s exists and is not a directory", path)
	}
	if created && u.listing {
		core.Info(fmt.Sprintf("%11s: %s", "creating", f.Name))
	}
	u.dirs = append(u.dirs, deferredDir{path: path, mode: u.mode(f, 0o777), mtime: f.Modified})
	return nil
}

func (u *unzipper) file(rel string, f *zip.File) error {
	parent, err := u.root.mkdirParents(filepath.Dir(rel))
	if err != nil {
		return err
	}
	path := filepath.Join(parent, filepath.Base(rel))
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		if err := removeEntry(path); err != nil {
			return err
		}
	}
	in, err := f.Open()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	verb := "inflating"
	if f.Method == zip.Store {
		verb = "extracting"
	}
	u.list(verb, f.Name, "")
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	_ = os.Chmod(path, u.mode(f, 0o666))
	if !f.Modified.IsZero() {
		_ = os.Chtimes(path, f.Modified, f.Modified)
	}
	return nil
}

// linkTarget reads the target of a symbolic link member.
func linkTarget(f *zip.File) (string, error) {
	in, err := f.Open()
	if err != nil {
		return "", err
	}
	defer in.Close()
	target, err := io.ReadAll(in)
	return string(target), err
}

// queueLink notes a symbolic link to create once the files are extracted.
func (u *unzipper) queueLink(rel string, f *zip.File) error {
	target, err := linkTarget(f)
	if err != nil {
		return err
	}
	u.list("linking", f.Name, "-> "+target+" ")
	u.links = append(u.links, f)
	u.pending[filepath.ToSlash(rel)] = true
	return nil
}

// finishLinks creates the symbolic links, which unzip leaves for the end so
// that they cannot redirect a member that follows them.
func (u *unzipper) finishLinks() {
	if len(u.links) > 0 && u.listing {
		core.Info("finishing deferred symbolic links:")
	}
	for _, f := range u.links {
		if err := u.link(f); err != nil {
			u.fail(unzipWarning, "%s: %v", f.Name, err)
		}
	}
}

func (u *unzipper) link(f *zip.File) error {
	rel, _ := zipName(f.Name)
	target, err := linkTarget(f)
	if err != nil {
		return err
	}
	if u.listing {
		core.Info(fmt.Sprintf("  %-*s -> %s", unzipColumnWidth, f.Name, target))
	}
	parent, err := u.root.mkdirParents(filepath.Dir(rel))
	if err != nil {
		return err
	}
	if !u.root.safeLink(parent, target) {
		return fmt.Errorf("refusing to create a symbolic link to %s outside of the destination", target)
	}
	path := filepath.Join(parent, filepath.Base(rel))
	if err := removeEntry(path); err != nil {
		return err
	}
	return os.Symlink(target, path)
}
