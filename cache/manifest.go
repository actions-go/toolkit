package cache

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/actions-go/toolkit/core"
)

// ToolReleaseFile is a downloadable file of a tool release, an entry of the
// "files" of a versions-manifest.json.
type ToolReleaseFile struct {
	Filename        string `json:"filename"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version,omitempty"`
	Arch            string `json:"arch"`
	DownloadURL     string `json:"download_url"`
}

// ToolRelease is a release of a tool, an entry of a versions-manifest.json.
type ToolRelease struct {
	Version    string            `json:"version"`
	Stable     bool              `json:"stable"`
	ReleaseURL string            `json:"release_url"`
	Files      []ToolReleaseFile `json:"files"`
}

// ManifestAPIURL is the GitHub API GetManifestFromRepo reads the manifest
// through. It is a variable for tests, which point it at a local server.
var ManifestAPIURL = "https://api.github.com"

// manifestFile is the name of the manifest in the repository.
const manifestFile = "versions-manifest.json"

// GetManifestFromRepo reads the versions-manifest.json at the root of the
// branch (master when empty) of the repository owner/repo, like
// getManifestFromRepo of @actions/tool-cache. auth is the value of the
// Authorization header, when not empty.
//
// A repository or branch that does not exist, an empty manifest and a manifest
// that is not JSON give an empty list, a manifest that is JSON null a nil list,
// any other failure of the API an error. A manifest that is JSON but not an
// array is returned as is by the reference; here it is not valid and gives an
// empty list.
func GetManifestFromRepo(owner, repo, auth, branch string) ([]ToolRelease, error) {
	if branch == "" {
		branch = "master"
	}
	releases := []ToolRelease{}
	treeURL := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s", strings.TrimSuffix(ManifestAPIURL, "/"), owner, repo, branch)
	headers := http.Header{}
	headers.Set("User-Agent", "tool-cache")
	if auth != "" {
		core.Debug("set auth")
		headers.Set("Authorization", auth)
	}
	headers.Set("Accept", "application/json")
	status, body, err := manifestGet(treeURL, headers)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return releases, nil
	}
	if status > 299 {
		return nil, apiError(status, body)
	}
	// a body that is not JSON, or null, is no result
	var result interface{}
	if len(body) == 0 || json.Unmarshal(body, &result) != nil || result == nil {
		return releases, nil
	}
	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			URL  string `json:"url"`
		} `json:"tree"`
	}
	if err := json.Unmarshal(body, &tree); err != nil || tree.Tree == nil {
		// the message of V8 for the reference
		return nil, errors.New("response.result.tree is not iterable")
	}
	manifestURL := ""
	for _, item := range tree.Tree {
		if item.Path == manifestFile {
			manifestURL = item.URL
			break
		}
	}
	if manifestURL == "" {
		return nil, errors.New("Invalid URL")
	}
	headers.Set("Accept", "application/vnd.github.VERSION.raw")
	_, raw, err := manifestGet(manifestURL, headers)
	if err != nil {
		return nil, err
	}
	if len(raw) > 0 {
		// protects against invalid json saved with a BOM
		raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
		if string(bytes.TrimSpace(raw)) == "null" {
			return nil, nil
		}
		var parsed []ToolRelease
		if err := json.Unmarshal(raw, &parsed); err != nil || parsed == nil {
			core.Debug("Invalid json")
			return releases, nil
		}
		releases = parsed
	}
	return releases, nil
}

// manifestGet reads a URL and returns the status and the body, whatever the
// status is.
func manifestGet(url string, headers http.Header) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, errors.New("Invalid URL")
	}
	req.Header = headers.Clone()
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, body, err
}

// apiError is the error @actions/http-client reports for a failed request: the
// message of the body when it is JSON with one, the body, or the status.
func apiError(status int, body []byte) error {
	var obj struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &obj) == nil && obj.Message != "" {
		return errors.New(obj.Message)
	}
	if len(body) > 0 {
		return errors.New(string(body))
	}
	return fmt.Errorf("Failed request: (%d)", status)
}

// FindFromManifest returns the first release of manifest whose version
// satisfies versionSpec, that is stable when stable is true, and that has a
// file for archFilter (the architecture of the machine when empty) and the
// platform and, when the file has one, the platform version of the machine.
// The release is returned with only that file. It is nil when nothing matches.
// Like findFromManifest of @actions/tool-cache.
func FindFromManifest(versionSpec string, stable bool, manifest []ToolRelease, archFilter string) *ToolRelease {
	if archFilter == "" {
		archFilter = jsArch()
	}
	platform := jsPlatform()
	for _, candidate := range manifest {
		core.Debugf("check %s satisfies %s", candidate.Version, versionSpec)
		if !satisfies(candidate.Version, versionSpec) || (stable && !candidate.Stable) {
			continue
		}
		for _, file := range candidate.Files {
			core.Debugf("%s===%s && %s===%s", file.Arch, archFilter, file.Platform, platform)
			if file.Arch != archFilter || file.Platform != platform {
				continue
			}
			if file.PlatformVersion != "" {
				osVersion := OSVersion()
				if osVersion != file.PlatformVersion && !satisfies(strings.TrimSpace(osVersion), file.PlatformVersion) {
					continue
				}
			}
			core.Debugf("matched %s", candidate.Version)
			match := candidate
			match.Files = []ToolReleaseFile{file}
			return &match
		}
	}
	return nil
}

// OSVersion returns the version of the operating system the way manifests
// name it: the product version on macOS, the VERSION_ID (or DISTRIB_RELEASE)
// of /etc/os-release (or /etc/lsb-release) on Linux, and an empty string
// elsewhere or when it cannot be read. Like _getOsVersion of
// @actions/tool-cache, including the new line macOS ends its answer with.
func OSVersion() string {
	switch jsPlatform() {
	case "darwin":
		out, _ := exec.Command("sw_vers", "-productVersion").Output()
		return string(out)
	case "linux":
		return linuxVersion(readLinuxVersionFile())
	}
	return ""
}

// readLinuxVersionFile returns lsb-release, or os-release when there is none.
func readLinuxVersionFile() string {
	for _, name := range []string{"/etc/lsb-release", "/etc/os-release"} {
		if _, err := os.Stat(name); err == nil {
			data, _ := os.ReadFile(name)
			return string(data)
		}
	}
	return ""
}

func linuxVersion(contents string) string {
	for _, line := range strings.Split(contents, "\n") {
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			continue
		}
		if key := strings.TrimSpace(parts[0]); key == "VERSION_ID" || key == "DISTRIB_RELEASE" {
			value := strings.TrimSpace(parts[1])
			value = strings.TrimPrefix(value, `"`)
			return strings.TrimSuffix(value, `"`)
		}
	}
	return ""
}

// IsExplicitVersion reports whether versionSpec is a complete semantic
// version rather than a range, like isExplicitVersion of @actions/tool-cache.
func IsExplicitVersion(versionSpec string) bool {
	return isExplicitVersion(versionSpec)
}

// EvaluateVersions returns the highest version of versions that satisfies the
// range versionSpec, or an empty string, like evaluateVersions of
// @actions/tool-cache.
func EvaluateVersions(versions []string, versionSpec string) string {
	return evaluateVersions(versions, versionSpec)
}
