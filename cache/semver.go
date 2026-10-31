package cache

import (
	"regexp"
	"strings"
)

// strictSemver is the full semantic version grammar node-semver accepts
// without its loose option (https://semver.org), once any leading "=" and "v"
// are removed.
var strictSemver = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
	`(-(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*)?` +
	`(?:\+[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*)?$`)

// maxSemverLength is the longest version string node-semver parses.
const maxSemverLength = 256

// explicitVersion reports whether version is a complete semantic version and
// returns its canonical form: surrounding blanks, leading "=" and "v" and the
// build metadata are dropped. This is semver.clean of node-semver.
func explicitVersion(version string) (string, bool) {
	s := strings.TrimLeft(strings.TrimSpace(version), "=v")
	if len(s) > maxSemverLength {
		return "", false
	}
	m := strictSemver.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	return m[1] + "." + m[2] + "." + m[3] + m[4], true
}

// cleanSemver returns the canonical form of an explicit version, or the
// version unchanged when it is not one (`semver.clean(version) || version`).
func cleanSemver(version string) string {
	if c, ok := explicitVersion(version); ok {
		return c
	}
	return version
}
