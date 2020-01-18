package cache

import (
	"os"
	"path/filepath"
)

var (
	tempDirectory = getenvOrDefault("RUNNER_TEMP", filepath.Join(baseLocation, "actions", "temp"))
	cacheRoot     = getenvOrDefault("RUNNER_TOOL_CACHE", filepath.Join(baseLocation, "actions", "cache"))
)

func getenvOrDefault(key, dflt string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return dflt
}
