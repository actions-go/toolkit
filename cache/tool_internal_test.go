package cache

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// SetTempDir a helper for easier testing
func SetTempDir(d string) {
	tempDirectory = d
}

// SetCacheRoot a helper for easier testing
func SetCacheRoot(d string) {
	cacheRoot = d
}

func TestDestination(t *testing.T) {
	defer SetTempDir(tempDirectory)
	SetTempDir("./temp")
	assert.Equal(t, "hello-world", destination(&DownloadToolOptions{Destination: "hello-world"}))
	assert.Regexp(t, regexp.MustCompilePOSIX(`^temp[/\\][0-9a-f-]{36}$`), destination(&DownloadToolOptions{}))
	assert.Regexp(t, regexp.MustCompilePOSIX(`^temp[/\\][0-9a-f-]{36}$`), destination(nil))
}

func TestEnsureDestDir(t *testing.T) {
	assert.NoError(t, ensureDestDir(""))
	dir := "test-" + uuid.New().String()
	defer os.Remove(dir)
	assert.NoError(t, ensureDestDir(filepath.Join(dir, "some-file")))
	assert.NoError(t, ensureDestDir(filepath.Join(dir, "some-file")))
	if runtime.GOOS != "windows" {
		assert.Error(t, ensureDestDir("/some-non-existing-dir/some-file"))
	}
}

func TestEnsureDestNotExists(t *testing.T) {
	assert.NoError(t, ensureDestNotExists("sone-non-existing-file"))
	assert.Error(t, ensureDestNotExists("tool.go"))
}

func TestToolPath(t *testing.T) {
	cacheRoot = "./cache"
	assert.Equal(t, "cache", toolPath(CacheOptions{}))
	assert.Equal(
		t,
		filepath.Join("cache", "some-tool", "version", "arch"),
		toolPath(CacheOptions{
			Tool:    "some-tool",
			Version: "version",
			Arch:    "arch",
		}),
	)
}

func TestDefaultOptions(t *testing.T) {
	_, err := defaultOptions(CacheOptions{})
	assert.Error(t, err)
	_, err = defaultOptions(CacheOptions{Tool: "my-tool"})
	assert.Error(t, err)
	options, err := defaultOptions(CacheOptions{Tool: "my-tool", Version: "0.1.0"})
	assert.NoError(t, err)
	assert.Equal(t, CacheOptions{Tool: "my-tool", Version: "0.1.0", Arch: jsArch(), UseJavascriptValues: Bool(true)}, options)
	options, err = defaultOptions(CacheOptions{Tool: "my-tool", Version: "0.1.0", UseJavascriptValues: Bool(false)})
	assert.NoError(t, err)
	assert.Equal(t, CacheOptions{Tool: "my-tool", Version: "0.1.0", Arch: runtime.GOARCH, UseJavascriptValues: Bool(false)}, options)
	options, err = defaultOptions(CacheOptions{Tool: "my-tool", Version: "0.1.0", UseJavascriptValues: Bool(false), Arch: "custom-arch"})
	assert.NoError(t, err)
	assert.Equal(t, CacheOptions{Tool: "my-tool", Version: "0.1.0", Arch: "custom-arch", UseJavascriptValues: Bool(false)}, options)
}

func TestCacheFile(t *testing.T) {
	cacheRoot = "test-cache-root-" + uuid.New().String()
	defer os.RemoveAll(cacheRoot)

	path, err := CacheFile("tool.go", "my-tool.go", CacheOptions{})
	assert.Error(t, err)
	assert.Equal(t, "", path)

	path, err = CacheFile("tool.go", "my-tool.go", CacheOptions{Tool: "some-tool", Version: "0.1.0"})
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(cacheRoot, "some-tool", "0.1.0", jsArch()), path)
	assert.FileExists(t, filepath.Join(path, "my-tool.go"))
	assert.FileExists(t, path+".complete")

	path, err = CacheFile("../core", "source", CacheOptions{Tool: "some-other-tool", Version: "0.1.0"})
	assert.EqualError(t, err, "sourceFile is not a file")
	assert.Equal(t, "", path)
}

func TestListAllCachedVersions(t *testing.T) {
	cacheRoot = "test-cache-root-" + uuid.New().String()
	defer os.RemoveAll(cacheRoot)

	assert.Equal(t, []string{}, ListAllCachedVersions(CacheOptions{Tool: "some-tool"}))
	fakeCached(t, "some-tool", "1.203.2", "x86")
	fakeCached(t, "some-tool", "1.204.2", "x86")
	fakeCached(t, "some-tool", "1.205.2", "386")
	// not completed, or not a version: not listed
	os.MkdirAll(filepath.Join(cacheRoot, "some-tool", "1.206.0", "386"), cachePerms)
	fakeCached(t, "some-tool", "latest", "386")

	versions := ListAllCachedVersions(CacheOptions{Tool: "some-tool", Arch: "x86"})
	assert.ElementsMatch(t, []string{"1.203.2", "1.204.2"}, versions)

	versions = ListAllCachedVersions(CacheOptions{Tool: "some-tool", Arch: "386"})
	assert.Equal(t, []string{"1.205.2"}, versions)

	fakeCached(t, "some-tool", "1.207.0", jsArch())
	assert.Equal(t, []string{"1.207.0"}, ListAllCachedVersions(CacheOptions{Tool: "some-tool"}))
}

// fakeCached creates the folder and the marker of a cached tool version.
func fakeCached(t *testing.T, tool, version, arch string) {
	t.Helper()
	folder := filepath.Join(cacheRoot, tool, version, arch)
	assert.NoError(t, os.MkdirAll(folder, cachePerms))
	assert.NoError(t, os.WriteFile(folder+".complete", nil, 0o644))
}

func TestFindVersions(t *testing.T) {
	cacheRoot = "test-cache-root-" + uuid.New().String()
	defer os.RemoveAll(cacheRoot)
	fakeCached(t, "some-tool", "1.203.2", "x86")
	fakeCached(t, "some-tool", "1.204.2", "x86")
	fakeCached(t, "some-tool", "1.205.2", "386")
	fakeCached(t, "some-tool", "1.205.2", "x86")
	fakeCached(t, "some-tool", "1.205.3", "x86")

	path, err := FindVersion(CacheOptions{Tool: "some-tool"})
	assert.EqualError(t, err, "versionSpec parameter is required")
	assert.Equal(t, "", path)

	path, err = FindVersion(CacheOptions{Version: "1.x"})
	assert.EqualError(t, err, "toolName parameter is required")
	assert.Equal(t, "", path)

	path, err = FindVersion(CacheOptions{Tool: "some-tool", Version: "~1.205", Arch: "x86"})
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(cacheRoot, "some-tool", "1.205.3", "x86"), path)

	path, err = FindVersion(CacheOptions{Tool: "some-tool", Version: "~1.205", Arch: "386"})
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(cacheRoot, "some-tool", "1.205.2", "386"), path)

	path, err = FindVersion(CacheOptions{Tool: "some-tool", Version: "1.205.2", Arch: "386"})
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(cacheRoot, "some-tool", "1.205.2", "386"), path)

	path, err = FindVersion(CacheOptions{Tool: "some-tool", Version: "2.x", Arch: "386"})
	assert.EqualError(t, err, "could not find any cached version for some-tool matching 2.x")
	assert.Equal(t, "", path)
}

func TestCacheDir(t *testing.T) {
	cacheRoot = "test-cache-root-" + uuid.New().String()
	defer os.RemoveAll(cacheRoot)

	path, err := CacheDir("../core", CacheOptions{Tool: "some-other-tool", Version: "0.1.0"})
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(cacheRoot, "some-other-tool", "0.1.0", jsArch()), path)
	assert.FileExists(t, filepath.Join(path, "core.go"))
	assert.FileExists(t, path+".complete")

	path, err = CacheDir("tool.go", CacheOptions{Tool: "some-other-tool", Version: "0.1.0"})
	assert.EqualError(t, err, "sourceDir is not a directory")
	assert.Equal(t, "", path)
}

func TestCleanSemver(t *testing.T) {
	for in, want := range map[string]string{
		"1.2.3":            "1.2.3",
		"v1.2.3":           "1.2.3",
		"=v1.2.3":          "1.2.3",
		"  1.2.3 ":         "1.2.3",
		"1.2.3+build":      "1.2.3",
		"1.2.3-rc.1+b":     "1.2.3-rc.1",
		"1.2":              "1.2",
		"latest":           "latest",
		"01.2.3":           "01.2.3",
		"1.2.3-01":         "1.2.3-01",
		"V1.2.3":           "V1.2.3",
		"1.2.3.4":          "1.2.3.4",
		"1.2.3-alpha.beta": "1.2.3-alpha.beta",
	} {
		assert.Equal(t, want, cleanSemver(in), in)
	}
}

func TestCopyURL(t *testing.T) {
	data := "hello-world"
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(data)) }))
	defer s.Close()
	b := bytes.NewBuffer(nil)
	assert.NoError(t, copyURL(b, s.URL))
	assert.Equal(t, data, b.String())

	assert.Error(t, copyURL(nil, s.URL))

	assert.Error(t, copyURL(b, "this is not a URL"))

	assert.Error(t, copyURL(writerInError{}, s.URL))

	s.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotAcceptable) })
	assert.Error(t, copyURL(bytes.NewBuffer(nil), s.URL))
}

type writerInError struct {
}

func (writerInError) Write([]byte) (int, error) {
	return 0, errors.New("test-error")
}
