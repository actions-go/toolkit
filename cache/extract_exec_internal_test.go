//go:build !windows

package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTool writes a shell script that records its arguments and working
// directory in <dir>/record and exits with code.
func fakeTool(t *testing.T, dir, name string, code string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\nprintf '%s\\n' \"$PWD\" \"$@\" > \"" + filepath.Join(dir, "record") + "\"\necho listed\nexit " + code + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

func onHost(t *testing.T, goos string) {
	t.Helper()
	old := hostOS
	hostOS = goos
	t.Cleanup(func() { hostOS = old })
}

func TestUnsupportedSystems(t *testing.T) {
	onHost(t, "linux")
	_, err := Extract7z("a.7z", "", "")
	assert.EqualError(t, err, "extract7z() not supported on current OS")
	_, err = ExtractXar("a.xar", "")
	assert.EqualError(t, err, "extractXar() not supported on current OS")
}

func TestSystemIsCheckedBeforeFile(t *testing.T) {
	onHost(t, "linux")
	_, err := ExtractXar("", "")
	assert.EqualError(t, err, "extractXar() not supported on current OS")
	onHost(t, "darwin")
	_, err = ExtractXar("", "")
	assert.EqualError(t, err, `parameter "file" is required`)
	onHost(t, "windows")
	_, err = Extract7z("", "", "")
	assert.EqualError(t, err, `parameter "file" is required`)
}

func TestExtract7zRunsTheGivenToolInTheDestination(t *testing.T) {
	onHost(t, "windows")
	dir := t.TempDir()
	tool := fakeTool(t, dir, "7zr", "0")
	dest := filepath.Join(dir, "out")

	got, err := Extract7z("a.7z", dest, tool)

	require.NoError(t, err)
	assert.Equal(t, dest, got)
	record, _ := os.ReadFile(filepath.Join(dir, "record"))
	resolved, _ := filepath.EvalSymlinks(dest)
	assert.Equal(t, resolved+"\nx\n-bb0\n-bd\n-sccUTF-8\na.7z\n", string(record))
}

func TestExtract7zReportsAFailureCode(t *testing.T) {
	onHost(t, "windows")
	dir := t.TempDir()
	tool := fakeTool(t, dir, "7zr", "2")

	_, err := Extract7z("a.7z", filepath.Join(dir, "out"), tool)

	assert.EqualError(t, err, "The process '"+tool+"' failed with exit code 2")
}

func TestExtract7zLogsMoreWhenDebugging(t *testing.T) {
	onHost(t, "windows")
	t.Setenv("RUNNER_DEBUG", "1")
	dir := t.TempDir()
	tool := fakeTool(t, dir, "7zr", "0")

	_, err := Extract7z("a.7z", filepath.Join(dir, "out"), tool)

	require.NoError(t, err)
	record, _ := os.ReadFile(filepath.Join(dir, "record"))
	assert.Contains(t, string(record), "\n-bb1\n")
}

func TestExtractXarRunsXarFromPath(t *testing.T) {
	onHost(t, "darwin")
	bin := t.TempDir()
	fakeTool(t, bin, "xar", "0")
	t.Setenv("PATH", bin)
	dest := filepath.Join(t.TempDir(), "out")

	got, err := ExtractXar("a.xar", dest, "-x", "--noop")

	require.NoError(t, err)
	assert.Equal(t, dest, got)
	record, _ := os.ReadFile(filepath.Join(bin, "record"))
	assert.Contains(t, string(record), "\n-x\n--noop\n-C\n"+dest+"\n-f\na.xar\n", "the repeated -x is dropped")
	assert.NotContains(t, string(record), "-v")
}

func TestExtractXarWithoutXar(t *testing.T) {
	onHost(t, "darwin")
	t.Setenv("PATH", t.TempDir())

	_, err := ExtractXar("a.xar", filepath.Join(t.TempDir(), "out"))

	assert.ErrorContains(t, err, "Unable to locate executable file: xar.")
}

func TestUnique(t *testing.T) {
	assert.Equal(t, []string{"-x", "-C", "d"}, unique([]string{"-x", "-C", "-x", "d", "-C"}))
}
