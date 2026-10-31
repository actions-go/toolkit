package cache

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/actions-go/toolkit/core"
)

// tarFailure is the exit code tar ends with when it could not extract
// everything ("fatal error" in GNU tar, any failure in bsdtar).
const tarFailure = 2

// tarBlock is the size of the blocks tar reads and writes.
const tarBlock = 512

// ExtractTar extracts a tar archive into dest, a new folder below RUNNER_TEMP
// when dest is empty, and returns the destination, like extractTar of
// @actions/tool-cache.
//
// flags are the arguments of the tar command; none means "xz" (a gzipped
// tar). Like in the reference a single flag is a bundle of option letters
// ("xz", "xj", "xzv") and several are separate arguments
// ("xz", "--strip-components=1"). The options supported are x, z, j, v, p, m,
// --strip-components (with = or as a separate argument), --no-same-owner,
// --same-owner, --no-same-permissions and --same-permissions. Anything else,
// and xz, zstd and compress archives, is reported as an error: the Go port
// reads the archive itself instead of running tar. A gzip or bzip2 archive is
// recognised whatever the compression flag, as GNU tar does, but a flag that
// does not match the archive fails.
//
// Unlike tar, a symbolic link pointing outside of dest (an absolute target or
// enough ".." to leave it) is not created, and nothing is ever written
// through a symbolic link that leaves dest. Such members, and members whose
// name contains "..", are skipped and make the extraction fail once the rest
// is extracted, the way tar fails on the latter.
func ExtractTar(file, dest string, flags ...string) (string, error) {
	if file == "" {
		return "", errors.New("parameter 'file' is required")
	}
	dest, err := createExtractFolder(dest)
	if err != nil {
		return "", err
	}
	core.Debug("Checking tar --version")
	tarPath, err := which("tar")
	if err != nil {
		return "", err
	}
	version, _ := exec.Command(tarPath, "--version").CombinedOutput()
	core.Debug(strings.TrimSpace(string(version)))
	isGnuTar := strings.Contains(strings.ToUpper(string(version)), "GNU TAR")

	user := flags
	if len(user) == 0 {
		user = []string{"xz"}
	}
	selected := append([]string(nil), user...)
	if core.IsDebug() && !includesV(user) {
		selected = append(selected, "-v")
	}
	args := append([]string(nil), selected...)
	destArg, fileArg := dest, file
	if runtime.GOOS == "windows" && isGnuTar {
		args = append(args, "--force-local")
		destArg, fileArg = strings.ReplaceAll(dest, `\`, "/"), strings.ReplaceAll(file, `\`, "/")
	}
	if isGnuTar {
		args = append(args, "--warning=no-unknown-keyword", "--overwrite")
	}
	args = append(args, "-C", destArg, "-f", fileArg)
	echoCommand(tarPath, args)

	opts, err := parseTarFlags(selected)
	if err != nil {
		return "", err
	}
	opts.gnu = isGnuTar
	if !opts.extract {
		// tar: You must specify one of the '-Acdtrux' options
		return "", processError(tarPath, tarFailure)
	}
	if opts.operands {
		// Extracting named members, which tar then does into the working
		// directory instead of dest, is not something to reproduce. The usual
		// outcome of a mistaken flag (a name that is in no archive) is the same
		// failure.
		tarWarnf("named members are not supported")
		return "", processError(tarPath, tarFailure)
	}
	ok, err := extractTarFile(file, dest, opts)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", processError(tarPath, tarFailure)
	}
	return dest, nil
}

// tarWarnf reports a problem on the error output, where tar does.
func tarWarnf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "tar: "+format+"\n", args...)
}

// includesV mirrors flags.includes('v') of the reference: a substring test for
// the string form, a membership test for the array form.
func includesV(flags []string) bool {
	if len(flags) == 1 {
		return strings.Contains(flags[0], "v")
	}
	for _, f := range flags {
		if f == "v" {
			return true
		}
	}
	return false
}

type tarOptions struct {
	extract, gzip, bzip2, verbose bool
	sameOwner, samePermissions    bool
	strip                         int
	gnu                           bool
	// operands is set when members to extract are named
	operands bool
}

func parseTarFlags(flags []string) (tarOptions, error) {
	root := os.Geteuid() == 0
	opts := tarOptions{sameOwner: root, samePermissions: root}
	for i := 0; i < len(flags); i++ {
		f := flags[i]
		switch {
		case f == "--strip-components" && i+1 < len(flags):
			i++
			n, err := strconv.Atoi(flags[i])
			if err != nil || n < 0 {
				return opts, fmt.Errorf("tar: invalid number for --strip-components: %q", flags[i])
			}
			opts.strip = n
		case strings.HasPrefix(f, "--strip-components="):
			n, err := strconv.Atoi(strings.TrimPrefix(f, "--strip-components="))
			if err != nil || n < 0 {
				return opts, fmt.Errorf("tar: invalid number for %s", f)
			}
			opts.strip = n
		case f == "--extract" || f == "--get":
			opts.extract = true
		case f == "--gzip" || f == "--gunzip" || f == "--ungzip":
			opts.gzip = true
		case f == "--bzip2":
			opts.bzip2 = true
		case f == "--verbose":
			opts.verbose = true
		case f == "--no-same-owner":
			opts.sameOwner = false
		case f == "--same-owner":
			opts.sameOwner = true
		case f == "--no-same-permissions":
			opts.samePermissions = false
		case f == "--same-permissions" || f == "--preserve-permissions":
			opts.samePermissions = true
		case strings.HasPrefix(f, "--"):
			return opts, fmt.Errorf("unsupported tar option %q: supported are x, z, j, v, p, m, --strip-components, --[no-]same-owner and --[no-]same-permissions", f)
		case i > 0 && !strings.HasPrefix(f, "-"):
			// only the first argument is a bundle of letters: the others select
			// the members to extract, after which the -C and -f that follow
			// have no effect
			opts.operands = true
		default:
			for _, letter := range strings.TrimPrefix(f, "-") {
				switch letter {
				case 'x':
					opts.extract = true
				case 'z':
					opts.gzip = true
				case 'j':
					opts.bzip2 = true
				case 'v':
					opts.verbose = true
				case 'p':
					opts.samePermissions = true
				case 'm':
					// the modification time is not part of what is compared and is
					// restored anyway
				default:
					return opts, fmt.Errorf("unsupported tar option %q in %q: supported are x, z, j, v, p, m, --strip-components, --[no-]same-owner and --[no-]same-permissions", string(letter), f)
				}
			}
		}
	}
	return opts, nil
}

// openTar opens the archive, decompressed according to the flags and to what
// the file contains. The returned bool is false when tar would have failed on
// the content.
func openTar(file string, opts tarOptions) (io.Reader, func(), bool, error) {
	f, err := os.Open(file)
	if err != nil {
		tarWarnf("%s: Cannot open: %v", file, err)
		return nil, nil, false, nil
	}
	br := bufio.NewReader(f)
	magic, _ := br.Peek(6)
	closer := func() { f.Close() }
	switch {
	case bytes.HasPrefix(magic, []byte{0x1f, 0x8b}):
		if opts.bzip2 {
			return nil, closer, false, nil
		}
		zr, err := gzip.NewReader(br)
		if err != nil {
			return nil, closer, false, nil
		}
		return zr, closer, true, nil
	case bytes.HasPrefix(magic, []byte("BZh")):
		if opts.gzip {
			return nil, closer, false, nil
		}
		return bzip2.NewReader(br), closer, true, nil
	case bytes.HasPrefix(magic, []byte{0xfd, '7', 'z', 'X', 'Z', 0}):
		return nil, closer, false, errors.New("unsupported tar archive: xz compressed archives cannot be extracted by the Go port")
	case bytes.HasPrefix(magic, []byte{0x28, 0xb5, 0x2f, 0xfd}):
		return nil, closer, false, errors.New("unsupported tar archive: zstd compressed archives cannot be extracted by the Go port")
	}
	if opts.gzip || opts.bzip2 {
		// gzip: stdin: not in gzip format
		return nil, closer, false, nil
	}
	return br, closer, true, nil
}

// extractTarFile extracts file below dest. The bool is false when tar would
// have exited with a failure.
func extractTarFile(file, dest string, opts tarOptions) (bool, error) {
	src, closeSrc, ok, err := openTar(file, opts)
	if closeSrc != nil {
		defer closeSrc()
	}
	if err != nil || !ok {
		return false, err
	}
	root, err := newExtractRoot(dest)
	if err != nil {
		return false, err
	}
	x := &tarExtraction{root: root, opts: opts, umask: umask()}
	ok = true
	counted := &countingReader{r: src}
	tr := tar.NewReader(counted)
	for {
		h, err := tr.Next()
		if err == io.EOF && x.pending != nil && counted.n-x.dataStart < (x.pending.written+tarBlock-1)/tarBlock*tarBlock {
			// the archive ends inside the last block of the file
			x.settle(io.ErrUnexpectedEOF)
			ok = false
			break
		}
		x.settle(err)
		x.dataStart = counted.n
		if err == io.EOF {
			// tar reads the stream to its end, which is where a cut
			// compressed stream is noticed; an empty file is not an archive
			if _, err := io.Copy(io.Discard, counted); err != nil || counted.n == 0 {
				ok = false
			}
			break
		}
		if err != nil {
			// not a tar archive, or cut short
			ok = false
			break
		}
		memberOK, fatal := x.extract(h, tr)
		ok = ok && memberOK
		if fatal {
			break
		}
	}
	x.applyDirectories()
	return ok, nil
}

// countingReader counts the bytes read.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

type tarExtraction struct {
	root    extractRoot
	opts    tarOptions
	umask   os.FileMode
	failed  bool
	pending *pendingFile
	// dataStart is the offset in the (decompressed) archive of the content of
	// the member being extracted
	dataStart int64
	dirs      []deferredDir
	// extracted maps the archive name of members to where they went, for hard
	// links
	extracted map[string]string
}

// memberName turns an archive name into a path relative to the destination,
// like tar does: leading slashes are removed, "." parts ignored and
// --strip-components applied. ok is false for a member to ignore (nothing is
// left of the name), dotdot when the name contains "..".
func memberName(name string, strip int) (rel string, ok, dotdot bool) {
	var parts []string
	for _, p := range strings.Split(name, "/") {
		if p != "" && p != "." {
			parts = append(parts, p)
		}
	}
	if len(parts) <= strip && strip > 0 {
		return "", false, false
	}
	parts = parts[strip:]
	for _, p := range parts {
		if p == ".." {
			return "", false, true
		}
	}
	return filepath.Join(parts...), true, false
}

// mode is the mode a member ends with.
func (x *tarExtraction) mode(h *tar.Header) os.FileMode {
	m := os.FileMode(h.Mode).Perm()
	if h.Mode&0o4000 != 0 {
		m |= os.ModeSetuid
	}
	if h.Mode&0o2000 != 0 {
		m |= os.ModeSetgid
	}
	if h.Mode&0o1000 != 0 {
		m |= os.ModeSticky
	}
	if !x.opts.samePermissions {
		m &^= x.umask
		m &^= os.ModeSetuid | os.ModeSetgid | os.ModeSticky
	}
	return m
}

func (x *tarExtraction) finish(path string, h *tar.Header, mtime bool) {
	if x.opts.sameOwner {
		_ = os.Lchown(path, h.Uid, h.Gid)
	}
	_ = os.Chmod(path, x.mode(h))
	if mtime && !h.ModTime.IsZero() {
		_ = os.Chtimes(path, h.ModTime, h.ModTime)
	}
}

func (x *tarExtraction) list(name string) {
	if !x.opts.verbose {
		return
	}
	if x.opts.gnu {
		core.Info(name)
		return
	}
	// bsdtar lists on the error output
	fmt.Fprintln(os.Stderr, "x "+name)
}

// extract writes one member. ok is false when the member could not be written
// and fatal when the extraction cannot go on.
func (x *tarExtraction) extract(h *tar.Header, content io.Reader) (ok, fatal bool) {
	if h.Typeflag == tar.TypeXGlobalHeader {
		return true, false
	}
	rel, keep, dotdot := memberName(h.Name, x.opts.strip)
	if dotdot {
		tarWarnf("%s: Member name contains '..'", h.Name)
		return false, false
	}
	if !keep {
		return true, false
	}
	x.list(strings.TrimLeft(h.Name, "/"))
	parent, err := x.root.mkdirParents(filepath.Dir(rel))
	if err != nil {
		tarWarnf("%s: %v", h.Name, err)
		return false, false
	}
	path := filepath.Join(parent, filepath.Base(rel))
	if rel == "" {
		path = x.root.dir
	}
	switch h.Typeflag {
	case tar.TypeDir:
		return x.directory(path, h), false
	case tar.TypeReg, tar.TypeRegA:
		return x.file(path, rel, h, content)
	case tar.TypeSymlink:
		return x.symlink(path, parent, h), false
	case tar.TypeLink:
		return x.hardlink(path, h), false
	default:
		tarWarnf("%s: Cannot extract members of type %q", h.Name, string(h.Typeflag))
		return false, false
	}
}

func (x *tarExtraction) directory(path string, h *tar.Header) bool {
	if info, err := os.Lstat(path); err == nil && !info.IsDir() {
		if err := os.Remove(path); err != nil {
			tarWarnf("%s: %v", h.Name, err)
			return false
		}
	}
	if err := os.Mkdir(path, cachePerms); err != nil && !errors.Is(err, os.ErrExist) {
		tarWarnf("%s: %v", h.Name, err)
		return false
	}
	if x.opts.sameOwner {
		_ = os.Lchown(path, h.Uid, h.Gid)
	}
	x.dirs = append(x.dirs, deferredDir{path: path, mode: x.mode(h), mtime: h.ModTime})
	return true
}

func (x *tarExtraction) applyDirectories() { applyDirectories(x.dirs) }

// removeEntry removes what is at path unless it is a directory.
func removeEntry(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	return os.Remove(path)
}

func (x *tarExtraction) file(path, rel string, h *tar.Header, content io.Reader) (ok, fatal bool) {
	// an existing regular file is overwritten in place, anything else replaced
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		if err := removeEntry(path); err != nil {
			tarWarnf("%s: %v", h.Name, err)
			return false, false
		}
	}
	out, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		tarWarnf("%s: %v", h.Name, err)
		return false, false
	}
	written, err := io.Copy(out, content)
	if err != nil {
		// Unexpected EOF in archive
		x.pending = &pendingFile{path: path, rel: rel, h: h, written: written}
		x.settle(err)
		out.Close()
		return false, true
	}
	if err := out.Close(); err != nil {
		return false, true
	}
	// the mode is set once the padding that completes the last block is read
	x.pending = &pendingFile{path: path, rel: rel, h: h, written: written}
	return true, false
}

// pendingFile is a file whose content is written but whose last block of the
// archive was not read yet.
type pendingFile struct {
	path, rel string
	h         *tar.Header
	written   int64
}

// settle ends the extraction of the last file given what was read after it:
// nil or io.EOF when the archive goes on. tar works on blocks, so a file whose
// last block is cut keeps the complete blocks only, with the mode it was
// created with.
func (x *tarExtraction) settle(err error) {
	p := x.pending
	x.pending = nil
	if p == nil {
		return
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		_ = os.Truncate(p.path, p.written/tarBlock*tarBlock)
		if !x.opts.samePermissions {
			// without the permissions of the archive tar still applies the mode
			_ = os.Chmod(p.path, x.mode(p.h))
		}
		return
	}
	x.finish(p.path, p.h, true)
	if x.extracted == nil {
		x.extracted = map[string]string{}
	}
	x.extracted[p.rel] = p.path
}

func (x *tarExtraction) symlink(path, parent string, h *tar.Header) bool {
	if !x.root.safeLink(parent, h.Linkname) {
		tarWarnf("%s: refusing to create a symbolic link to %s outside of the destination", h.Name, h.Linkname)
		return false
	}
	if err := removeEntry(path); err != nil {
		tarWarnf("%s: %v", h.Name, err)
		return false
	}
	if err := os.Symlink(h.Linkname, path); err != nil {
		tarWarnf("%s: %v", h.Name, err)
		return false
	}
	if x.opts.sameOwner {
		_ = os.Lchown(path, h.Uid, h.Gid)
	}
	return true
}

func (x *tarExtraction) hardlink(path string, h *tar.Header) bool {
	rel, keep, dotdot := memberName(h.Linkname, x.opts.strip)
	if dotdot || !keep {
		tarWarnf("%s: Cannot hard link to %s", h.Name, h.Linkname)
		return false
	}
	target, found := x.extracted[rel]
	if !found {
		// a link to a member that is not a regular file written by this run
		target = filepath.Join(x.root.dir, rel)
		if resolved, err := filepath.EvalSymlinks(target); err != nil || !isInside(x.root.dir, resolved) {
			tarWarnf("%s: Cannot hard link to %s", h.Name, h.Linkname)
			return false
		}
	}
	if err := removeEntry(path); err != nil {
		tarWarnf("%s: %v", h.Name, err)
		return false
	}
	if err := os.Link(target, path); err != nil {
		tarWarnf("%s: %v", h.Name, err)
		return false
	}
	return true
}
