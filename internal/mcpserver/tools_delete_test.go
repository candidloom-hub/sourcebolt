package mcpserver

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/cache"
	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
	if n := st.count("wXbhsf"); n != 2 {
		t.Fatalf("list calls = %d, want 2 (cache invalidated)", n)
	}
}

func TestDeleteBlockedAndBadInput(t *testing.T) {
	st := newStub()
	st.payloads["cFji9"] = notesPayload
	cs := connect(t, st, "default")
	// A note preview works; the confirmed call surfaces UNSUPPORTED.
	var out DeleteOut
	res := call(t, cs, "delete", map[string]any{"kind": "note", "notebook": idA, "ids": []string{"first"}})
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil || out.Status != "preview" || out.Items[0].ID != "n1" {
		t.Fatalf("preview = %s (%v)", text(res), err)
	}
	e := envelope(t, call(t, cs, "delete", map[string]any{"kind": "note", "notebook": idA, "ids": []string{"n1"}, "confirm": true}))
	if e.Code != "UNSUPPORTED_BY_TRANSPORT" {
		t.Fatalf("envelope = %+v", e)
	}
	for _, args := range []map[string]any{
		{"kind": "label", "notebook": idA, "ids": []string{"L1"}},
		{"kind": "collection", "ids": []string{"x"}},
		{"kind": "chat_history", "notebook": idA, "ids": []string{"x"}},
	} {
		if e := envelope(t, call(t, cs, "delete", args)); e.Code != "UNSUPPORTED_BY_TRANSPORT" {
			t.Fatalf("%v: envelope = %+v", args, e)
		}
	}
	for _, args := range []map[string]any{
		{"kind": "bogus", "ids": []string{"x"}},
		{"kind": "notebook", "ids": []string{}},
		{"kind": "source", "ids": []string{"x"}},
	} {
		if e := envelope(t, call(t, cs, "delete", args)); e.Code != "INVALID_ARGUMENT" {
			t.Fatalf("%v: envelope = %+v", args, e)
		}
	}
}
