// Package githubparity compares the github package with the reference
// JavaScript implementation (@actions/github, installed next to @actions/core
// in ../js).
//
// Every scenario in scenarios/*.json is run through both implementations. The
// observable behaviour (context fields, payload values, stdout, the HTTP
// request a client sends to a local server) must be identical unless a
// documented entry exists in known-divergences.json.
//
// DownloadSelectedRepositoryFiles and MatchesOneOf have no equivalent in
// @actions/github and are not covered.
package githubparity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	gh "github.com/actions-go/toolkit/github"
	gogithub "github.com/google/go-github/v74/github"
)

type scenario struct {
	Name string `json:"name"`
	// Kind is one of context, userAgent, baseUrl, request.
	Kind string            `json:"kind"`
	Env  map[string]string `json:"env"`

	// the event file GITHUB_EVENT_PATH points to, unless Env sets it itself
	Event        json.RawMessage `json:"event,omitempty"`
	EventRaw     *string         `json:"eventRaw,omitempty"`
	EventFixture string          `json:"eventFixture,omitempty"`

	PayloadPaths []string `json:"payloadPaths,omitempty"`

	Base *string `json:"base,omitempty"` // userAgent: the base user agent, null for none

	Token  string `json:"token,omitempty"` // request
	Call   string `json:"call,omitempty"`  // request: repos.get or issues.createComment
	Params struct {
		Owner  string `json:"owner"`
		Repo   string `json:"repo"`
		Number int    `json:"number"`
		Body   string `json:"body"`
	} `json:"params"`
}

type fields struct {
	EventName  string `json:"eventName"`
	SHA        string `json:"sha"`
	Ref        string `json:"ref"`
	Workflow   string `json:"workflow"`
	Action     string `json:"action"`
	Actor      string `json:"actor"`
	Job        string `json:"job"`
	RunNumber  int64  `json:"runNumber"`
	RunID      int64  `json:"runId"`
	RunAttempt int64  `json:"runAttempt"`
	APIURL     string `json:"apiUrl"`
	ServerURL  string `json:"serverUrl"`
	GraphqlURL string `json:"graphqlUrl"`
}

type captured struct {
	Value interface{} `json:"value,omitempty"`
	Error string      `json:"error,omitempty"`
}

// recorded is a request received by the local server.
type recorded struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   string            `json:"query"`
	Headers map[string]string `json:"headers"`
	Body    interface{}       `json:"body"`
}

type result struct {
	Stdout           string                 `json:"stdout,omitempty"`
	ConstructorError string                 `json:"constructorError,omitempty"`
	Fields           *fields                `json:"fields,omitempty"`
	Repo             *captured              `json:"repo,omitempty"`
	Issue            *captured              `json:"issue,omitempty"`
	Payload          map[string]interface{} `json:"payload,omitempty"`
	Value            interface{}            `json:"value,omitempty"`
	Status           int                    `json:"status,omitempty"`
	Error            string                 `json:"error,omitempty"`
	Requests         []recorded             `json:"requests,omitempty"`
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
	names := map[string]bool{}
	for i := range all {
		sc := &all[i]
		if names[sc.Name] {
			t.Fatalf("duplicate scenario %q", sc.Name)
		}
		names[sc.Name] = true
		if sc.EventFixture != "" {
			// fixtures are the real event payloads shipped with the github package
			data, err := os.ReadFile(filepath.Join("..", "..", "github", sc.EventFixture))
			if err != nil {
				t.Fatal(err)
			}
			raw := string(data)
			sc.EventRaw, sc.EventFixture = &raw, ""
		}
	}
	return all
}

// recorder is a local HTTP server that remembers what it received, by
// scenario: the first path segment is the scenario name (see {{SERVER}}).
type recorder struct {
	*httptest.Server
	mu   sync.Mutex
	reqs map[string][]recorded
}

// headers that say nothing about the toolkit: connection handling, content
// negotiation defaults of the HTTP stack, the host the server is on, and the
// per-endpoint media type and the REST API version that octokit and go-github
// each pick for themselves (go-github v74 sends X-GitHub-Api-Version, v42 did not).
var ignoredHeaders = map[string]bool{
	"host": true, "connection": true, "keep-alive": true, "content-length": true,
	"accept-encoding": true, "accept-language": true, "sec-fetch-mode": true,
	"transfer-encoding": true, "accept": true, "x-github-api-version": true,
}

// the user agent starts with the name of the HTTP library (octokit-core.js,
// go-github): only the orchestration tag, which the toolkit adds, is compared.
var orchestrationTag = regexp.MustCompile(`actions_orchestration_id/\S+`)

func newRecorder(t *testing.T) *recorder {
	r := &recorder{reqs: map[string][]recorded{}}
	r.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		rest := strings.TrimPrefix(req.URL.Path, "/")
		name, path, _ := strings.Cut(rest, "/")
		name, _ = url.PathUnescape(name)
		rec := recorded{Method: req.Method, Path: "/" + path, Query: req.URL.RawQuery, Headers: map[string]string{}}
		for k, v := range req.Header {
			k = strings.ToLower(k)
			switch {
			case ignoredHeaders[k]:
			case k == "user-agent":
				rec.Headers[k] = orchestrationTag.FindString(strings.Join(v, ","))
			case k == "content-type":
				// JSON is UTF-8 unless said otherwise: octokit spells the charset out, go-github does not
				rec.Headers[k] = strings.TrimSuffix(strings.Join(v, ","), "; charset=utf-8")
			default:
				rec.Headers[k] = strings.Join(v, ",")
			}
		}
		if len(body) > 0 {
			var parsed interface{}
			if json.Unmarshal(body, &parsed) == nil {
				rec.Body = parsed
			} else {
				rec.Body = string(body)
			}
		}
		r.mu.Lock()
		r.reqs[name] = append(r.reqs[name], rec)
		r.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{}"))
	}))
	t.Cleanup(r.Close)
	return r
}

func (r *recorder) take() map[string][]recorded {
	r.mu.Lock()
	defer r.mu.Unlock()
	taken := r.reqs
	r.reqs = map[string][]recorded{}
	return taken
}

func serverFor(r *recorder, sc scenario) string {
	return r.URL + "/" + url.PathEscape(sc.Name)
}

func lookup(root interface{}, dotted string) interface{} {
	cur := root
	for _, part := range strings.Split(dotted, ".") {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil
		}
		cur = m[part]
	}
	return cur
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	out, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	real := os.Stdout
	os.Stdout = out
	func() {
		defer func() { os.Stdout = real; out.Close() }()
		fn()
	}()
	data, _ := os.ReadFile(out.Name())
	return string(data)
}

func setEnv(t *testing.T, sc scenario, srv string) {
	t.Helper()
	for _, kv := range os.Environ() {
		k := strings.SplitN(kv, "=", 2)[0]
		if cleanedEnv.MatchString(k) {
			t.Setenv(k, "") // restores the variable when the test ends
			os.Unsetenv(k)
		}
	}
	if sc.Event != nil || sc.EventRaw != nil {
		file := filepath.Join(t.TempDir(), "event.json")
		content := string(sc.Event)
		if sc.EventRaw != nil {
			content = *sc.EventRaw
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GITHUB_EVENT_PATH", file)
	}
	for k, v := range sc.Env {
		t.Setenv(k, strings.ReplaceAll(v, "{{SERVER}}", srv))
	}
}

func runGo(t *testing.T, sc scenario, rec *recorder) result {
	t.Helper()
	srv := serverFor(rec, sc)
	setEnv(t, sc, srv)
	switch sc.Kind {
	case "context":
		return goContext(t, sc)
	case "userAgent":
		base := ""
		if sc.Base != nil {
			base = *sc.Base
		}
		return result{Value: gh.UserAgentWithOrchestrationID(base)}
	case "baseUrl":
		return result{Value: strings.TrimSuffix(gh.NewClient().BaseURL.String(), "/")}
	case "request":
		return goRequest(t, sc, rec)
	}
	t.Fatalf("unknown scenario kind %q", sc.Kind)
	return result{}
}

func goContext(t *testing.T, sc scenario) result {
	var ctx gh.ActionContext
	stdout := captureStdout(t, func() { ctx = gh.ParseActionEnv() })
	res := result{
		Stdout: stdout,
		Fields: &fields{
			EventName: ctx.EventName, SHA: ctx.SHA, Ref: ctx.Ref, Workflow: ctx.Workflow,
			Action: ctx.Action, Actor: ctx.Actor, Job: ctx.Job,
			RunNumber: ctx.RunNumber, RunID: ctx.RunID, RunAttempt: ctx.RunAttempt,
			APIURL: ctx.ApiUrl, ServerURL: ctx.ServerUrl, GraphqlURL: ctx.GraphqlUrl,
		},
		Repo:    &captured{Value: map[string]interface{}{"owner": ctx.Repo.Owner, "repo": ctx.Repo.Repo}},
		Issue:   &captured{Value: map[string]interface{}{"owner": ctx.Issue.Owner, "repo": ctx.Issue.Repo, "number": ctx.Issue.Number}},
		Payload: map[string]interface{}{},
	}
	if len(sc.PayloadPaths) > 0 {
		// the typed payload is looked at through its JSON form, as the reference payload is plain JSON
		data, err := json.Marshal(ctx.Payload)
		if err != nil {
			t.Fatal(err)
		}
		var generic interface{}
		if err := json.Unmarshal(data, &generic); err != nil {
			t.Fatal(err)
		}
		for _, p := range sc.PayloadPaths {
			res.Payload[p] = lookup(generic, p)
		}
		// WebhookPayload.Action is a plain string, absent and empty look the same
		if res.Payload["action"] == "" {
			res.Payload["action"] = nil
		}
	}
	return res
}

func goRequest(t *testing.T, sc scenario, rec *recorder) result {
	if sc.Token != "" {
		t.Setenv("GITHUB_TOKEN", sc.Token)
	}
	client := gh.NewClient()
	ctx := context.Background()
	p := sc.Params
	var err error
	var resp *gogithub.Response
	switch sc.Call {
	case "repos.get":
		_, resp, err = client.Repositories.Get(ctx, p.Owner, p.Repo)
	case "issues.createComment":
		_, resp, err = client.Issues.CreateComment(ctx, p.Owner, p.Repo, p.Number, &gogithub.IssueComment{Body: &p.Body})
	default:
		t.Fatalf("no Go adapter for call %q", sc.Call)
	}
	res := result{}
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Status = resp.StatusCode
	}
	return res
}

func runJS(t *testing.T, scenarios []scenario, rec *recorder) map[string]result {
	t.Helper()
	// CI sets PARITY_REQUIRED so that a missing node never turns into a silent pass
	unavailable := t.Skip
	if os.Getenv("PARITY_REQUIRED") != "" {
		unavailable = t.Fatal
	}
	if _, err := exec.LookPath("node"); err != nil {
		unavailable("node is not installed")
	}
	if _, err := os.Stat("../js/node_modules/@actions/github"); err != nil {
		unavailable("run `npm ci` in parity/js first, with @actions/github installed")
	}
	merged, _ := json.Marshal(scenarios)
	file := filepath.Join(t.TempDir(), "scenarios.json")
	if err := os.WriteFile(file, merged, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "github-driver.mjs", file, rec.URL)
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
	seen := rec.take()
	for name, r := range res {
		r.Requests = seen[name]
		res[name] = r
	}
	return res
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

func TestParityWithReferenceGithub(t *testing.T) {
	scenarios := loadScenarios(t)
	rec := newRecorder(t)
	js := runJS(t, scenarios, rec)
	known := knownDivergences(t)
	seen := map[string]bool{}
	for _, sc := range scenarios {
		sc := sc
		t.Run(sc.Name, func(t *testing.T) {
			want, ok := js[sc.Name]
			if !ok {
				t.Fatalf("js driver returned nothing for %s", sc.Name)
			}
			got := runGo(t, sc, rec)
			got.Requests = rec.take()[sc.Name]
			g, _ := json.MarshalIndent(got, "", "  ")
			w, _ := json.MarshalIndent(want, "", "  ")
			same := bytes.Equal(g, w)
			if reason, isKnown := known[sc.Name]; isKnown {
				seen[sc.Name] = true
				if same {
					t.Errorf("scenario is listed in known-divergences.json (%s) but now matches: remove the entry", reason)
				}
				t.Skipf("known divergence: %s", reason)
			}
			if !same {
				t.Errorf("Go differs from @actions/github\n--- go\n%s\n--- js\n%s", g, w)
			}
		})
	}
	for name := range known {
		if !seen[name] {
			t.Errorf("known-divergences.json lists unknown scenario %q", name)
		}
	}
	fmt.Fprintf(os.Stderr, "parity/github: %d scenarios, %d known divergences\n", len(scenarios), len(known))
}
