package mcpserver

import (
	"context"
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const notesPayload = `[[["n1",["n1","body one",null,null,"First"]],["n2",null,2]]]`

func authErr(rpc string) error {
	return &transport.Error{Kind: transport.KindAuth, RPC: rpc, Transport: "rpc"}
}

// toolAnn returns the annotations of one listed tool.
func toolAnn(t *testing.T, cs *mcp.ClientSession, name string) *mcp.ToolAnnotations {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tl := range res.Tools {
		if tl.Name == name {
			if tl.Annotations == nil {
				t.Fatalf("%s has no annotations", name)
			}
			return tl.Annotations
		}
	}
	t.Fatalf("tool %s not registered", name)
	return nil
}

func TestNoteListFiltersTombstones(t *testing.T) {
	st := newStub()
	st.payloads["cFji9"] = notesPayload
	cs := connect(t, st, "default")
	if a := toolAnn(t, cs, "note_list"); !a.ReadOnlyHint {
		t.Errorf("note_list annotations = %+v", a)
	}
	var out NoteListOut
	res := call(t, cs, "note_list", map[string]any{"notebook": idA})
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 || out.Notes[0].Title != "First" {
		t.Fatalf("out = %+v", out)
	}
}

func TestNoteListAuthError(t *testing.T) {
	st := newStub()
	st.errs["cFji9"] = authErr("notes.list")
	e := envelope(t, call(t, connect(t, st, "default"), "note_list", map[string]any{"notebook": idA}))
	if e.Code != "AUTH_REQUIRED" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestNoteSaveCreateAndUpdate(t *testing.T) {
	st := newStub()
	st.payloads["cFji9"] = notesPayload
	st.payloads["CYK0Xb"] = `[["new1"]]`
	st.payloads["cYAfTb"] = `[]`
	cs := connect(t, st, "default")
	if a := toolAnn(t, cs, "note_save"); a.ReadOnlyHint || a.DestructiveHint == nil || *a.DestructiveHint {
		t.Errorf("note_save annotations = %+v", a)
	}
	res := call(t, cs, "note_save", map[string]any{"notebook": idA, "title": "T", "content": "C"})
	if res.IsError || !strings.Contains(text(res), "new1") {
		t.Fatalf("create = %s", text(res))
	}
	if st.count("CYK0Xb") != 1 || st.count("cYAfTb") != 1 {
		t.Fatalf("create calls: %d/%d", st.count("CYK0Xb"), st.count("cYAfTb"))
	}
	res = call(t, cs, "note_save", map[string]any{"notebook": idA, "note": "first", "content": "edited"})
	if res.IsError || !strings.Contains(text(res), `"n1"`) || st.count("CYK0Xb") != 1 {
		t.Fatalf("update = %s", text(res))
	}
}

func TestNoteSaveFromTurnUnsupported(t *testing.T) {
	cs := connect(t, newStub(), "default")
	e := envelope(t, call(t, cs, "note_save", map[string]any{"notebook": idA, "from_turn": "c1:2"}))
	if e.Code != "UNSUPPORTED_BY_TRANSPORT" {
		t.Fatalf("envelope = %+v", e)
	}
	e = envelope(t, call(t, cs, "note_save", map[string]any{"notebook": idA}))
	if e.Code != "INVALID_ARGUMENT" {
		t.Fatalf("envelope = %+v", e)
	}
}
