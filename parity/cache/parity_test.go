// Package cacheparity compares the cache package with the reference JavaScript
// implementation (@actions/tool-cache, run by ../js/toolcache-driver.mjs).
//
// Every scenario in scenarios/*.json is run through both implementations, each
// in its own temporary RUNNER_TOOL_CACHE and RUNNER_TEMP. The observable
// behaviour (stdout, return values, errors and the resulting directory tree
// with file contents and modes) must be identical unless a documented entry
// exists in known-divergences.json.
package cacheparity

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/actions-go/toolkit/cache"
	"github.com/actions-go/toolkit/core"
	"github.com/stretchr/testify/assert"
)

// entry is a fixture file, directory or symlink created before a scenario.
type entry struct {
	Path    string  `json:"path"`
	Content string  `json:"content,omitempty"`
	Mode    string  `json:"mode,omitempty"`
	Dir     bool    `json:"dir,omitempty"`
	Symlink *string `json:"symlink,omitempty"`
}

type call struct {
	Fn   string        `json:"fn"`
	Args []interface{} `json:"args"`
}

type scenario struct {
	Name  string            `json:"name"`
	Env   map[string]string `json:"env"`
	Src   []entry           `json:"src"`
	Cache []entry           `json:"cache"`
	Temp  []entry           `json:"temp"`
	Calls []call            `json:"calls"`
}

type ret struct {
	Value interface{} `json:"value,omitempty"`
	Error string      `json:"error,omitempty"`
}

type node struct {
	Kind    string `json:"kind"`
	Mode    string `json:"mode,omitempty"`
	Content string `json:"content,omitempty"`
	Target  string `json:"target,omitempty"`
}

type result struct {
	Root    string          `json:"root"`
	Stdout  string          `json:"stdout"`
	Returns []ret           `json:"returns"`
	Tree    map[string]node `json:"tree"`
}

// fileBody is what the test server answers on /file.
const fileBody = "hello world\n"

// newServer starts the HTTP server both implementations download from.
func newServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(fileBody)) })
	mux.HandleFunc("/dir/nested/file.txt", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("nested\n")) })
	mux.HandleFunc("/empty", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/binary", func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 256)
		for i := range b {
			b[i] = byte(i)
		}
		_, _ = w.Write(b)
	})
	// Location is absolute: the reference cannot follow a relative one
	redirect := func(to string, status int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://"+r.Host+to, status)
		}
	}
	mux.HandleFunc("/redirect", redirect("/file", http.StatusFound))
	mux.HandleFunc("/redirect-permanent", redirect("/redirect", http.StatusMovedPermanently))
	mux.HandleFunc("/redirect-307", redirect("/file", http.StatusTemporaryRedirect))
	mux.HandleFunc("/redirect-to-404", redirect("/404", http.StatusFound))
	mux.HandleFunc("/redirect-relative", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/file", http.StatusFound) })
	mux.HandleFunc("/redirect-loop", redirect("/redirect-loop", http.StatusFound))
	for _, code := range []int{204, 206, 400, 401, 403, 404, 408, 429, 500, 503} {
		code := code
		mux.HandleFunc("/"+strconv.Itoa(code), func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		})
	}
	return httptest.NewServer(mux)
}

func loadScenarios(t *testing.T) []scenario {
	t.Helper()
	paths, err := filepath.Glob("scenarios/*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no scenarios found: %v", err)
	}
	sort.Strings(paths)
	var all []scenario
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var s []scenario
		if err := json.Unmarshal(data, &s); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		all = append(all, s...)
	}
	seen := map[string]bool{}
	for _, s := range all {
		if seen[s.Name] {
			t.Fatalf("duplicate scenario name %q", s.Name)
		}
		seen[s.Name] = true
	}
	return all
}

// materialise creates the fixture entries below base.
func materialise(t *testing.T, base string, entries []entry) {
	t.Helper()
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		p := filepath.Join(base, e.Path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		switch {
		case e.Symlink != nil:
			if err := os.Symlink(*e.Symlink, p); err != nil {
				t.Fatal(err)
			}
			continue
		case e.Dir:
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
		default:
			if err := os.WriteFile(p, []byte(e.Content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if e.Mode != "" {
			m, err := strconv.ParseUint(e.Mode, 8, 32)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(p, fs.FileMode(m)); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// snapshot describes every entry below root with root-relative paths.
func snapshot(t *testing.T, root string) map[string]node {
	t.Helper()
	tree := map[string]node{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		mode := fmt.Sprintf("0%o", info.Mode().Perm())
		if runtime.GOOS == "windows" {
			// Windows has no Unix permission bits: Go and node report different
			// placeholders (0666 vs 0777), so modes are not compared there
			mode = ""
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			tree[rel] = node{Kind: "symlink", Target: target}
		case info.IsDir():
			tree[rel] = node{Kind: "dir", Mode: mode}
		default:
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			content := string(data)
			if !utf8.Valid(data) {
				content = "base64:" + base64.StdEncoding.EncodeToString(data)
			}
			tree[rel] = node{Kind: "file", Mode: mode, Content: content}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

type dirs struct{ root, src, cache, temp, server string }

func (d dirs) subst(v interface{}) interface{} {
	s, ok := v.(string)
	if !ok {
		return v
	}
	return strings.NewReplacer("{{src}}", d.src, "{{cache}}", d.cache, "{{temp}}", d.temp, "{{server}}", d.server).Replace(s)
}

// invoke maps a JS function name to the equivalent Go call. Optional JS
// arguments that are absent or null are the empty string.
func invoke(t *testing.T, d dirs, c call) ret {
	a := func(i int) string {
		if i >= len(c.Args) || c.Args[i] == nil {
			return ""
		}
		return fmt.Sprint(d.subst(c.Args[i]))
	}
	switch c.Fn {
	case "cacheDir":
		return result1(cache.CacheDir(a(0), cache.CacheOptions{Tool: a(1), Version: a(2), Arch: a(3)}))
	case "cacheFile":
		return result1(cache.CacheFile(a(0), a(1), cache.CacheOptions{Tool: a(2), Version: a(3), Arch: a(4)}))
	case "find":
		p, err := cache.FindVersion(cache.CacheOptions{Tool: a(0), Version: a(1), Arch: a(2)})
		// JS returns "" when nothing matches; Go returns an error for it.
		if err != nil && strings.HasPrefix(err.Error(), "could not find any cached version") {
			return ret{Value: ""}
		}
		return result1(p, err)
	case "findAllVersions":
		v := cache.ListAllCachedVersions(cache.CacheOptions{Tool: a(0), Arch: a(1)})
		if v == nil {
			v = []string{}
		}
		// the order of directory entries is not part of the contract
		sort.Strings(v)
		return ret{Value: v}
	case "downloadTool":
		var opts *cache.DownloadToolOptions
		if a(1) != "" {
			opts = &cache.DownloadToolOptions{Destination: a(1)}
		}
		return result1(cache.DownloadTool(a(0), opts))
	default:
		t.Fatalf("no Go adapter for %q", c.Fn)
	}
	return ret{}
}

// result1 reports a thrown error like the JS driver: an error and no value.
func result1(value string, err error) ret {
	if err != nil {
		return ret{Error: err.Error()}
	}
	return ret{Value: value}
}

var cleanedEnv = regexp.MustCompile(`^(RUNNER_|ACTIONS_|GITHUB_|INPUT_)`)

func runGo(t *testing.T, sc scenario, server string) result {
	t.Helper()
	for _, kv := range os.Environ() {
		k := strings.SplitN(kv, "=", 2)[0]
		if cleanedEnv.MatchString(k) {
			t.Setenv(k, "")
			os.Unsetenv(k)
		}
	}
	root := t.TempDir()
	d := dirs{root: root, src: filepath.Join(root, "src"), cache: filepath.Join(root, "cache"), temp: filepath.Join(root, "temp"), server: server}
	materialise(t, d.src, sc.Src)
	materialise(t, d.cache, sc.Cache)
	materialise(t, d.temp, sc.Temp)
	t.Setenv("RUNNER_TOOL_CACHE", d.cache)
	t.Setenv("RUNNER_TEMP", d.temp)
	for k, v := range sc.Env {
		t.Setenv(k, v)
	}
	// same switch as the JS global: the reference would sleep 10-20s between attempts
	t.Setenv("TEST_DOWNLOAD_TOOL_RETRY_MIN_SECONDS", "0")
	t.Setenv("TEST_DOWNLOAD_TOOL_RETRY_MAX_SECONDS", "0")

	out, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	realStdout := os.Stdout
	os.Stdout = out
	core.SetStdout(out)
	var returns []ret
	func() {
		defer func() {
			os.Stdout = realStdout
			core.SetStdout(realStdout)
			out.Close()
		}()
		for _, c := range sc.Calls {
			returns = append(returns, invoke(t, d, c))
		}
	}()
	stdout, _ := os.ReadFile(out.Name())
	return result{Root: root, Stdout: string(stdout), Returns: returns, Tree: snapshot(t, root)}
}

func runJS(t *testing.T, scenariosFile, server string) map[string]result {
	t.Helper()
	// CI sets PARITY_REQUIRED so that a missing node never turns into a silent pass
	unavailable := t.Skip
	if os.Getenv("PARITY_REQUIRED") != "" {
		unavailable = t.Fatal
	}
	if _, err := exec.LookPath("node"); err != nil {
		unavailable("node is not installed")
	}
	if _, err := os.Stat("../js/node_modules/@actions/tool-cache"); err != nil {
		unavailable("run `npm ci` in parity/js first (and make sure @actions/tool-cache is installed)")
	}
	cmd := exec.Command("node", "toolcache-driver.mjs", scenariosFile, server)
	cmd.Dir = "../js"
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("js driver failed: %v\nstderr: %s\nstdout: %.500s", err, stderr.String(), stdout.String())
	}
	var res map[string]result
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("js driver output: %v\n%s", err, stdout.String())
	}
	return res
}

// Temporary locations and the random file names of default downloads are the
// only values that legitimately differ between runs and implementations.
var uuidName = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func (r result) normalise(server string) result {
	n := strings.NewReplacer(
		filepath.Join(r.Root, "cache"), "<CACHE>",
		filepath.Join(r.Root, "src"), "<SRC>",
		filepath.Join(r.Root, "temp"), "<TEMP>",
		r.Root, "<ROOT>",
		server, "<SERVER>",
	).Replace
	norm := func(s string) string { return uuidName.ReplaceAllString(n(s), "<UUID>") }

	r.Stdout = norm(r.Stdout)
	r.Root = ""
	returns := make([]ret, len(r.Returns))
	for i, v := range r.Returns {
		returns[i] = ret{Error: norm(v.Error)}
		switch val := v.Value.(type) {
		case string:
			returns[i].Value = norm(val)
		default:
			returns[i].Value = val
		}
	}
	r.Returns = returns
	tree := map[string]node{}
	for k, v := range r.Tree {
		v.Target = n(v.Target)
		tree[norm(k)] = v
	}
	r.Tree = tree
	return r
}

type divergence struct {
	Scenario string `json:"scenario"`
	Reason   string `json:"reason"`
}

func knownDivergences(t *testing.T) map[string]string {
	data, err := os.ReadFile("known-divergences.json")
	if err != nil {
		t.Fatal(err)
	}
	var d []divergence
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	for _, e := range d {
		if strings.TrimSpace(e.Reason) == "" {
			t.Fatalf("divergence %q has no reason", e.Scenario)
		}
		m[e.Scenario] = e.Reason
	}
	return m
}

// umaskModes returns the modes a new directory and a new file get, so the
// hand-worked answer holds whatever the umask is.
func umaskModes(t *testing.T) (dir, file string) {
	t.Helper()
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "d"), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "f"), nil, 0o666); err != nil {
		t.Fatal(err)
	}
	d, _ := os.Stat(filepath.Join(base, "d"))
	f, _ := os.Stat(filepath.Join(base, "f"))
	return fmt.Sprintf("0%o", d.Mode().Perm()), fmt.Sprintf("0%o", f.Mode().Perm())
}

// TestHarnessMatchesHandWorkedAnswer checks the harness itself: the answer for
// caching one file is worked out by hand from the documented behaviour of
// cacheFile (copy to <cache>/<tool>/<version>/<arch>/<target>, then write the
// empty <arch>.complete marker), and both implementations must produce it.
func TestHarnessMatchesHandWorkedAnswer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the hand-worked answer contains Unix permission bits")
	}
	sc := scenario{
		Name:  "hand worked",
		Src:   []entry{{Path: "a.txt", Content: "x"}},
		Calls: []call{{Fn: "cacheFile", Args: []interface{}{"{{src}}/a.txt", "t.txt", "tool", "v1.2.3", "x64"}}},
	}
	d, f := umaskModes(t)
	want := result{
		Stdout:  "::debug::Caching tool tool 1.2.3 x64\n::debug::source file: <SRC>/a.txt\n::debug::destination <CACHE>/tool/1.2.3/x64\n::debug::destination file <CACHE>/tool/1.2.3/x64/t.txt\n::debug::finished caching tool\n",
		Returns: []ret{{Value: "<CACHE>/tool/1.2.3/x64"}},
		Tree: map[string]node{
			"cache":                         {Kind: "dir", Mode: d},
			"cache/tool":                    {Kind: "dir", Mode: d},
			"cache/tool/1.2.3":              {Kind: "dir", Mode: d},
			"cache/tool/1.2.3/x64":          {Kind: "dir", Mode: d},
			"cache/tool/1.2.3/x64.complete": {Kind: "file", Mode: f},
			"cache/tool/1.2.3/x64/t.txt":    {Kind: "file", Mode: f, Content: "x"},
			"src":                           {Kind: "dir", Mode: d},
			"src/a.txt":                     {Kind: "file", Mode: f, Content: "x"},
			"temp":                          {Kind: "dir", Mode: d},
		},
	}
	wantJSON, _ := json.MarshalIndent(want, "", "  ")
	srv := newServer()
	defer srv.Close()

	file := filepath.Join(t.TempDir(), "scenarios.json")
	data, _ := json.Marshal([]scenario{sc})
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	js := runJS(t, file, srv.URL)[sc.Name]
	jsJSON, _ := json.MarshalIndent(js.normalise(srv.URL), "", "  ")
	assert.Equal(t, string(wantJSON), string(jsJSON), "reference driver")

	goJSON, _ := json.MarshalIndent(runGo(t, sc, srv.URL).normalise(srv.URL), "", "  ")
	assert.Equal(t, string(wantJSON), string(goJSON), "Go")
}

func TestParityWithReferenceToolCache(t *testing.T) {
	scenarios := loadScenarios(t)
	merged, _ := json.Marshal(scenarios)
	mergedFile := filepath.Join(t.TempDir(), "scenarios.json")
	if err := os.WriteFile(mergedFile, merged, 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newServer()
	defer srv.Close()

	js := runJS(t, mergedFile, srv.URL)
	known := knownDivergences(t)
	seen := map[string]bool{}
	for _, sc := range scenarios {
		sc := sc
		t.Run(sc.Name, func(t *testing.T) {
			want, ok := js[sc.Name]
			if !ok {
				t.Fatalf("js driver returned nothing for %s", sc.Name)
			}
			got := runGo(t, sc, srv.URL)
			g, _ := json.MarshalIndent(got.normalise(srv.URL), "", "  ")
			w, _ := json.MarshalIndent(want.normalise(srv.URL), "", "  ")
			same := bytes.Equal(g, w)
			if reason, isKnown := known[sc.Name]; isKnown {
				seen[sc.Name] = true
				if same {
					t.Errorf("scenario is listed in known-divergences.json (%s) but now matches: remove the entry", reason)
				}
				t.Skipf("known divergence: %s", reason)
			}
			if !same {
				// expected is the reference, actual is Go
				assert.Equal(t, string(w), string(g), "Go differs from @actions/tool-cache")
			}
		})
	}
	for name := range known {
		if !seen[name] {
			t.Errorf("known-divergences.json lists unknown scenario %q", name)
		}
	}
	fmt.Fprintf(os.Stderr, "cache parity: %d scenarios, %d known divergences\n", len(scenarios), len(known))
}
