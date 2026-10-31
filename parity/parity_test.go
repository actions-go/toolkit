// Package parity compares this module with the reference JavaScript
// implementation (@actions/core, pinned in js/package.json).
//
// Every scenario in scenarios/*.json is run through both implementations. The
// observable behaviour (stdout, file commands, return values, errors, selected
// environment variables, requests received by a local server) must be identical
// unless a documented entry exists in known-divergences.json.
//
// The scenario format is documented at the top of js/driver.mjs.
package parity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/actions-go/toolkit/core"
)

type call struct {
	Fn   string            `json:"fn"`
	Args []json.RawMessage `json:"args"`
}

type scenario struct {
	Name       string            `json:"name"`
	Env        map[string]string `json:"env"`
	InheritEnv []string          `json:"inheritEnv"`
	NoFiles    bool              `json:"noFiles"`
	ObserveEnv []string          `json:"observeEnv"`
	Server     *serverSpec       `json:"server"`
	Calls      []call            `json:"calls"`
}

type ret struct {
	Value interface{} `json:"value,omitempty"`
	Error string      `json:"error,omitempty"`
}

type result struct {
	Stdout   string             `json:"stdout"`
	Returns  []ret              `json:"returns"`
	Files    map[string]string  `json:"files"`
	Env      map[string]*string `json:"env"`
	Requests []request          `json:"requests,omitempty"`
}

var fileEnvs = map[string]string{
	"output":  "GITHUB_OUTPUT",
	"env":     "GITHUB_ENV",
	"path":    "GITHUB_PATH",
	"state":   "GITHUB_STATE",
	"summary": "GITHUB_STEP_SUMMARY",
}

var cleanedEnv = regexp.MustCompile(`^(INPUT_|STATE_|GITHUB_|RUNNER_|ACTIONS_)`)

func loadScenarios(t *testing.T) []scenario {
	t.Helper()
	paths, err := filepath.Glob("scenarios/*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no scenarios found: %v", err)
	}
	sort.Strings(paths)
	var all []scenario
	seen := map[string]bool{}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var s []scenario
		if err := json.Unmarshal(data, &s); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for _, sc := range s {
			if seen[sc.Name] {
				t.Fatalf("%s: duplicate scenario name %q", p, sc.Name)
			}
			seen[sc.Name] = true
		}
		all = append(all, s...)
	}
	return all
}

// str converts a JSON argument the way the JS API stringifies it
// (toCommandValue): strings as is, null as empty, anything else as compact JSON.
func str(raw json.RawMessage) string {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	}
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return string(raw)
	}
	return b.String()
}

// props maps the JS annotation properties object to the Go struct.
func props(t *testing.T, c call) []core.AnnotationProperties {
	if len(c.Args) < 2 {
		return nil
	}
	var p struct {
		Title       string `json:"title"`
		File        string `json:"file"`
		StartLine   int    `json:"startLine"`
		EndLine     int    `json:"endLine"`
		StartColumn int    `json:"startColumn"`
		EndColumn   int    `json:"endColumn"`
	}
	if err := json.Unmarshal(c.Args[1], &p); err != nil {
		t.Fatal(err)
	}
	return []core.AnnotationProperties{{Title: p.Title, File: p.File, StartLine: p.StartLine, EndLine: p.EndLine, StartColumn: p.StartColumn, EndColumn: p.EndColumn}}
}

// invoke maps a JS function name to the equivalent Go call.
func invoke(t *testing.T, c call) ret {
	a := func(i int) string { return str(c.Args[i]) }
	switch {
	case strings.HasPrefix(c.Fn, "summary."):
		return invokeSummary(t, strings.TrimPrefix(c.Fn, "summary."), c.Args)
	case strings.HasPrefix(c.Fn, "platform."):
		return invokePlatform(t, strings.TrimPrefix(c.Fn, "platform."))
	}
	switch c.Fn {
	case "exportVariable":
		core.ExportVariable(a(0), a(1))
	case "setSecret":
		core.SetSecret(a(0))
	case "addPath":
		core.AddPath(a(0))
	case "getInput":
		v, _ := core.GetInput(a(0))
		return ret{Value: v}
	case "getMultilineInput":
		return ret{Value: core.GetMultilineInput(a(0))}
	case "getBooleanInput":
		return ret{Value: core.GetBoolInput(a(0))}
	case "setOutput":
		core.SetOutput(a(0), a(1))
	case "setCommandEcho":
		core.SetCommandEcho(string(c.Args[0]) == "true")
	case "setFailed":
		core.SetFailed(a(0))
	case "isDebug":
		return ret{Value: core.IsDebug()}
	case "debug":
		core.Debug(a(0))
	case "error":
		core.Error(a(0), props(t, c)...)
	case "warning":
		core.Warning(a(0), props(t, c)...)
	case "notice":
		core.Notice(a(0), props(t, c)...)
	case "info":
		core.Info(a(0))
	case "startGroup":
		core.StartGroup(a(0))
	case "endGroup":
		core.EndGroup()
	case "group":
		return invokeGroup(t, c)
	case "stopCommands":
		core.StopCommands(a(0))
	case "startCommands":
		core.StartCommands(a(0))
	case "saveState":
		core.SaveState(a(0), a(1))
	case "getState":
		return ret{Value: core.GetState(a(0))}
	case "getIDToken":
		audience := ""
		if len(c.Args) > 0 {
			audience = a(0)
		}
		token, err := core.GetIDToken(audience)
		if err != nil {
			return ret{Error: err.Error()}
		}
		return ret{Value: token}
	case "toPosixPath":
		return ret{Value: core.ToPosixPath(a(0))}
	case "toWin32Path":
		return ret{Value: core.ToWin32Path(a(0))}
	case "toPlatformPath":
		return ret{Value: core.ToPlatformPath(a(0))}
	default:
		t.Fatalf("no Go adapter for %q", c.Fn)
	}
	return ret{}
}

// callbackSpec is a function argument: the nested calls it runs, and optionally
// an error it throws after running them.
type callbackSpec struct {
	Calls []call `json:"$calls"`
	Throw string `json:"throw"`
}

func invokeGroup(t *testing.T, c call) (r ret) {
	var cb callbackSpec
	if err := json.Unmarshal(c.Args[1], &cb); err != nil {
		t.Fatal(err)
	}
	// like the JS promise rejection, a throwing callback surfaces as the call's error
	defer func() {
		if p := recover(); p != nil {
			r = ret{Error: fmt.Sprint(p)}
		}
	}()
	core.Group(str(c.Args[0]), func() {
		for _, n := range cb.Calls {
			invoke(t, n)
		}
		if cb.Throw != "" {
			panic(cb.Throw)
		}
	})
	return ret{}
}

func runGo(t *testing.T, sc scenario) result {
	t.Helper()
	inherited := map[string]string{}
	for _, k := range sc.InheritEnv {
		if v, ok := os.LookupEnv(k); ok {
			inherited[k] = v
		}
	}
	for _, kv := range os.Environ() {
		k := strings.SplitN(kv, "=", 2)[0]
		if cleanedEnv.MatchString(k) || k == "PATH" {
			t.Setenv(k, "")
			os.Unsetenv(k)
		}
	}
	for k, v := range inherited {
		t.Setenv(k, v)
	}
	// the summary builder caches the file path and buffer for the whole process
	core.JobSummary = &core.Summary{}
	dir := t.TempDir()
	files := map[string]string{}
	if !sc.NoFiles {
		for key, envName := range fileEnvs {
			files[key] = filepath.Join(dir, key)
			if err := os.WriteFile(files[key], nil, 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv(envName, files[key])
		}
	}
	var srv *testServer
	serverURL := ""
	if sc.Server != nil {
		srv = startServer(t, sc.Server)
		serverURL = srv.URL
	}
	for k, v := range sc.Env {
		t.Setenv(k, strings.ReplaceAll(v, "{{server}}", serverURL))
	}

	// capture both the injectable writer and the process stdout (core.Info prints directly)
	out, err := os.Create(filepath.Join(dir, "stdout"))
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
			returns = append(returns, invoke(t, c))
		}
	}()
	stdout, _ := os.ReadFile(out.Name())

	res := result{Stdout: string(stdout), Returns: returns, Files: map[string]string{}, Env: map[string]*string{}}
	for key, p := range files {
		data, err := os.ReadFile(p)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		res.Files[key] = string(data)
	}
	for _, k := range sc.ObserveEnv {
		if v, ok := os.LookupEnv(k); ok {
			v := v
			res.Env[k] = &v
		} else {
			res.Env[k] = nil
		}
	}
	if srv != nil {
		res.Requests = srv.Requests()
		srv.Close()
	}
	return res
}

func runJS(t *testing.T, scenariosFile string) map[string]result {
	t.Helper()
	// CI sets PARITY_REQUIRED so that a missing node never turns into a silent pass
	unavailable := t.Skip
	if os.Getenv("PARITY_REQUIRED") != "" {
		unavailable = t.Fatal
	}
	if _, err := exec.LookPath("node"); err != nil {
		unavailable("node is not installed")
	}
	if _, err := os.Stat("js/node_modules/@actions/core"); err != nil {
		unavailable("run `npm ci` in parity/js first")
	}
	cmd := exec.Command("node", "driver.mjs", scenariosFile)
	cmd.Dir = "js"
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

// normalisation of values that legitimately differ between runs/implementations:
// the file command delimiter is random in the reference, and the local server
// listens on a different port each time.
var (
	jsDelimiter = regexp.MustCompile(`ghadelimiter_[0-9a-f-]{36}`)
	serverAddr  = regexp.MustCompile(`http://127\.0\.0\.1:\d+`)
)

func (r result) normalise() result {
	n := func(s string) string {
		s = jsDelimiter.ReplaceAllString(s, "<DELIM>")
		s = strings.ReplaceAll(s, "_GitHubActionsGoFileCommandDelimeter_", "<DELIM>")
		return serverAddr.ReplaceAllString(s, "<SERVER>")
	}
	r.Stdout = n(r.Stdout)
	files := map[string]string{}
	for k, v := range r.Files {
		files[k] = n(v)
	}
	r.Files = files
	var returns []ret
	for _, x := range r.Returns {
		x.Error = n(x.Error)
		returns = append(returns, x)
	}
	r.Returns = returns
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

func TestParityWithReferenceCore(t *testing.T) {
	scenarios := loadScenarios(t)
	merged, _ := json.Marshal(scenarios)
	mergedFile := filepath.Join(t.TempDir(), "scenarios.json")
	if err := os.WriteFile(mergedFile, merged, 0o644); err != nil {
		t.Fatal(err)
	}

	js := runJS(t, mergedFile)
	known := knownDivergences(t)
	seen := map[string]bool{}
	for _, sc := range scenarios {
		sc := sc
		t.Run(sc.Name, func(t *testing.T) {
			want, ok := js[sc.Name]
			if !ok {
				t.Fatalf("js driver returned nothing for %s", sc.Name)
			}
			got := runGo(t, sc)
			g, _ := json.MarshalIndent(got.normalise(), "", "  ")
			w, _ := json.MarshalIndent(want.normalise(), "", "  ")
			same := bytes.Equal(g, w)
			if reason, isKnown := known[sc.Name]; isKnown {
				seen[sc.Name] = true
				if same {
					t.Errorf("scenario is listed in known-divergences.json (%s) but now matches: remove the entry", reason)
				}
				t.Skipf("known divergence: %s", reason)
			}
			if !same {
				t.Errorf("Go differs from @actions/core\n--- go\n%s\n--- js\n%s", g, w)
			}
		})
	}
	for name := range known {
		if !seen[name] {
			t.Errorf("known-divergences.json lists unknown scenario %q", name)
		}
	}
	fmt.Fprintf(os.Stderr, "parity: %d scenarios, %d known divergences\n", len(scenarios), len(known))
}
