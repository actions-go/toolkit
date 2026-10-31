package github

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/actions-go/toolkit/core"
	"github.com/google/go-github/v74/github"
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

// isJSSpace reports whether r is trimmed by JavaScript's String.prototype.trim.
// It differs from unicode.IsSpace on U+0085 (not trimmed) and U+FEFF (trimmed).
func isJSSpace(r rune) bool {
	return (unicode.IsSpace(r) && r != 0x85) || r == 0xFEFF
}

// sanitizeOrchestrationID replaces every character outside [a-zA-Z0-9_.-] by
// an underscore. Like JavaScript, it counts UTF-16 code units, so a character
// outside the basic multilingual plane becomes two underscores.
func sanitizeOrchestrationID(id string) string {
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '.', r == '-':
			b.WriteRune(r)
		case r >= 0x10000:
			b.WriteString("__")
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// UserAgentWithOrchestrationID appends the actions_orchestration_id tag built from
// ACTIONS_ORCHESTRATION_ID to baseUserAgent. The tag is not added twice.
func UserAgentWithOrchestrationID(baseUserAgent string) string {
	id := strings.TrimFunc(os.Getenv("ACTIONS_ORCHESTRATION_ID"), isJSSpace)
	if id == "" {
		return baseUserAgent
	}
	tag := "actions_orchestration_id/" + sanitizeOrchestrationID(id)
	if strings.Contains(baseUserAgent, tag) {
		return baseUserAgent
	}
	if baseUserAgent != "" {
		baseUserAgent += " "
	}
	return baseUserAgent + tag
}

func NewClient() *github.Client {
	client := newClient()
	client.UserAgent = UserAgentWithOrchestrationID(client.UserAgent)
	return client
}

func newClient() *github.Client {
	token := token()
	httpClient := http.DefaultClient
	if token != "" {
		// like @actions/github, authenticate with `Authorization: token <token>`
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token, TokenType: "token"})
		httpClient = oauth2.NewClient(context.Background(), ts)
	}
	client := github.NewClient(httpClient)
	if server, ok := os.LookupEnv("GITHUB_SERVER_URL"); ok && server != "https://github.com" {
		var err error
		client, err = github.NewEnterpriseClient(server, server, httpClient)
		if err != nil {
			core.Errorf("failed to initialise GitHub client: %v", err)
			return github.NewClient(httpClient)
		}
	}
	// like @actions/github, GITHUB_API_URL is the API base URL
	if api := os.Getenv("GITHUB_API_URL"); api != "" {
		u, err := url.Parse(strings.TrimSuffix(api, "/") + "/")
		if err != nil {
			core.Errorf("failed to parse GITHUB_API_URL: %v", err)
		} else {
			client.BaseURL = u
		}
	}
	return client
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
		splittedName := strings.SplitN(hdr.Name, "/", 2)
		if len(splittedName) > 1 {
			name := splittedName[1]
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
