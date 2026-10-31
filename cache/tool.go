package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/actions-go/toolkit/core"
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

func jsArch() string {
	return nodeArch(runtime.GOARCH)
}

// nodeArch maps a GOARCH to the value of os.arch() in node, see
// https://nodejs.org/api/os.html#os_os_arch
func nodeArch(goarch string) string {
	switch goarch {
	case "386":
		return "ia32"
	case "amd64":
		return "x64"
	case "mipsle":
		return "mipsel"
	case "ppc64le":
		return "ppc64"
	default:
		return goarch
	}
}

func jsPlatform() string {
	return nodePlatform(runtime.GOOS)
}

// nodePlatform maps a GOOS to the value of os.platform() in node, see
// https://nodejs.org/api/os.html#os_os_platform
func nodePlatform(goos string) string {
	switch goos {
	case "windows":
		return "win32"
	case "solaris", "illumos":
		return "sunos"
	default:
		return goos
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
