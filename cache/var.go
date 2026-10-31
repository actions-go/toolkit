package cache

import (
	"os"
	"path/filepath"
)

var (
	// tempDirectory and cacheRoot override RUNNER_TEMP and RUNNER_TOOL_CACHE
	// when non empty. They are empty in production: the environment is read at
	// each call, like @actions/tool-cache does.
	tempDirectory string
	cacheRoot     string
)

func tempDir() string {
	if tempDirectory != "" {
		return tempDirectory
	}
	return getenvOrDefault("RUNNER_TEMP", filepath.Join(baseLocation, "actions", "temp"))
}

func cacheDirectory() string {
	if cacheRoot != "" {
		return cacheRoot
	}
	return getenvOrDefault("RUNNER_TOOL_CACHE", filepath.Join(baseLocation, "actions", "cache"))
}

func getenvOrDefault(key, dflt string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return dflt
}
