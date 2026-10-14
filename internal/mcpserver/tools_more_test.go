package mcpserver

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/cache"
	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const idNew = "0b1c2d3e-0000-4000-8000-0000000000ff"

func moreStub() *stub {
	st := newStub()
	st.payloads["CCqFvf"] = `["",null,"` + idNew + `",null,null,null,null,null,null,null,null,[["thread-1"]]]`
	st.payloads["s0tc2d"] = `[]`
	st.payloads["fejl7e"] = `[]`
	return st
}

func findTool(t *testing.T, cs *mcp.ClientSession, name string) *mcp.Tool {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tl := range res.Tools {
		if tl.Name == name {
			return tl
		}
	}
	t.Fatalf("tool %s not registered", name)
	return nil
}

func TestMoreAnnotations(t *testing.T) {
	cs := connect(t, moreStub(), "all")
	want := map[string]struct{ destructive, idem bool }{
		"notebook_create": {false, false},
		"notebook_update": {false, true},
		"auth_login":      {false, false},
		"auth_logout":     {true, true},
		"chat_ask_many":   {false, false},
	}
	for name, w := range want {
		a := findTool(t, cs, name).Annotations
		if a == nil || a.ReadOnlyHint || a.DestructiveHint == nil || *a.DestructiveHint != w.destructive || a.IdempotentHint != w.idem {
			t.Errorf("%s annotations = %+v", name, a)
		}
	}
}

func TestChatAskManyIsOptIn(t *testing.T) {
	names := toolNames(t, connect(t, moreStub(), "default"))
	if slices.Contains(names, "chat_ask_many") {
		t.Fatal("chat_ask_many must be off in the default profile")
	}
	for _, n := range []string{"notebook_create", "notebook_update", "auth_login", "auth_logout"} {
		if !slices.Contains(names, n) {
			t.Errorf("%s missing from default profile", n)
		}
	}
}

func TestNotebookCreateWithSources(t *testing.T) {
	st := moreStub()
	st.payloads["izAoDd"] = `[[` + srcRow + `]]`
	cs := connect(t, st, "default")
	res := call(t, cs, "notebook_create", map[string]any{
		"title":   "Q3 plan",
		"sources": []any{map[string]any{"url": "https://93.184.216.34/page"}},
	})
	out := decode[NotebookCreateOut](t, res)
	if out.NotebookID != idNew || out.Title != "Q3 plan" {
		t.Fatalf("out = %+v", out)
	}
	if out.Sources == nil || out.Sources.Succeeded != 1 {
		t.Fatalf("sources = %+v", out.Sources)
	}
	if st.count("CCqFvf") != 1 || st.count("s0tc2d") != 1 || st.count("izAoDd") != 1 {
		t.Fatalf("calls create=%d rename=%d add=%d", st.count("CCqFvf"), st.count("s0tc2d"), st.count("izAoDd"))
	}
	for _, c := range st.calls {
		if c.ID == "CCqFvf" && c.Idempotent {
			t.Fatal("create must not be retried")
		}
	}
}

func TestNotebookCreateEmojiNote(t *testing.T) {
	cs := connect(t, moreStub(), "default")
	out := decode[NotebookCreateOut](t, call(t, cs, "notebook_create", map[string]any{"title": "Q3 plan", "emoji": "X"}))
	if len(out.Warnings) != 1 {
		t.Fatalf("warnings = %v", out.Warnings)
	}
}

func TestNotebookCreateBadInputCreatesNothing(t *testing.T) {
	st := moreStub()
	cs := connect(t, st, "default")
	if e := envelope(t, call(t, cs, "notebook_create", map[string]any{"title": " "})); e.Code != "INVALID_ARGUMENT" {
		t.Fatalf("code = %s", e.Code)
	}
	bad := map[string]any{"title": "T", "sources": []any{map[string]any{"url": "http://127.0.0.1/x"}}}
	if e := envelope(t, call(t, cs, "notebook_create", bad)); e.Code != "INVALID_ARGUMENT" {
		t.Fatalf("code = %s", e.Code)
	}
	drive := map[string]any{"title": "T", "sources": []any{map[string]any{"drive": map[string]any{"document_id": "d"}}}}
	if e := envelope(t, call(t, cs, "notebook_create", drive)); e.Code != "UNSUPPORTED_BY_TRANSPORT" {
		t.Fatalf("code = %s", e.Code)
	}
	if st.count("CCqFvf") != 0 {
		t.Fatal("nothing may be created for bad input")
	}
}

func TestNotebookCreateRenameMismatchKeepsID(t *testing.T) {
	st := moreStub()
	cs := connect(t, st, "default")
	out := decode[NotebookCreateOut](t, call(t, cs, "notebook_create", map[string]any{"title": "Different title"}))
	if out.NotebookID != idNew || len(out.Warnings) == 0 {
		t.Fatalf("out = %+v", out)
	}
}

func TestNotebookCreateFailure(t *testing.T) {
	st := moreStub()
	st.errs["CCqFvf"] = &transport.Error{Kind: transport.KindQuota, RPC: "notebooks.create"}
	cs := connect(t, st, "default")
	if e := envelope(t, call(t, cs, "notebook_create", map[string]any{"title": "T"})); e.Code != "QUOTA" {
		t.Fatalf("code = %s", e.Code)
	}
}

func TestNotebookUpdateRenameAndRecent(t *testing.T) {
	st := moreStub()
	cs := connect(t, st, "default")
	out := decode[NotebookUpdateOut](t, call(t, cs, "notebook_update", map[string]any{
		"notebook": idA, "title": "Q3 plan", "remove_from_recent": true, "alias": "q3",
	}))
	if !out.Renamed || !out.RemovedFromRecent || out.NotebookID != idA || len(out.Notes) != 1 {
		t.Fatalf("out = %+v", out)
	}
	if st.count("s0tc2d") != 1 || st.count("fejl7e") != 1 {
		t.Fatalf("calls rename=%d recent=%d", st.count("s0tc2d"), st.count("fejl7e"))
	}
}

func TestNotebookUpdateInvalidatesList(t *testing.T) {
	st := moreStub()
	cs := connect(t, st, "default")
	call(t, cs, "notebook_list", map[string]any{})
	call(t, cs, "notebook_update", map[string]any{"notebook": idA, "title": "Q3 plan"})
	call(t, cs, "notebook_list", map[string]any{})
	// list, update's fresh resolve list, then list again after invalidation.
	if st.count("wXbhsf") != 3 {
		t.Fatalf("list calls = %d, cache not invalidated", st.count("wXbhsf"))
	}
}

func TestNotebookUpdateRejectsFuzzyTitle(t *testing.T) {
	st := moreStub()
	cs := connect(t, st, "default")
	for _, args := range []map[string]any{
		{"notebook": "research", "remove_from_recent": true},
		{"notebook": "research", "title": "New"},
	} {
		if e := envelope(t, call(t, cs, "notebook_update", args)); e.Code != "NOT_FOUND" || len(e.Suggestions) == 0 {
			t.Fatalf("%v: envelope = %+v", args, e)
		}
	}
	if st.count("fejl7e") != 0 || st.count("s0tc2d") != 0 {
		t.Fatal("fuzzy match reached a mutation")
	}
}

func TestNotebookUpdateErrors(t *testing.T) {
	st := moreStub()
	cs := connect(t, st, "default")
	if e := envelope(t, call(t, cs, "notebook_update", map[string]any{"notebook": idA})); e.Code != "INVALID_ARGUMENT" {
		t.Fatalf("empty update code = %s", e.Code)
	}
	if e := envelope(t, call(t, cs, "notebook_update", map[string]any{"notebook": idA, "pinned": true})); e.Code != "UNSUPPORTED_BY_TRANSPORT" {
		t.Fatalf("pinned-only code = %s", e.Code)
	}
	st.errs["s0tc2d"] = &transport.Error{Kind: transport.KindPermission, RPC: "notebooks.mutate"}
	if e := envelope(t, call(t, cs, "notebook_update", map[string]any{"notebook": idA, "title": "X"})); e.Code != "PERMISSION_DENIED" {
		t.Fatalf("code = %s", e.Code)
	}
}

func TestAuthToolsAreCLIOnly(t *testing.T) {
	cs := connect(t, moreStub(), "default")
	for name, args := range map[string]any{
		"auth_login":  map[string]any{"method": "import"},
		"auth_logout": map[string]any{"confirm": true},
	} {
		e := envelope(t, call(t, cs, name, args))
		if e.Code != "UNSUPPORTED_BY_TRANSPORT" || e.Error == "" {
			t.Errorf("%s: %+v", name, e)
		}
	}
}

// manyStub streams one fixed answer, fails for failNB and tracks concurrency.
type manyStub struct {
	*stub
	failNB string
	mu     sync.Mutex
	cur    int
	max    int
}

func (m *manyStub) Stream(ctx context.Context, nb string, _ any, onSnap func(wire.Snapshot)) (wire.Snapshot, error) {
	m.mu.Lock()
	m.cur++
	m.max = max(m.max, m.cur)
	m.mu.Unlock()
	defer func() { m.mu.Lock(); m.cur--; m.mu.Unlock() }()
	time.Sleep(30 * time.Millisecond)
	if nb == m.failNB {
		return wire.Snapshot{}, &transport.Error{Kind: transport.KindRateLimited, RPC: "chat.ask", Retriable: true}
	}
	sn := wire.Snapshot{Text: "answer for " + nb}
	onSnap(sn)
	return sn, nil
}

func connectMany(t *testing.T, m *manyStub) *mcp.ClientSession {
	t.Helper()
	m.stub.payloads["hPTbtc"] = `[[["conv-1",[1700000000,0],[1700000001,0]]]]`
	srv := New(Deps{NLM: nlm.New(m), Cache: cache.New(nil), Profile: "default", Host: "notebook.google.com"}, "all")
	ct, stt := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(context.Background(), stt, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil).Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func TestChatAskManyPartialFailure(t *testing.T) {
	m := &manyStub{stub: moreStub(), failNB: idB}
	cs := connectMany(t, m)
	out := decode[ChatAskManyOut](t, call(t, cs, "chat_ask_many", map[string]any{
		"question": "what?", "notebooks": []any{idA, idB, "Shared research"},
	}))
	if out.Asked != 3 || out.Succeeded != 2 || out.Failed != 1 {
		t.Fatalf("out = %+v", out)
	}
	if r := out.Results[0]; r.Status != "ok" || r.Answer != "answer for "+idA || r.ConversationID != "conv-1" {
		t.Fatalf("result 0 = %+v", r)
	}
	if r := out.Results[1]; r.Status != "failed" || r.Code != "RATE_LIMITED" {
		t.Fatalf("result 1 = %+v", r)
	}
	if r := out.Results[2]; r.NotebookID != idC || r.Status != "ok" {
		t.Fatalf("result 2 = %+v", r)
	}
}

func TestChatAskManyHonoursMaxParallel(t *testing.T) {
	m := &manyStub{stub: moreStub()}
	cs := connectMany(t, m)
	nbs := []any{idA, idB, idC, "0b1c2d3e-0000-4000-8000-0000000000a1", "0b1c2d3e-0000-4000-8000-0000000000a2"}
	out := decode[ChatAskManyOut](t, call(t, cs, "chat_ask_many", map[string]any{"question": "q", "notebooks": nbs, "max_parallel": 2}))
	if out.Succeeded != 5 {
		t.Fatalf("out = %+v", out)
	}
	if m.max > 2 || m.max < 2 {
		t.Fatalf("max concurrency = %d, want 2", m.max)
	}
}

func TestChatAskManyDuplicateAndUnknown(t *testing.T) {
	m := &manyStub{stub: moreStub()}
	cs := connectMany(t, m)
	out := decode[ChatAskManyOut](t, call(t, cs, "chat_ask_many", map[string]any{
		"question": "q", "notebooks": []any{idA, idA, "no such notebook"},
	}))
	if out.Succeeded != 1 || out.Failed != 2 {
		t.Fatalf("out = %+v", out)
	}
}

func TestChatAskManyInvalid(t *testing.T) {
	cs := connectMany(t, &manyStub{stub: moreStub()})
	for _, tc := range []struct {
		args map[string]any
		code string
	}{
		{map[string]any{"question": " ", "notebooks": []any{idA}}, "INVALID_ARGUMENT"},
		{map[string]any{"question": "q"}, "INVALID_ARGUMENT"},
		{map[string]any{"question": "q", "tag": "work"}, "UNSUPPORTED_BY_TRANSPORT"},
		{map[string]any{"question": "q", "notebooks": []any{idA}, "max_parallel": -1}, "INVALID_ARGUMENT"},
	} {
		if e := envelope(t, call(t, cs, "chat_ask_many", tc.args)); e.Code != tc.code {
			t.Errorf("%v: code = %s, want %s", tc.args, e.Code, tc.code)
		}
	}
}
