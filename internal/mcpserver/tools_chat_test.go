package mcpserver

import (
	"context"
	"encoding/json/v2"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/candidloom-hub/sourcebolt/internal/cache"
	"github.com/candidloom-hub/sourcebolt/internal/jobs"
	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// streamStub is a stub transport that also implements nlm.Streamer.
type streamStub struct {
	*stub
	snaps []wire.Snapshot
	delay time.Duration // pause before each snapshot
	block bool          // wait for ctx cancellation instead of answering
}

func (s *streamStub) Stream(ctx context.Context, _ string, _ any, onSnap func(wire.Snapshot)) (wire.Snapshot, error) {
	if s.block {
		<-ctx.Done()
		return wire.Snapshot{}, ctx.Err()
	}
	var last wire.Snapshot
	for _, sn := range s.snaps {
		time.Sleep(s.delay)
		onSnap(sn)
		last = sn
	}
	return last, nil
}

type progressLog struct {
	mu   sync.Mutex
	msgs []string
	vals []float64
}

func (p *progressLog) handler(_ context.Context, r *mcp.ProgressNotificationClientRequest) {
	p.mu.Lock()
	p.msgs, p.vals = append(p.msgs, r.Params.Message), append(p.vals, r.Params.Progress)
	p.mu.Unlock()
}

func connectChat(t *testing.T, tr *streamStub, jt *jobs.Table, pl *progressLog) *mcp.ClientSession {
	t.Helper()
	srv := New(Deps{NLM: nlm.New(tr), Cache: cache.New(nil), Profile: "default", Host: "notebook.google.com", Jobs: jt}, "default")
	ct, stt := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, stt, nil); err != nil {
		t.Fatal(err)
	}
	var opts *mcp.ClientOptions
	if pl != nil {
		opts = &mcp.ClientOptions{ProgressNotificationHandler: pl.handler}
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, opts).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func newChatStub() *streamStub {
	st := newStub()
	st.payloads["hPTbtc"] = `[[["conv-1",[1700000000,0],[1700000001,0]]]]`
	st.payloads["s0tc2d"] = `[]`
	st.payloads["XgrPMd"] = `[]`
	st.payloads["otmP3b"] = `[[["Title one","Prompt one"]]]`
	st.payloads["khqZz"] = `[[[null,[1700000001,0],2,null,[["hi there"]]],[null,[1700000000,0],1,"hello"]],"cur-2"]`
	return &streamStub{stub: st, snaps: []wire.Snapshot{
		{Text: "Partial"},
		{Text: "Partial answer [1]", Citations: []wire.Citation{{N: 1, SourceID: "s1", Text: "passage", Start: 15, End: 18}}},
	}}
}

func TestAskProgressSendsTailOnly(t *testing.T) {
	var msgs []string
	var vals []float64
	p := askProgress(func(m string, v, _ float64) { msgs, vals = append(msgs, m), append(vals, v) })

	long := strings.Repeat("é", 300) + strings.Repeat("x", 300) // 600 runes, multi-byte
	p("short")
	p(long)
	p(long) // same text again: progress must still increase
	if msgs[0] != "short" || vals[0] != 5 {
		t.Fatalf("short: %q %v", msgs[0], vals[0])
	}
	if n := utf8.RuneCountInString(msgs[1]); n > progressTail+1 || !utf8.ValidString(msgs[1]) {
		t.Fatalf("message is %d runes (valid=%v), want at most ~%d", n, utf8.ValidString(msgs[1]), progressTail)
	}
	if !strings.HasSuffix(long, strings.TrimPrefix(msgs[1], "…")) {
		t.Fatalf("message is not the answer's tail: %q", msgs[1])
	}
	if vals[1] != 600 {
		t.Fatalf("progress = %v, want the answer length 600", vals[1])
	}
	if vals[2] <= vals[1] {
		t.Fatalf("progress not increasing: %v", vals)
	}
}

func TestChatAskProgressAndCitations(t *testing.T) {
	tr := newChatStub()
	tr.delay = 1100 * time.Millisecond // Ask throttles progress to one per second
	pl := &progressLog{}
	cs := connectChat(t, tr, nil, pl)

	params := &mcp.CallToolParams{Name: "chat_ask", Arguments: map[string]any{"notebook": idA, "question": "what?"}}
	params.SetProgressToken("tok")
	res, err := cs.CallTool(context.Background(), params)
	if err != nil || res.IsError {
		t.Fatalf("chat_ask: %v %v", err, res)
	}
	var out ChatAskOut
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil {
		t.Fatal(err)
	}
	if out.Answer != "Partial answer [1]" || out.ConversationID != "conv-1" {
		t.Fatalf("out = %+v", out)
	}
	if len(out.Citations) != 1 || out.Citations[0].SourceTitle != "Doc" || out.Citations[0].Start != 15 || out.Citations[0].End != 18 {
		t.Fatalf("citations = %+v", out.Citations)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		pl.mu.Lock()
		n := len(pl.msgs)
		pl.mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if len(pl.msgs) != 2 || pl.msgs[0] != "Partial" || pl.msgs[1] != "Partial answer [1]" {
		t.Fatalf("progress messages = %v", pl.msgs)
	}
	if !(pl.vals[0] < pl.vals[1]) {
		t.Fatalf("progress not increasing: %v", pl.vals)
	}
}

func TestChatAskErrors(t *testing.T) {
	cs := connectChat(t, newChatStub(), nil, nil)
	if e := envelope(t, call(t, cs, "chat_ask", map[string]any{"notebook": idA, "question": "  "})); e.Code != "INVALID_ARGUMENT" {
		t.Fatalf("empty question: %+v", e)
	}
	f := false
	e := envelope(t, call(t, cs, "chat_ask", map[string]any{"notebook": idA, "question": "q", "wait": f}))
	if e.Code != "UNSUPPORTED_BY_TRANSPORT" {
		t.Fatalf("wait=false without jobs: %+v", e)
	}
	e = envelope(t, call(t, cs, "chat_ask", map[string]any{"notebook": idA, "question": "q", "sources": []string{"nope-nope"}}))
	if e.Code != "NOT_FOUND" {
		t.Fatalf("unknown source: %+v", e)
	}
}

func TestChatHistory(t *testing.T) {
	cs := connectChat(t, newChatStub(), nil, nil)
	res := call(t, cs, "chat_history", map[string]any{"notebook": idA, "format": "markdown"})
	var out ChatHistoryOut
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil {
		t.Fatal(err)
	}
	if out.ConversationID != "conv-1" || out.NextCursor != "cur-2" || len(out.Turns) != 2 || out.Turns[0].Role != "user" {
		t.Fatalf("out = %+v", out)
	}
	if want := "**You**\n\nhello\n\n**Assistant**\n\nhi there"; out.Markdown != want {
		t.Fatalf("markdown = %q", out.Markdown)
	}
	if e := envelope(t, call(t, cs, "chat_history", map[string]any{"notebook": idA, "format": "xml"})); e.Code != "INVALID_ARGUMENT" {
		t.Fatalf("bad format: %+v", e)
	}
}

func TestChatConfigure(t *testing.T) {
	tr := newChatStub()
	cs := connectChat(t, tr, nil, nil)
	res := call(t, cs, "chat_configure", map[string]any{"notebook": idA, "mode": "learning_guide", "response_length": "shorter"})
	var out ChatConfigureOut
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil || !out.Updated || out.Mode != "learning_guide" || out.Length != "shorter" {
		t.Fatalf("configure = %q %v", text(res), err)
	}
	if tr.count("s0tc2d") != 1 {
		t.Fatalf("mutate calls = %d", tr.count("s0tc2d"))
	}
	for _, tc := range []struct {
		args map[string]any
		code string
	}{
		{map[string]any{"notebook": idA, "mode": "wild"}, "INVALID_ARGUMENT"},
		{map[string]any{"notebook": idA}, "INVALID_ARGUMENT"},
		{map[string]any{"notebook": idA, "mode": "concise"}, "INVALID_ARGUMENT"},
		{map[string]any{"notebook": idA, "mode": "custom", "response_length": "longer"}, "INVALID_ARGUMENT"}, // custom without prompt: rejected by the domain layer
	} {
		if e := envelope(t, call(t, cs, "chat_configure", tc.args)); e.Code != tc.code {
			t.Fatalf("%v: %+v", tc.args, e)
		}
	}
	if tr.count("s0tc2d") != 1 {
		t.Fatalf("refused changes wrote settings: %d mutate calls", tr.count("s0tc2d"))
	}
	// Reset to the defaults (capture #3b).
	res = call(t, cs, "chat_configure", map[string]any{"notebook": idA, "mode": "default", "response_length": "default"})
	out = ChatConfigureOut{}
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil || !out.Updated || out.Mode != "default" || out.Length != "default" {
		t.Fatalf("reset = %q %v", text(res), err)
	}
}

// The schema offers only the proven settings.
func TestChatConfigureSchemaEnums(t *testing.T) {
	cs := connectChat(t, newChatStub(), nil, nil)
	res, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name != "chat_configure" {
			continue
		}
		b, _ := json.Marshal(tool.InputSchema)
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		if got := enumOf(t, m, "mode"); !slices.Equal(got, []string{"default", "learning_guide", "custom"}) {
			t.Errorf("mode enum = %v", got)
		}
		if got := enumOf(t, m, "response_length"); !slices.Equal(got, []string{"default", "longer", "shorter"}) {
			t.Errorf("response_length enum = %v", got)
		}
		return
	}
	t.Fatal("chat_configure not listed")
}

func TestSuggest(t *testing.T) {
	cs := connectChat(t, newChatStub(), nil, nil)
	res := call(t, cs, "suggest", map[string]any{"notebook": idA, "surface": "chat"})
	var out SuggestOut
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil || len(out.Suggestions) != 1 || out.Suggestions[0].Prompt != "Prompt one" {
		t.Fatalf("suggest = %q %v", text(res), err)
	}
	if e := envelope(t, call(t, cs, "suggest", map[string]any{"notebook": idA, "surface": "followups"})); e.Code != "UNSUPPORTED_BY_TRANSPORT" {
		t.Fatalf("followups: %+v", e)
	}
	if e := envelope(t, call(t, cs, "suggest", map[string]any{"surface": "chat"})); e.Code != "INVALID_ARGUMENT" {
		t.Fatalf("missing notebook: %+v", e)
	}
}

func TestChatToolAnnotations(t *testing.T) {
	cs := connectChat(t, newChatStub(), jobs.New(), nil)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	type want struct{ readOnly, idem bool }
	wants := map[string]want{
		"chat_ask": {false, false}, "chat_status": {true, true}, "chat_history": {true, true},
		"chat_configure": {false, true}, "suggest": {true, true}, "job_cancel": {false, true},
	}
	seen := 0
	for _, tl := range res.Tools {
		w, ok := wants[tl.Name]
		if !ok {
			continue
		}
		seen++
		a := tl.Annotations
		if a == nil || a.ReadOnlyHint != w.readOnly || a.IdempotentHint != w.idem {
			t.Errorf("%s annotations = %+v", tl.Name, a)
		}
		if tl.Name == "job_cancel" && (a.DestructiveHint == nil || !*a.DestructiveHint) {
			t.Errorf("job_cancel must be destructive")
		}
		if !w.readOnly && tl.Name != "job_cancel" && (a.DestructiveHint == nil || *a.DestructiveHint) {
			t.Errorf("%s must be non-destructive", tl.Name)
		}
	}
	if seen != len(wants) {
		t.Fatalf("saw %d of %d tools", seen, len(wants))
	}
}
