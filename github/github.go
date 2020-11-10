package github

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/actions-go/toolkit/core"
	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

func token() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	for _, input := range []string{"github-token", "token"} {
		if t, ok := core.GetInput(input); ok {
			return t
		}
	}
	return ""
}

func NewClient() *github.Client {
	token := token()
	httpClient := http.DefaultClient
	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		httpClient = oauth2.NewClient(context.Background(), ts)
	}
	return github.NewClient(httpClient)
}

var GitHub = NewClient()

func authorize(r *http.Request) {
	t := token()
	if t != "" {
		r.SetBasicAuth("", t)
	}
}

type Matcher func(path string) bool

type RepositoryFile struct {
	Path     string
	FileInfo os.FileInfo
	Data     []byte
}

// DownloadSelectedRepositoryFiles downloads files from a given repository and granch, given that their name matches regarding the `include` function
func DownloadSelectedRepositoryFiles(c *http.Client, owner, repo, branch string, include Matcher) map[string]RepositoryFile {
	u := fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball/%s", owner, repo, branch)
	core.Debugf("Downloading tarball for repo: %s", u)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		core.Warningf("failed to download repository: %v", err)
		return nil
	}
	authorize(req)
	resp, err := c.Do(req)
	if err != nil {
		core.Warningf("failed to download repository: %v", err)
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		core.Warningf("failed to download repository: unexpected code %d", resp.StatusCode)
		return nil
	}
	defer resp.Body.Close()
	var body io.Reader = resp.Body
	switch resp.Header.Get("Content-Type") {
	case "application/gzip", "application/x-gzip":
		body, err = gzip.NewReader(body)
		if err != nil {
			core.Warningf("failed to download repository: %v", err)
			return nil
		}
	}
	files := map[string]RepositoryFile{}
	tr := tar.NewReader(body)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			core.Warningf("failed to download repository: %v", err)
			return nil
		}
		if hdr.Format == tar.FormatPAX || hdr.FileInfo().IsDir() {
			continue
		}
		name := strings.SplitN(hdr.Name, string(os.PathSeparator), 2)[1]
		if include(name) {
			core.Debugf("Downloading %v", hdr.Name)
			b := bytes.NewBuffer(nil)
			if _, err := io.Copy(b, tr); err != nil {
				core.Warningf("failed to download repository: %v", err)
				return nil
			}
			files[name] = RepositoryFile{
				Path:     name,
				FileInfo: hdr.FileInfo(),
				Data:     b.Bytes(),
			}
		}
	}
	return files
}

// MatchesOneOf returns a matcher returning whether the path matches one of the provided glob patterns
func MatchesOneOf(patterns ...string) Matcher {
	return func(path string) bool {
		for _, p := range patterns {
			exp, err := regexp.CompilePOSIX(p)
			if err != nil {
				core.Warningf("unable to compile pattern %s: %v", p, err)
			}
			if exp.MatchString(path) {
				return true
			}
		}
		return false
	}
}
