package cache_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tjamet/go-github-action-toolkit/cache"
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
	assert.Regexp(t, regexp.MustCompile(fmt.Sprintf("^temp-%s/[0-9a-f-]{36}", testID)), f)
	_, err = os.Stat(f)
	assert.NoError(t, err)
	bytes, err := ioutil.ReadFile(f)
	assert.NoError(t, err)
	assert.Equal(t, data, string(bytes))
}
