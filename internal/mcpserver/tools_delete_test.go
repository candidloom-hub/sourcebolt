package mcpserver

import (
	"encoding/json/v2"
	"testing"
)

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
