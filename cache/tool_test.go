package cache_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/actions-go/toolkit/cache"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestDownloadTool(t *testing.T) {
	data := "hello-world"
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(data)) }))
	defer s.Close()
	testID := uuid.New().String()
	cacheDir := "./temp-" + testID
	defer os.RemoveAll(cacheDir)
	cache.SetTempDir(cacheDir)

	f, err := cache.DownloadTool(s.URL, nil)
	assert.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(fmt.Sprintf("^temp-%s[/\\][0-9a-f-]{36}", testID)), f)
	_, err = os.Stat(f)
	assert.NoError(t, err)
	bytes, err := ioutil.ReadFile(f)
	assert.NoError(t, err)
	assert.Equal(t, data, string(bytes))
}

func TestGetCachedToolOrDownload(t *testing.T) {
	data := "hello-world"
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(data)) }))
	defer s.Close()
	testID := uuid.New().String()
	tempDir := "./temp-" + testID
	cacheDir := "./test-cache-" + testID
	defer os.RemoveAll(tempDir)
	defer os.RemoveAll(cacheDir)
	cache.SetTempDir(tempDir)
	cache.SetCacheRoot(cacheDir)

	f, err := cache.GetCachedToolOrDownload(cache.CacheOptions{Tool: "my-tool", Version: "1.0.1"}, &cache.DownloadToolOptions{}, s.URL)
	assert.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(fmt.Sprintf("^temp-%s[/\\][0-9a-f-]{36}", testID)), f)
	_, err = os.Stat(f)
	assert.NoError(t, err)
	bytes, err := ioutil.ReadFile(f)
	assert.NoError(t, err)
	assert.Equal(t, data, string(bytes))

	s.Close()

	f, err = cache.GetCachedToolOrDownload(cache.CacheOptions{Tool: "my-tool", Version: "1.0.1"}, &cache.DownloadToolOptions{}, s.URL)
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join("test-cache-"+testID, "my-tool", "1.0.1", "my-tool"), f)
	_, err = os.Stat(f)
	assert.NoError(t, err)
	bytes, err = ioutil.ReadFile(f)
	assert.NoError(t, err)
	assert.Equal(t, data, string(bytes))
}
