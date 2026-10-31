package parity

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/actions-go/toolkit/core"
)

// summaryChained is what the JS driver reports for builder methods, which return the summary.
const summaryChained = "<summary>"

// truthy returns the argument as a string, or "" when JS would consider it
// falsy (absent, null, false, 0, ""): the reference skips falsy options.
func truthy(args []json.RawMessage, i int) string {
	if i >= len(args) {
		return ""
	}
	switch s := str(args[i]); s {
	case "false", "0":
		return ""
	default:
		return s
	}
}

func boolArg(args []json.RawMessage, i int) bool {
	return truthy(args, i) != ""
}

var headingLevel = regexp.MustCompile(`^[1-6]$`)

type tableCell struct {
	Data    json.RawMessage `json:"data"`
	Header  bool            `json:"header"`
	Colspan json.RawMessage `json:"colspan"`
	Rowspan json.RawMessage `json:"rowspan"`
}

// invokeSummary maps core.summary.<method> to the Go JobSummary builder.
func invokeSummary(t *testing.T, method string, args []json.RawMessage) ret {
	t.Helper()
	s := core.JobSummary
	arg := func(i int) string {
		if i >= len(args) {
			return ""
		}
		return str(args[i])
	}
	chained := ret{Value: summaryChained}
	switch method {
	case "addRaw":
		s.AddRaw(arg(0), boolArg(args, 1))
	case "addEOL":
		s.AddEOL()
	case "addCodeBlock":
		s.AddCodeBlock(arg(0), truthy(args, 1))
	case "addList":
		var items []string
		var raw []json.RawMessage
		mustUnmarshal(t, args[0], &raw)
		for _, r := range raw {
			items = append(items, str(r))
		}
		s.AddList(items, boolArg(args, 1))
	case "addTable":
		s.AddTable(tableRows(t, args[0]))
	case "addDetails":
		s.AddDetails(arg(0), arg(1))
	case "addImage":
		var opts struct {
			Width, Height json.RawMessage
		}
		if len(args) > 2 {
			mustUnmarshal(t, args[2], &opts)
		}
		s.AddImage(arg(0), arg(1), core.SummaryImageOptions{Width: truthyRaw(opts.Width), Height: truthyRaw(opts.Height)})
	case "addHeading":
		level := 0
		if l := arg(1); headingLevel.MatchString(l) {
			level = int(l[0] - '0')
		}
		s.AddHeading(arg(0), level)
	case "addSeparator":
		s.AddSeparator()
	case "addBreak":
		s.AddBreak()
	case "addQuote":
		s.AddQuote(arg(0), truthy(args, 1))
	case "addLink":
		s.AddLink(arg(0), arg(1))
	case "emptyBuffer":
		s.EmptyBuffer()
	case "stringify":
		return ret{Value: s.Stringify()}
	case "isEmptyBuffer":
		return ret{Value: s.IsEmptyBuffer()}
	case "write":
		var opts struct{ Overwrite bool }
		if len(args) > 0 {
			mustUnmarshal(t, args[0], &opts)
		}
		if err := s.Write(core.SummaryWriteOptions{Overwrite: opts.Overwrite}); err != nil {
			return ret{Error: err.Error()}
		}
	case "clear":
		if err := s.Clear(); err != nil {
			return ret{Error: err.Error()}
		}
	default:
		t.Fatalf("no Go adapter for summary.%s", method)
	}
	return chained
}

func truthyRaw(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	return truthy([]json.RawMessage{raw}, 0)
}

func tableRows(t *testing.T, raw json.RawMessage) [][]core.SummaryTableCell {
	var rows [][]json.RawMessage
	mustUnmarshal(t, raw, &rows)
	var out [][]core.SummaryTableCell
	for _, row := range rows {
		var cells []core.SummaryTableCell
		for _, c := range row {
			if strings.HasPrefix(strings.TrimSpace(string(c)), "{") {
				var cell tableCell
				mustUnmarshal(t, c, &cell)
				cells = append(cells, core.SummaryTableCell{
					Data: str(cell.Data), Header: cell.Header,
					Colspan: truthyRaw(cell.Colspan), Rowspan: truthyRaw(cell.Rowspan),
				})
			} else {
				// the reference accepts a bare string as a data cell
				cells = append(cells, core.SummaryTableCell{Data: str(c)})
			}
		}
		out = append(out, cells)
	}
	return out
}

func mustUnmarshal(t *testing.T, raw json.RawMessage, v interface{}) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatal(err)
	}
}

// invokePlatform maps the reference's platform module members to the Go package.
func invokePlatform(t *testing.T, member string) ret {
	t.Helper()
	switch member {
	case "platform":
		return ret{Value: core.Platform}
	case "arch":
		return ret{Value: core.Arch}
	case "isWindows":
		return ret{Value: core.IsWindows}
	case "isMacOS":
		return ret{Value: core.IsMacOS}
	case "isLinux":
		return ret{Value: core.IsLinux}
	case "getDetails":
		d, err := core.GetDetails()
		if err != nil {
			return ret{Error: err.Error()}
		}
		return ret{Value: map[string]interface{}{
			"name": d.Name, "version": d.Version, "platform": d.Platform, "arch": d.Arch,
			"isWindows": d.IsWindows, "isMacOS": d.IsMacOS, "isLinux": d.IsLinux,
		}}
	}
	t.Fatalf("no Go adapter for platform.%s", member)
	return ret{}
}

// serverSpec describes the local http server of a scenario: it answers with the
// responses in order and repeats the last one.
type serverSpec struct {
	Responses []struct {
		Status  int               `json:"status"`
		Body    string            `json:"body"`
		Headers map[string]string `json:"headers"`
	} `json:"responses"`
}

// request is what the server received; only headers the reference sets are recorded.
type request struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

var recordedHeaders = []string{"authorization", "accept", "content-type", "user-agent"}

type testServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []request
}

func startServer(t *testing.T, spec *serverSpec) *testServer {
	t.Helper()
	s := &testServer{}
	n := 0
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		s.mu.Lock()
		defer s.mu.Unlock()
		headers := map[string]string{}
		for _, h := range recordedHeaders {
			if v := r.Header.Values(h); len(v) > 0 {
				headers[h] = strings.Join(v, ", ")
			}
		}
		s.requests = append(s.requests, request{Method: r.Method, URL: r.URL.RequestURI(), Headers: headers})
		i := n
		if i >= len(spec.Responses) {
			i = len(spec.Responses) - 1
		}
		n++
		resp := spec.Responses[i]
		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(resp.Status)
		io.WriteString(w, resp.Body)
	}))
	t.Cleanup(s.Server.Close)
	return s
}

func (s *testServer) Requests() []request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]request(nil), s.requests...)
}
