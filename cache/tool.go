package cache

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/actions-go/toolkit/core"
	"github.com/google/uuid"
)

const (
	// cachePerms is the mode of created directories before the umask applies,
	// like fs.mkdir of node.
	cachePerms = 0o777
)

// DownloadToolOptions defines available options to download tools
type DownloadToolOptions struct {
	Destination string
	FileMode    os.FileMode
}

// CacheOptions defines the available options for tool and file caching
type CacheOptions struct {
	Tool    string
	Version string
	Arch    string
	// UseJavascriptValues instructs to use
	// the javascript os.arch() and os.platform() values instead of respectively
	// runtime.GOARCH and runtime.GOOS
	UseJavascriptValues *bool
}

func Bool(b bool) *bool {
	return &b
}

func destination(options *DownloadToolOptions) string {
	if options != nil && options.Destination != "" {
		return options.Destination
	}
	return filepath.Join(tempDir(), uuid.New().String())
}

func ensureDestDir(dest string) error {
	fmt.Println(dest)
	destDir := filepath.Dir(dest)
	if destDir == "" {
		return nil
	}
	if err := os.MkdirAll(destDir, cachePerms); err != nil {
		return fmt.Errorf("Unable to create destination directory %s: %v", destDir, err)
	}
	return nil
}

func ensureDestNotExists(dest string) error {
	_, err := os.Stat(dest)
	if err == nil {
		return fmt.Errorf("already exists")
	}
	if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func copyURL(dest io.Writer, source string) error {
	wrapError := func(err error, format string, args ...interface{}) error {
		return fmt.Errorf("failed to download "+source+" "+format+" : %v", append(args, err)...)
	}
	if dest == nil {
		return wrapError(fmt.Errorf("destination should not be null"), "")
	}
	resp, err := http.Get(source)
	if err != nil {
		return wrapError(err, "download failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return wrapError(fmt.Errorf("unexpected status code %d (%s). Expecting %d", resp.StatusCode, resp.Status, http.StatusOK), "")
	}
	_, err = io.Copy(dest, resp.Body)
	if err != nil {
		return wrapError(err, "failed to write to destination")
	}
	return nil
}

func jsArch() string {
	// mapping https://github.com/golang/go/blob/98d2717499575afe13d9f815d46fcd6e384efb0c/src/go/build/syslist.go#L11
	// to https://nodejs.org/api/os.html#os_os_arch
	switch runtime.GOARCH {
	case "386":
		return "x32"
	case "amd64":
		return "x64"
	default:
		return runtime.GOARCH
	}
}

func jsPlatform() string {
	// mapping https://github.com/golang/go/blob/98d2717499575afe13d9f815d46fcd6e384efb0c/src/go/build/syslist.go#L10
	// to https://nodejs.org/api/os.html#os_os_platform
	switch runtime.GOOS {
	case "windows":
		return "win32"
	default:
		return runtime.GOOS
	}
}

func toolPath(options CacheOptions) string {
	return filepath.Join(cacheDirectory(), options.Tool, options.Version, options.Arch)
}

func defaultOptions(options CacheOptions) (CacheOptions, error) {
	if options.Tool == "" {
		return options, fmt.Errorf("missing tool name to cache in options.Tool")
	}
	if options.Version == "" {
		return options, fmt.Errorf("missing tool version to cache in options.Version")
	}
	options.Version = cleanSemver(options.Version)
	return withDefaultArch(options), nil
}

// withDefaultArch sets the machine architecture when none is given,
// as node's os.arch() unless UseJavascriptValues is false.
func withDefaultArch(options CacheOptions) CacheOptions {
	if options.UseJavascriptValues == nil {
		options.UseJavascriptValues = Bool(true)
	}
	if options.Arch == "" {
		if *options.UseJavascriptValues {
			options.Arch = jsArch()
		} else {
			options.Arch = runtime.GOARCH
		}
	}
	return options
}

// DownloadTool Download a tool from an url and stream it into a file
func DownloadTool(url string, options *DownloadToolOptions) (string, error) {
	// TODO
	//   const http = new httpm.HttpClient(userAgent, [], {
	//     allowRetries: true,
	//     maxRetries: 3
	//   })
	wrapError := func(err error, format string, args ...interface{}) (string, error) {
		return "", fmt.Errorf(format+" : %v", append(args, err)...)
	}
	dest := destination(options)
	core.Debugf("Downloading %s", url)
	core.Debugf("Downloading %s", dest)
	if err := ensureDestDir(dest); err != nil {
		return wrapError(err, "Unable to create destination directory")
	}
	if err := ensureDestNotExists(dest); err != nil {
		return wrapError(err, "Destination file path %v", dest)
	}
	out, err := os.Create(dest)
	if err != nil {
		return wrapError(err, "failed to create destination file %s", dest)
	}
	if err := copyURL(out, url); err != nil {
		return wrapError(err, "failed to write file %s", dest)
	}
	if options != nil && options.FileMode != 0 {
		err = os.Chmod(dest, options.FileMode)
	}
	return dest, err
}

// GetCachedToolOrDownload returns the path of a cached tool or downloads and caches it
func GetCachedToolOrDownload(cache CacheOptions, download *DownloadToolOptions, url string) (string, error) {
	path, err := FindVersion(cache)
	if err != nil {
		path, err := DownloadTool(url, download)
		if err != nil {
			return "", err
		}
		_, err = CacheFile(path, cache.Tool, cache)
		if err != nil {
			core.Warningf("failed to cache downloaded file %v: %v", path, err)
		}
		return path, nil
	}
	return filepath.Join(path, cache.Tool), nil
}
