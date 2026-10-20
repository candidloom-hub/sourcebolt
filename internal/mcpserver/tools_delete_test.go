package mcpserver

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/cache"
	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/resolve"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDeleteRejectsFuzzyTitle(t *testing.T) {
	st := newStub()
	st.payloads["WWINqb"] = `[]`
	cs := connect(t, st, "default")
	// "research" is a unique substring of "Shared research": fine for reads,
	// never for a delete. "Q3" is a substring of two titles.
	for ref, want := range map[string]string{"research": "Shared research (" + idC + ")", "Q3": "Q3 plan (" + idA + ")"} {
		for _, confirm := range []bool{false, true} {
			e := envelope(t, call(t, cs, "delete", map[string]any{"kind": "notebook", "ids": []string{ref}, "confirm": confirm}))
			if e.Code != "NOT_FOUND" || !slices.Contains(e.Suggestions, want) {
				t.Fatalf("%q confirm=%v: envelope = %+v", ref, confirm, e)
			}
		}
	}
	if st.count("WWINqb") != 0 {
		t.Fatal("a fuzzy match reached the delete RPC")
	}
	// Exact id and exact title (any case) still resolve.
	for _, ref := range []string{idC, "SHARED RESEARCH"} {
		var out DeleteOut
		res := call(t, cs, "delete", map[string]any{"kind": "notebook", "ids": []string{ref}})
		if err := json.Unmarshal([]byte(text(res)), &out); err != nil || len(out.Items) != 1 || out.Items[0].ID != idC {
			t.Fatalf("%q: %s", ref, text(res))
		}
	}
}

func TestResolveStrict(t *testing.T) {
	items := []resolve.Item{
		{ID: "7f00aa11-0000-4000-8000-000000000001", Title: "Q3 plan"},
		{ID: "0b1c2d3e-0000-4000-8000-000000000002", Title: "Q3 planning notes"},
		{ID: "0b1c2d3e-0000-4000-8000-000000000003", Title: "dup"},
		{ID: "0b1c2d3e-0000-4000-8000-000000000004", Title: "DUP"},
	}
	for ref, want := range map[string]string{
		items[1].ID: items[1].ID, // exact id
		"7f00aa":    items[0].ID, // unique id prefix
		"q3 PLAN":   items[0].ID, // exact title, any case
	} {
		if it, err := resolveStrict(ref, items, true); err != nil || it.ID != want {
			t.Errorf("%q: got %+v, %v", ref, it, err)
		}
	}
	var nf *resolve.ErrNotFound
	if _, err := resolveStrict("planning", items, true); !errors.As(err, &nf) || len(nf.Suggestions) != 1 || nf.Suggestions[0].ID != items[1].ID {
		t.Errorf("substring must be NOT_FOUND with a suggestion: %v", err)
	}
	if _, err := resolveStrict("7f00a", items, true); !errors.As(err, &nf) {
		t.Errorf("short prefix: %v", err)
	}
	if _, err := resolveStrict("7f00aa", items, false); !errors.As(err, &nf) {
		t.Errorf("prefix with idPrefix=false: %v", err)
	}
	var amb *resolve.ErrAmbiguous
	for _, ref := range []string{"dup", "0b1c2d"} {
		if _, err := resolveStrict(ref, items, true); !errors.As(err, &amb) {
			t.Errorf("%q: want ambiguous, got %v", ref, err)
		}
	}
}

func TestDeleteRefreshesNotebookList(t *testing.T) {
	st := newStub()
	cs := connect(t, st, "default")
	call(t, cs, "notebook_list", nil)
	call(t, cs, "delete", map[string]any{"kind": "notebook", "ids": []string{idC}})
	if n := st.count("wXbhsf"); n != 2 {
		t.Fatalf("list calls = %d, want 2 (delete must not resolve against the cache)", n)
	}
}

// failDelete fails WWINqb calls for chosen notebook ids and passes the rest
// to the stub.
type failDelete struct {
	*stub
	fail map[string]bool
}

func (f failDelete) Do(ctx context.Context, c transport.Call) (transport.Result, error) {
	res, err := f.stub.Do(ctx, c)
	if c.ID == "WWINqb" {
		if p, ok := c.Params.([]any); ok && len(p) > 0 {
			if inner, ok := p[0].([]any); ok && len(inner) > 0 && f.fail[fmt.Sprint(inner[0])] {
				return transport.Result{}, &transport.Error{Kind: transport.KindPermission, RPC: "notebooks.delete"}
			}
		}
	}
	return res, err
}

func connectWith(t *testing.T, tr transport.Transport) *mcp.ClientSession {
	t.Helper()
	srv := New(Deps{NLM: nlm.New(tr), Cache: cache.New(nil), Profile: "default", Host: "notebook.google.com"}, "default")
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

func TestDeletePerIDResultsAndDedupe(t *testing.T) {
	st := newStub()
	st.payloads["WWINqb"] = `[]`
	cs := connectWith(t, failDelete{stub: st, fail: map[string]bool{idA: true}})
	res := call(t, cs, "delete", map[string]any{"kind": "notebook", "ids": []string{idA, idC, idC}, "confirm": true})
	var out DeleteOut
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil {
		t.Fatalf("%s: %v", text(res), err)
	}
	if st.count("WWINqb") != 2 {
		t.Fatalf("delete calls = %d, want 2 (dedupe, continue after failure)", st.count("WWINqb"))
	}
	if out.Status != "partial" || len(out.Items) != 2 {
		t.Fatalf("out = %s", text(res))
	}
	got := map[string]DeleteItem{}
	for _, it := range out.Items {
		got[it.ID] = it
	}
	if a := got[idA]; a.Status != "failed" || a.Error == "" || a.Title != "Q3 plan" {
		t.Fatalf("idA = %+v", a)
	}
	if c := got[idC]; c.Status != "deleted" || c.Error != "" {
		t.Fatalf("idC = %+v", c)
	}
}

func TestDeletePreviewMakesNoCall(t *testing.T) {
	st := newStub()
	st.payloads["WWINqb"] = `[]`
	cs := connect(t, st, "default")
	a := toolAnn(t, cs, "delete")
	if a.ReadOnlyHint || a.DestructiveHint == nil || !*a.DestructiveHint {
		t.Errorf("annotations = %+v", a)
	}
	var out DeleteOut
	res := call(t, cs, "delete", map[string]any{"kind": "notebook", "ids": []string{"shared research"}})
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "preview" || len(out.Items) != 1 || out.Items[0].ID != idC || out.Items[0].Title != "Shared research" {
		t.Fatalf("out = %s", text(res))
	}
	if st.count("WWINqb") != 0 {
		t.Fatal("preview made a destructive call")
	}
}

func TestDeleteConfirmInvalidatesCache(t *testing.T) {
	st := newStub()
	st.payloads["WWINqb"] = `[]`
	cs := connect(t, st, "default")
	call(t, cs, "notebook_list", nil)
	res := call(t, cs, "delete", map[string]any{"kind": "notebook", "ids": []string{idC}, "confirm": true})
	if res.IsError || st.count("WWINqb") != 1 {
		t.Fatalf("res = %s, calls %d", text(res), st.count("WWINqb"))
	}
	call(t, cs, "notebook_list", nil)
	// list, delete's fresh resolve list, then list again after invalidation.
	if n := st.count("wXbhsf"); n != 3 {
		t.Fatalf("list calls = %d, want 3 (cache invalidated)", n)
	}
}

func TestDeleteBlockedAndBadInput(t *testing.T) {
	st := newStub()
	st.payloads["cFji9"] = notesPayload
	cs := connect(t, st, "default")
	// Kinds that cannot be deleted yet fail before any preview or lookup.
	for _, args := range []map[string]any{
		{"kind": "artifact", "notebook": idA, "ids": []string{"a1"}},
		{"kind": "artifact", "notebook": idA, "ids": []string{"a1"}, "confirm": true},
		{"kind": "label", "notebook": idA, "ids": []string{"L1"}},
		{"kind": "collection", "ids": []string{"x"}},
	} {
		if e := envelope(t, call(t, cs, "delete", args)); e.Code != "UNSUPPORTED_BY_TRANSPORT" {
			t.Fatalf("%v: envelope = %+v", args, e)
		}
	}
	if n := len(st.calls); n != 0 {
		t.Fatalf("blocked kinds made %d calls", n)
	}
	for _, args := range []map[string]any{
		{"kind": "bogus", "ids": []string{"x"}},
		{"kind": "notebook", "ids": []string{}},
		{"kind": "source", "ids": []string{"s1"}},
		{"kind": "note", "notebook": idA, "ids": []string{}},
		{"kind": "chat_history", "ids": []string{"conv-1"}, "confirm": true},
	} {
		if e := envelope(t, call(t, cs, "delete", args)); e.Code != "INVALID_ARGUMENT" {
			t.Fatalf("%v: envelope = %+v", args, e)
		}
	}
	if n := len(st.calls); n != 0 {
		t.Fatalf("bad input made %d calls", n)
	}
}

// TestDeleteNotebookItems covers the kinds live capture #2 pinned: source,
// note and chat_history. Each previews without confirm and deletes with it,
// resolving refs strictly within the notebook.
func TestDeleteNotebookItems(t *testing.T) {
	for _, tc := range []struct {
		kind, wire, ref, id, title, loose string
	}{
		{"source", "tGMBJ", "DOC", "s1", "Doc", "Do"},
		{"note", "AH0mwd", "first", "n1", "First", "fir"},
		{"chat_history", "J7Gthc", "conv-1", "conv-1", "", "conv"},
	} {
		st := newStub()
		st.payloads["cFji9"] = notesPayload
		st.payloads["hPTbtc"] = `[[["conv-1",[1700000000,0],[1700000001,0]]]]`
		st.payloads[tc.wire] = `[]`
		cs := connect(t, st, "default")

		var out DeleteOut
		res := call(t, cs, "delete", map[string]any{"kind": tc.kind, "notebook": idA, "ids": []string{tc.ref}})
		if err := json.Unmarshal([]byte(text(res)), &out); err != nil {
			t.Fatalf("%s preview: %s", tc.kind, text(res))
		}
		if out.Status != "preview" || len(out.Items) != 1 || out.Items[0].ID != tc.id || out.Items[0].Title != tc.title {
			t.Fatalf("%s preview = %s", tc.kind, text(res))
		}
		if st.count(tc.wire) != 0 {
			t.Fatalf("%s: preview made a destructive call", tc.kind)
		}
		if e := envelope(t, call(t, cs, "delete", map[string]any{"kind": tc.kind, "notebook": idA, "ids": []string{tc.loose}, "confirm": true})); e.Code != "NOT_FOUND" {
			t.Fatalf("%s: loose ref %q: envelope = %+v", tc.kind, tc.loose, e)
		}
		if st.count(tc.wire) != 0 {
			t.Fatalf("%s: a loose ref reached the delete RPC", tc.kind)
		}

		res = call(t, cs, "delete", map[string]any{"kind": tc.kind, "notebook": idA, "ids": []string{tc.ref, tc.id}, "confirm": true})
		out = DeleteOut{}
		if err := json.Unmarshal([]byte(text(res)), &out); err != nil || out.Status != "deleted" || len(out.Items) != 1 || out.Items[0].Status != "deleted" {
			t.Fatalf("%s confirm = %s", tc.kind, text(res))
		}
		if st.count(tc.wire) != 1 {
			t.Fatalf("%s: delete calls = %d, want 1", tc.kind, st.count(tc.wire))
		}
		for _, c := range st.calls {
			if c.ID == tc.wire {
				if b, _ := json.Marshal(c.Params); !strings.Contains(string(b), `"`+tc.id+`"`) || c.SourcePath != "/notebook/"+idA {
					t.Fatalf("%s: call %s path %s", tc.kind, b, c.SourcePath)
				}
			}
		}
	}
}
