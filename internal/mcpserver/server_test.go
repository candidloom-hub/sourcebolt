package mcpserver

import (
	"context"
	"encoding/json/v2"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/cache"
	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	idA = "0b1c2d3e-0000-4000-8000-00000000000a"
	idB = "0b1c2d3e-0000-4000-8000-00000000000b"
	idC = "0b1c2d3e-0000-4000-8000-00000000000c"
)

var listPayload = `[[` +
	`["Q3 plan",[],"` + idA + `","",null,[1]],` +
	`["Q3 planning notes",[],"` + idB + `","",null,[1]],` +
	`["Shared research",[],"` + idC + `","",null,[2]]]]`

var getPayload = `[["Q3 plan",[[["s1"],"Doc",null,[null,2]]],"` + idA + `","",null,[1]]]`

// stub answers calls by wire id with fixed payloads or errors.
type stub struct {
	mu       sync.Mutex
	payloads map[string]string
	errs     map[string]error
	block    bool
	calls    []transport.Call
	seenCtx  chan context.Context
}

func newStub() *stub {
	return &stub{
		payloads: map[string]string{"wXbhsf": listPayload, "rLM1Ne": getPayload, "VfAZjd": `[[["sum"],[]]]`},
		errs:     map[string]error{},
		seenCtx:  make(chan context.Context, 8),
	}
}

func (s *stub) Name() string         { return "rpc" }
func (s *stub) Supports(string) bool { return true }
func (s *stub) Do(ctx context.Context, c transport.Call) (transport.Result, error) {
	s.mu.Lock()
	s.calls = append(s.calls, c)
	p, err, block := s.payloads[c.ID], s.errs[c.ID], s.block
	s.mu.Unlock()
	if block {
		s.seenCtx <- ctx
		<-ctx.Done()
		return transport.Result{}, ctx.Err()
	}
	if err != nil {
		return transport.Result{}, err
	}
	return transport.Result{Payload: []byte(p), Transport: "rpc"}, nil
}

func (s *stub) count(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, c := range s.calls {
		if c.ID == id {
			n++
		}
	}
	return n
}

func connect(t *testing.T, st *stub, profile string) *mcp.ClientSession {
	t.Helper()
	srv := New(Deps{NLM: nlm.New(st), Cache: cache.New(nil), Profile: "default", Host: "notebook.google.com"}, profile)
	ct, stt := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, stt, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res
}

func text(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func envelope(t *testing.T, res *mcp.CallToolResult) Envelope {
	t.Helper()
	if !res.IsError {
		t.Fatalf("want error result, got %s", text(res))
	}
	var e Envelope
	if err := json.Unmarshal([]byte(text(res)), &e); err != nil {
		t.Fatalf("envelope is not JSON: %q", text(res))
	}
	return e
}

func toolNames(t *testing.T, cs *mcp.ClientSession) []string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range res.Tools {
		names = append(names, tl.Name)
	}
	slices.Sort(names)
	return names
}

func TestToolsListDefault(t *testing.T) {
	got := toolNames(t, connect(t, newStub(), "default"))
	for _, want := range []string{"notebook_list", "notebook_open", "server_info"} {
		if !slices.Contains(got, want) {
			t.Fatalf("tools = %v, missing %s", got, want)
		}
	}
	res, _ := connect(t, newStub(), "default").ListTools(context.Background(), nil)
	for _, tl := range res.Tools {
		if !slices.Contains([]string{"notebook_list", "notebook_open", "server_info"}, tl.Name) {
			continue
		}
		if tl.Annotations == nil || !tl.Annotations.ReadOnlyHint || !tl.Annotations.IdempotentHint {
			t.Errorf("%s annotations = %+v", tl.Name, tl.Annotations)
		}
	}
}

func TestMinimalProfileHidesNonMinimal(t *testing.T) {
	got := toolNames(t, connect(t, newStub(), "minimal"))
	for _, n := range got {
		if !minimal[n] {
			t.Errorf("minimal profile exposes %s", n)
		}
	}
}

func TestNotebookListFiltersQuery(t *testing.T) {
	cs := connect(t, newStub(), "default")
	var out NotebookListOut
	if err := json.Unmarshal([]byte(text(call(t, cs, "notebook_list", map[string]any{"query": "PLAN"}))), &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 2 || len(out.Notebooks) != 2 {
		t.Fatalf("out = %+v", out)
	}
	if err := json.Unmarshal([]byte(text(call(t, cs, "notebook_list", map[string]any{"scope": "shared"}))), &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 || out.Notebooks[0].ID != idC {
		t.Fatalf("shared = %+v", out)
	}
}

func TestNotebookListCached(t *testing.T) {
	st := newStub()
	cs := connect(t, st, "default")
	call(t, cs, "notebook_list", nil)
	call(t, cs, "notebook_list", nil)
	if n := st.count("wXbhsf"); n != 1 {
		t.Fatalf("upstream calls = %d, want 1 (cached)", n)
	}
	call(t, cs, "notebook_list", map[string]any{"refresh": true})
	if n := st.count("wXbhsf"); n != 2 {
		t.Fatalf("refresh must bypass cache: calls = %d", n)
	}
}

func TestNotebookOpenByTitle(t *testing.T) {
	st := newStub()
	cs := connect(t, st, "default")
	res := call(t, cs, "notebook_open", map[string]any{"notebook": "q3 plan"})
	if res.IsError {
		t.Fatalf("error: %s", text(res))
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	var got string
	for _, c := range st.calls {
		if c.ID == "rLM1Ne" {
			got = c.SourcePath
		}
	}
	if got != "/notebook/"+idA {
		t.Fatalf("GetNotebook source-path = %q", got)
	}
}

func TestAmbiguousEnvelope(t *testing.T) {
	cs := connect(t, newStub(), "default")
	e := envelope(t, call(t, cs, "notebook_open", map[string]any{"notebook": "plan"}))
	if e.Code != "AMBIGUOUS" || len(e.Suggestions) != 2 || !strings.Contains(e.Suggestions[0], "Q3 plan") {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestAuthEnvelope(t *testing.T) {
	st := newStub()
	st.errs["wXbhsf"] = &transport.Error{Kind: transport.KindAuth, RPC: "notebooks.list", Transport: "rpc"}
	cs := connect(t, st, "default")
	e := envelope(t, call(t, cs, "notebook_list", nil))
	if e.Code != "AUTH_REQUIRED" || !strings.Contains(e.Guidance, "sourcebolt login") || e.Transport != "rpc" || e.Status != "error" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestCtxPropagates(t *testing.T) {
	st := newStub()
	st.block = true
	cs := connect(t, st, "default")
	ctx, cancel := context.WithCancel(context.Background())
	go cs.CallTool(ctx, &mcp.CallToolParams{Name: "notebook_list"})
	var seen context.Context
	select {
	case seen = <-st.seenCtx:
	case <-time.After(2 * time.Second):
		t.Fatal("transport never called")
	}
	cancel()
	select {
	case <-seen.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("client cancellation did not reach the transport ctx")
	}
}

func TestServerInfo(t *testing.T) {
	cs := connect(t, newStub(), "default")
	var out ServerInfoOut
	if err := json.Unmarshal([]byte(text(call(t, cs, "server_info", nil))), &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != "sourcebolt" || out.Profile != "default" || out.Host != "notebook.google.com" || out.ToolProfile != "default" {
		t.Fatalf("out = %+v", out)
	}
}

func TestEnvelopeRedactsSecrets(t *testing.T) {
	st := newStub()
	leak := errors.New("POST https://h/x?f.sid=-1234567&at=AJpMsecret:1 Cookie: SID=sidsecret")
	st.errs["wXbhsf"] = &transport.Error{Kind: transport.KindTransport, RPC: "notebooks.list", Cause: leak}
	st.errs["VfAZjd"] = &transport.Error{Kind: transport.KindTransport, RPC: "notebooks.guide", Cause: leak}
	cs := connect(t, st, "default")
	for _, res := range []*mcp.CallToolResult{
		call(t, cs, "notebook_list", nil),
		call(t, cs, "notebook_open", map[string]any{"notebook": idA}),
	} {
		s := text(res)
		for _, secret := range []string{"-1234567", "AJpMsecret", "sidsecret"} {
			if strings.Contains(s, secret) {
				t.Errorf("tool output leaks %q: %s", secret, s)
			}
		}
	}
}
