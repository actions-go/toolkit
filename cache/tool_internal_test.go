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
	assert.Regexp(t, regexp.MustCompile("^temp[/\\][0-9a-f-]{36}$"), destination(&DownloadToolOptions{}))
	assert.Regexp(t, regexp.MustCompile("^temp[/\\][0-9a-f-]{36}$"), destination(nil))
}

func TestEnsureDestDir(t *testing.T) {
	assert.NoError(t, ensureDestDir(""))
	dir := "test-" + uuid.New().String()
	defer os.Remove(dir)
	assert.NoError(t, ensureDestDir(filepath.Join(dir, "some-file")))
	assert.NoError(t, ensureDestDir(filepath.Join(dir, "some-file")))
	assert.Error(t, ensureDestDir(filepath.Join("some-non-existing-dir", "some-file")))
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
	assert.Equal(t, filepath.Join(cacheRoot, "some-tool", "0.1.0"), path)
	assert.FileExists(t, filepath.Join(path, "my-tool.go"))

	path, err = CacheFile("../core", "source", CacheOptions{Tool: "some-other-tool", Version: "0.1.0"})
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(cacheRoot, "some-other-tool", "0.1.0"), path)
	assert.FileExists(t, filepath.Join(path, "source", "core.go"))
	assert.FileExists(t, path+".complete")
}

func TestListAllCachedVersions(t *testing.T) {
	cacheRoot = "test-cache-root-" + uuid.New().String()
	defer os.RemoveAll(cacheRoot)

	assert.Equal(t, []string{}, ListAllCachedVersions(CacheOptions{Tool: "some-tool"}))
	os.MkdirAll(filepath.Join(cacheRoot, "some-tool", "1.203.2", "x86"), cachePerms)
	os.MkdirAll(filepath.Join(cacheRoot, "some-tool", "1.204.2", "x86"), cachePerms)
	os.MkdirAll(filepath.Join(cacheRoot, "some-tool", "1.205.2", "386"), cachePerms)
	versions := ListAllCachedVersions(CacheOptions{Tool: "some-tool"})
	assert.Len(t, versions, 3)
	assert.Contains(t, versions, "1.203.2")
	assert.Contains(t, versions, "1.204.2")
	assert.Contains(t, versions, "1.205.2")

	versions = ListAllCachedVersions(CacheOptions{Tool: "some-tool", Arch: "386"})
	assert.Len(t, versions, 1)
	assert.Contains(t, versions, "1.205.2")
}

func TestFindVersions(t *testing.T) {
	cacheRoot = "test-cache-root-" + uuid.New().String()
	defer os.RemoveAll(cacheRoot)
	os.MkdirAll(filepath.Join(cacheRoot, "some-tool", "1.203.2", "x86"), cachePerms)
	os.MkdirAll(filepath.Join(cacheRoot, "some-tool", "1.204.2", "x86"), cachePerms)
	os.MkdirAll(filepath.Join(cacheRoot, "some-tool", "1.205.2", "386"), cachePerms)
	os.MkdirAll(filepath.Join(cacheRoot, "some-tool", "1.205.2", "x86"), cachePerms)
	os.MkdirAll(filepath.Join(cacheRoot, "some-tool", "1.205.3", "x86"), cachePerms)

	path, err := FindVersion(CacheOptions{Tool: "some-tool"})
	assert.Error(t, err)
	assert.Equal(t, "", path)

	path, err = FindVersion(CacheOptions{Tool: "some-tool", Version: "~1.205"})
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(cacheRoot, "some-tool", "1.205.3"), path)

	path, err = FindVersion(CacheOptions{Tool: "some-tool", Version: "~1.205", Arch: "386"})
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(cacheRoot, "some-tool", "1.205.2", "386"), path)
}

func TestCacheDir(t *testing.T) {
	cacheRoot = "test-cache-root-" + uuid.New().String()
	defer os.RemoveAll(cacheRoot)

	path, err := CacheDir("../core", CacheOptions{Tool: "some-other-tool", Version: "0.1.0"})
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(cacheRoot, "some-other-tool", "0.1.0"), path)
	assert.FileExists(t, filepath.Join(path, "core.go"))
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
