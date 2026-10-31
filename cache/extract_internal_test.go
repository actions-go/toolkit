package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemberName(t *testing.T) {
	rel, keep, dotdot := memberName("./a/./b/", 0)
	assert.Equal(t, filepath.Join("a", "b"), rel)
	assert.True(t, keep)
	assert.False(t, dotdot)

	rel, keep, _ = memberName("/etc/passwd", 0)
	assert.Equal(t, filepath.Join("etc", "passwd"), rel, "the leading slash is removed")
	assert.True(t, keep)

	_, _, dotdot = memberName("a/../b", 0)
	assert.True(t, dotdot)

	rel, keep, _ = memberName("top/a/b", 1)
	assert.Equal(t, filepath.Join("a", "b"), rel)
	assert.True(t, keep)

	_, keep, _ = memberName("top/", 1)
	assert.False(t, keep, "nothing is left once the components are stripped")

	rel, keep, _ = memberName("./", 0)
	assert.Equal(t, "", rel)
	assert.True(t, keep, "the destination itself")
}

func TestParseTarFlags(t *testing.T) {
	opts, err := parseTarFlags([]string{"xzv"})
	require.NoError(t, err)
	assert.True(t, opts.extract && opts.gzip && opts.verbose)

	opts, err = parseTarFlags([]string{"xj", "--strip-components", "2"})
	require.NoError(t, err)
	assert.True(t, opts.bzip2)
	assert.Equal(t, 2, opts.strip)

	opts, err = parseTarFlags([]string{"xz", "v"})
	require.NoError(t, err)
	assert.True(t, opts.operands, "a second bundle is a member name")

	_, err = parseTarFlags([]string{"xJ"})
	assert.ErrorContains(t, err, "unsupported tar option")
	_, err = parseTarFlags([]string{"xz", "--zstd"})
	assert.ErrorContains(t, err, "unsupported tar option")
	_, err = parseTarFlags([]string{"xz", "--strip-components=x"})
	assert.Error(t, err)
}

func TestExtractRootMkdirParentsRefusesEscapingSymlinks(t *testing.T) {
	base := t.TempDir()
	outside := filepath.Join(base, "outside")
	require.NoError(t, os.MkdirAll(outside, 0o755))
	dest := filepath.Join(base, "dest")
	require.NoError(t, os.MkdirAll(filepath.Join(dest, "real"), 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(dest, "out")))
	require.NoError(t, os.Symlink("real", filepath.Join(dest, "in")))
	root, err := newExtractRoot(dest)
	require.NoError(t, err)

	_, err = root.mkdirParents(filepath.Join("out", "new"))
	assert.ErrorIs(t, err, errOutside)
	assert.NoDirExists(t, filepath.Join(outside, "new"))

	parent, err := root.mkdirParents(filepath.Join("in", "new"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root.dir, "real", "new"), parent)
}

func TestZipName(t *testing.T) {
	rel, dir := zipName("/a/../b/./c/")
	assert.Equal(t, filepath.Join("a", "b", "c"), rel, "unzip drops the leading slash and the .. parts")
	assert.True(t, dir)

	rel, dir = zipName("file.txt")
	assert.Equal(t, "file.txt", rel)
	assert.False(t, dir)

	rel, _ = zipName("../..")
	assert.Equal(t, "", rel)
}

func TestSafeLink(t *testing.T) {
	root := extractRoot{dir: "/dest"}
	assert.True(t, root.safeLink("/dest/d", "../a"))
	assert.True(t, root.safeLink("/dest", "a/b"))
	assert.False(t, root.safeLink("/dest", "../a"))
	assert.False(t, root.safeLink("/dest/d", "/etc/passwd"))
	assert.False(t, root.safeLink("/dest/d", "../../etc"))
}
