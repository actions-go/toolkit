package cache

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/uuid"
	"github.com/tjamet/go-github-action-toolkit/core"
)

const (
	cachePerms = 0755
)

// DownloadToolOptions defines available options to download tools
type DownloadToolOptions struct {
	Destination string
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
	return filepath.Join(tempDirectory, uuid.New().String())
}

func ensureDestDir(dest string) error {
	destDir := filepath.Dir(dest)
	if destDir == "" {
		return nil
	}
	if err := os.MkdirAll(destDir, cachePerms); err != nil {
		return fmt.Errorf("Unable to create destination directory %s: %v", destDir, err)
	}
	return nil
}

func createEmptyCache(folder string) error {
	if err := os.RemoveAll(folder); err != nil {
		return err
	}
	return os.MkdirAll(folder, cachePerms)
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
	return filepath.Join(cacheRoot, options.Tool, options.Version, options.Arch)
}

func cleanSemver(version string) string {
	return strings.TrimPrefix(version, "=v")
}

func defaultOptions(options CacheOptions) (CacheOptions, error) {
	if options.Tool == "" {
		return options, fmt.Errorf("missing tool name to cache in options.Tool")
	}
	if options.Version == "" {
		return options, fmt.Errorf("missing tool name to cache in options.Version")
	}
	options.Version = cleanSemver(options.Version)
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
	return options, nil
}

func noRel(path string) string {
	p := strings.Split(path, string(filepath.Separator))
	// In case the source is provided
	for len(p) > 0 && (p[0] == ".." || p[0] == ".") {
		p = p[1:]
	}
	return filepath.Join(p...)
}

func cache(source, target string, options CacheOptions) (string, error) {
	destFolder := toolPath(options)
	completeMarker := destFolder + ".complete"
	wrapError := func(err error, format string, args ...interface{}) (string, error) {
		return "", fmt.Errorf("failed to save "+source+" to cache "+format+" : %v", append(args, err)...)
	}
	options, err := defaultOptions(options)
	if err != nil {
		return wrapError(err, "invalid options")
	}
	core.Debugf(`destination file %s`, destFolder)
	err = createEmptyCache(destFolder)
	if err != nil {
		return wrapError(err, "")
	}
	err = os.Remove(completeMarker)
	if err != nil && !os.IsNotExist(err) {
		return wrapError(err, "")
	}
	// Ensure provided arguments are namespaced to the destFolder
	target = noRel(target)
	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		destination := filepath.Join(destFolder, target)
		r, err := filepath.Rel(source, path)
		if err == nil && r != "." {
			destination = filepath.Join(destination, r)
		}
		core.Debugf("copying %s to %s", path, destination)
		return copyFile(destination, path)
	})
	if err != nil {
		return wrapError(err, "failed to copy all files")
	}
	fd, err := os.Create(completeMarker)
	if err != nil {
		return wrapError(err, "failed to mark copy complete")
	}
	fd.Close()
	return destFolder, nil
}

func copyFile(dest, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(dest), cachePerms)
	if err != nil {
		return err
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	return err
}

// CacheFile caches a downloaded file (GUID) and installs it
// into the tool cache with a given targetName
func CacheFile(source, target string, options CacheOptions) (string, error) {
	return cache(source, target, options)
}

// CacheDir caches a directory and installs it into the tool cacheDir
// with a given targetName
func CacheDir(source string, options CacheOptions) (string, error) {
	return cache(source, "", options)
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
	return dest, nil
}
