package cache

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/actions-go/toolkit/core"
	"github.com/google/uuid"
)

const (
	userAgent           = "actions/tool-cache"
	downloadMaxAttempts = 3
	// retry delays in seconds, overridable like the TEST_DOWNLOAD_TOOL_RETRY_*
	// globals of @actions/tool-cache so that tests do not wait
	retryMinSecondsEnv     = "TEST_DOWNLOAD_TOOL_RETRY_MIN_SECONDS"
	retryMaxSecondsEnv     = "TEST_DOWNLOAD_TOOL_RETRY_MAX_SECONDS"
	defaultRetryMinSeconds = 10
	defaultRetryMaxSeconds = 20
)

// maxRedirects is the number of redirects followed, like @actions/http-client.
const maxRedirects = 50

// httpClient follows up to maxRedirects redirects, then hands the last
// redirect response back so that it is reported as an unexpected status.
var httpClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

// httpError is an unexpected status of the server.
type httpError struct{ statusCode int }

func (e httpError) Error() string {
	return fmt.Sprintf("Unexpected HTTP response: %d", e.statusCode)
}

// retryable tells whether a failed download attempt is worth another one:
// anything but a client error, except 408 Request Timeout and 429 Too Many Requests.
func retryable(err error) bool {
	if e, ok := err.(httpError); ok && e.statusCode != 0 {
		return e.statusCode >= 500 || e.statusCode == http.StatusRequestTimeout || e.statusCode == http.StatusTooManyRequests
	}
	return true
}

func envSeconds(name string, dflt int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v >= 0 {
		return v
	}
	return dflt
}

// retrySeconds picks the wait before the next attempt.
func retrySeconds() int {
	min, max := envSeconds(retryMinSecondsEnv, defaultRetryMinSeconds), envSeconds(retryMaxSecondsEnv, defaultRetryMaxSeconds)
	if max < min {
		max = min
	}
	return min + rand.Intn(max-min+1)
}

func destination(options *DownloadToolOptions) string {
	if options != nil && options.Destination != "" {
		return options.Destination
	}
	return filepath.Join(tempDir(), uuid.New().String())
}

func ensureDestDir(dest string) error {
	destDir := filepath.Dir(dest)
	if err := os.MkdirAll(destDir, cachePerms); err != nil {
		return fmt.Errorf("Unable to create destination directory %s: %v", destDir, err)
	}
	return nil
}

// downloadAttempt downloads url into the not yet existing file dest.
func downloadAttempt(url, dest string) (err error) {
	if exists(dest) {
		return fmt.Errorf("Destination file path %s already exists", dest)
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		core.Debugf(`Failed to download from "%s". Code(%d) Message(%s)`, url, resp.StatusCode, strings.TrimPrefix(resp.Status, strconv.Itoa(resp.StatusCode)+" "))
		return httpError{resp.StatusCode}
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() {
		if err == nil {
			core.Debug("download complete")
			return
		}
		core.Debug("download failed")
		if rmErr := os.RemoveAll(dest); rmErr != nil {
			core.Debugf("Failed to delete '%s'. %s", dest, rmErr)
		}
	}()
	if _, err = io.Copy(out, resp.Body); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// DownloadTool Download a tool from an url and stream it into a file.
// Like @actions/tool-cache, an attempt that fails is retried (up to 3 attempts
// in total) unless the server answered with a client error other than
// 408 and 429.
func DownloadTool(url string, options *DownloadToolOptions) (string, error) {
	dest := destination(options)
	if err := ensureDestDir(dest); err != nil {
		return "", err
	}
	core.Debugf("Downloading %s", url)
	core.Debugf("Destination %s", dest)
	var err error
	for attempt := 1; attempt <= downloadMaxAttempts; attempt++ {
		if err = downloadAttempt(url, dest); err == nil {
			break
		}
		if !retryable(err) {
			return "", err
		}
		if attempt == downloadMaxAttempts {
			return "", err
		}
		core.Info(err.Error())
		seconds := retrySeconds()
		core.Infof("Waiting %d seconds before trying again", seconds)
		time.Sleep(time.Duration(seconds) * time.Second)
	}
	if options != nil && options.FileMode != 0 {
		if err := os.Chmod(dest, options.FileMode); err != nil {
			return "", err
		}
	}
	return dest, nil
}
