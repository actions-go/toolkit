package cache

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinuxVersion(t *testing.T) {
	assert.Equal(t, "22.04", linuxVersion("DISTRIB_ID=Ubuntu\nDISTRIB_RELEASE=22.04\nDISTRIB_CODENAME=jammy\n"))
	assert.Equal(t, "12", linuxVersion("NAME=\"Debian\"\nVERSION_ID=\"12\"\n"))
	assert.Equal(t, "", linuxVersion("NAME=x\nVERSION_ID=1=2\n"), "a line with several = is ignored")
	assert.Equal(t, "", linuxVersion(""))
}

func release(version string, stable bool, files ...ToolReleaseFile) ToolRelease {
	return ToolRelease{Version: version, Stable: stable, ReleaseURL: "https://example.invalid/" + version, Files: files}
}

func file(platform, arch string) ToolReleaseFile {
	return ToolReleaseFile{Filename: platform + "-" + arch, Platform: platform, Arch: arch, DownloadURL: "https://example.invalid/" + platform + "-" + arch}
}

func TestFindFromManifestKeepsOnlyTheMatchingFile(t *testing.T) {
	here := file(jsPlatform(), "x64")
	manifest := []ToolRelease{
		release("2.0.0", false, here),
		release("1.5.0", true, file("plan9", "x64")),
		release("1.4.0", true, file(jsPlatform(), "arm64"), here),
	}
	got := FindFromManifest("1.x", true, manifest, "x64")
	require.NotNil(t, got)
	assert.Equal(t, "1.4.0", got.Version)
	assert.Equal(t, []ToolReleaseFile{here}, got.Files)
	assert.Len(t, manifest[2].Files, 2, "the manifest is not modified")

	assert.Nil(t, FindFromManifest("3.x", true, manifest, "x64"))
	assert.Equal(t, "2.0.0", FindFromManifest("2.x", false, manifest, "x64").Version)
	assert.Nil(t, FindFromManifest("2.x", true, manifest, "x64"), "2.0.0 is not stable")
}

func TestFindFromManifestDefaultsToTheArchitectureOfTheMachine(t *testing.T) {
	manifest := []ToolRelease{release("1.0.0", true, file(jsPlatform(), jsArch()))}
	assert.NotNil(t, FindFromManifest("*", true, manifest, ""))
}

func TestGetManifestFromRepoUsesTheAPI(t *testing.T) {
	var treePath, treeAuth, blobAccept string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/git/trees/dev":
			treePath, treeAuth = r.URL.Path, r.Header.Get("Authorization")
			fmt.Fprintf(w, `{"tree":[{"path":"versions-manifest.json","url":"%s/blob"}]}`, srv.URL)
		case "/blob":
			blobAccept = r.Header.Get("Accept")
			fmt.Fprint(w, `[{"version":"1.0.0","stable":true,"release_url":"u","files":[]}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	old := ManifestAPIURL
	ManifestAPIURL = srv.URL
	defer func() { ManifestAPIURL = old }()

	releases, err := GetManifestFromRepo("o", "r", "token abc", "dev")
	require.NoError(t, err)
	assert.Equal(t, []ToolRelease{{Version: "1.0.0", Stable: true, ReleaseURL: "u", Files: []ToolReleaseFile{}}}, releases)
	assert.Equal(t, "/repos/o/r/git/trees/dev", treePath)
	assert.Equal(t, "token abc", treeAuth)
	assert.Equal(t, "application/vnd.github.VERSION.raw", blobAccept)

	releases, err = GetManifestFromRepo("o", "missing", "", "")
	require.NoError(t, err)
	assert.Empty(t, releases, "a 404 is an empty manifest")
}
