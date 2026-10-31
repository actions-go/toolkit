package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/actions-go/toolkit/core"
)

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isExplicitVersion reports whether versionSpec is a complete version rather
// than a range, like isExplicitVersion of @actions/tool-cache.
func isExplicitVersion(versionSpec string) bool {
	c, ok := explicitVersion(versionSpec)
	core.Debugf("isExplicit: %s", c)
	core.Debugf("explicit? %t", ok)
	return ok
}

// satisfies reports whether version is in the range versionSpec. A version or
// a range that does not parse never satisfies.
func satisfies(version, versionSpec string) bool {
	// node-semver parses a version with an optional "v" and nothing else
	if !strictSemver.MatchString(strings.TrimPrefix(version, "v")) {
		return false
	}
	v, err := semver.NewVersion(version)
	if err != nil {
		return false
	}
	versionSpec = strings.TrimSpace(versionSpec)
	if versionSpec == "" {
		versionSpec = "*"
	}
	// Masterminds accepts "," and "!=" which are not node-semver range syntax
	if strings.ContainsAny(versionSpec, ",!") {
		return false
	}
	c, err := semver.NewConstraint(versionSpec)
	if err != nil {
		return false
	}
	return c.Check(v)
}

// evaluateVersions returns the highest version of versions that satisfies
// versionSpec, or "".
func evaluateVersions(versions []string, versionSpec string) string {
	core.Debugf("evaluating %d versions", len(versions))
	sorted := append([]string(nil), versions...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, errA := semver.NewVersion(sorted[i])
		b, errB := semver.NewVersion(sorted[j])
		return errA == nil && errB == nil && a.LessThan(b)
	})
	for i := len(sorted) - 1; i >= 0; i-- {
		if satisfies(sorted[i], versionSpec) {
			core.Debugf("matched: %s", sorted[i])
			return sorted[i]
		}
	}
	core.Debug("match not found")
	return ""
}

// ListAllCachedVersions discovers all versions available in cache
// for options.Tool and options.Arch (the machine architecture by default).
func ListAllCachedVersions(options CacheOptions) []string {
	if options.Tool == "" {
		core.Errorf("missing tool name to list versions")
		return []string{}
	}
	options = withDefaultArch(options)
	versions := []string{}
	toolDir := filepath.Join(cacheDirectory(), options.Tool)
	children, err := os.ReadDir(toolDir)
	if err != nil {
		return versions
	}
	for _, child := range children {
		if !isExplicitVersion(child.Name()) {
			continue
		}
		full := filepath.Join(toolDir, child.Name(), options.Arch)
		if exists(full) && exists(full+".complete") {
			versions = append(versions, child.Name())
		}
	}
	return versions
}

// FindVersion finds the path to a tool version in the local installed tool cache
// by matching the pattern provided in the Version field
func FindVersion(options CacheOptions) (string, error) {
	if options.Tool == "" {
		return "", fmt.Errorf("toolName parameter is required")
	}
	if options.Version == "" {
		return "", fmt.Errorf("versionSpec parameter is required")
	}
	options = withDefaultArch(options)
	spec := options.Version
	// attempt to resolve an explicit version
	if !isExplicitVersion(spec) {
		spec = evaluateVersions(ListAllCachedVersions(options), spec)
	}
	if spec != "" {
		options.Version, _ = explicitVersion(spec)
		path := toolPath(options)
		core.Debugf("checking cache: %s", path)
		if exists(path) && exists(path+".complete") {
			core.Debugf("Found tool in cache %s %s %s", options.Tool, options.Version, options.Arch)
			return path, nil
		}
		core.Debug("not found")
	}
	return "", fmt.Errorf("could not find any cached version for %s matching %s", options.Tool, options.Version)
}
