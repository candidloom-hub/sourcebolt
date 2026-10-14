package mcpserver

import (
	"encoding/json/v2"
	"testing"
)

const sharePayload = `[[["a@x.com",2,[],["Ann","u"]]],[true]]`

func TestShareStatus(t *testing.T) {
	st := newStub()
	st.payloads["JFMDGd"] = sharePayload
	cs := connect(t, st, "default")
	if a := toolAnn(t, cs, "share_status"); !a.ReadOnlyHint {
		t.Errorf("annotations = %+v", a)
	}
	res := call(t, cs, "share_status", map[string]any{"notebook": idA})
	var out struct {
		Public bool `json:"public"`
		Users  []struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		} `json:"users"`
	}
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Public || len(out.Users) != 1 || out.Users[0].Role != "editor" {
		t.Fatalf("out = %s", text(res))
	}
}

func TestShareUpdateRequiresConfirm(t *testing.T) {
	st := newStub()
	st.payloads["QDyure"] = `[]`
	cs := connect(t, st, "default")
	a := toolAnn(t, cs, "share_update")
	if a.ReadOnlyHint || a.DestructiveHint == nil || !*a.DestructiveHint || a.OpenWorldHint == nil || !*a.OpenWorldHint {
		t.Errorf("annotations = %+v", a)
	}
	for _, args := range []map[string]any{
		{"notebook": idA, "public": true},
		{"notebook": idA, "users": []map[string]any{{"email": "b@x.com", "role": "viewer"}}},
		{"notebook": idA, "users": []map[string]any{{"email": "b@x.com", "role": "remove"}}},
	} {
		var out ShareUpdateOut
		res := call(t, cs, "share_update", args)
		if err := json.Unmarshal([]byte(text(res)), &out); err != nil {
			t.Fatal(err)
		}
		if out.Status != "preview" || !out.RequiresConfirm || len(out.Changes) == 0 {
			t.Fatalf("out = %s", text(res))
		}
	}
	if n := st.count("QDyure"); n != 0 {
		t.Fatalf("preview made %d share calls", n)
	}
	res := call(t, cs, "share_update", map[string]any{"notebook": idA, "public": true, "confirm": true})
	if res.IsError || st.count("QDyure") != 1 {
		t.Fatalf("confirmed update: %s, calls %d", text(res), st.count("QDyure"))
	}
}

func TestShareUpdateNamesNotebook(t *testing.T) {
	st := newStub()
	st.payloads["QDyure"] = `[]`
	cs := connect(t, st, "default")
	for _, confirm := range []bool{false, true} {
		var out ShareUpdateOut
		res := call(t, cs, "share_update", map[string]any{"notebook": "shared research", "public": true, "confirm": confirm})
		if err := json.Unmarshal([]byte(text(res)), &out); err != nil {
			t.Fatal(err)
		}
		if out.NotebookID != idC || out.NotebookTitle != "Shared research" {
			t.Fatalf("confirm=%v: out = %s", confirm, text(res))
		}
	}
}

func TestShareUpdateRejectsFuzzyTitle(t *testing.T) {
	st := newStub()
	st.payloads["QDyure"] = `[]`
	cs := connect(t, st, "default")
	for _, args := range []map[string]any{
		{"notebook": "research", "public": true, "confirm": true},
		{"notebook": "research", "public": false},
		{"notebook": "Q3", "users": []map[string]any{{"email": "b@x.com", "role": "remove"}}, "confirm": true},
	} {
		if e := envelope(t, call(t, cs, "share_update", args)); e.Code != "NOT_FOUND" || len(e.Suggestions) == 0 {
			t.Fatalf("%v: envelope = %+v", args, e)
		}
	}
	if n := st.count("QDyure"); n != 0 {
		t.Fatalf("fuzzy match made %d share calls", n)
	}
	call(t, cs, "notebook_list", nil)
	before := st.count("wXbhsf")
	if res := call(t, cs, "share_update", map[string]any{"notebook": "shared research", "public": false}); res.IsError {
		t.Fatalf("exact title: %s", text(res))
	}
	if st.count("wXbhsf") != before+1 {
		t.Fatal("share_update must refresh the notebook list before resolving")
	}
}

func TestShareUpdateNarrowingNeedsNoConfirm(t *testing.T) {
	st := newStub()
	st.payloads["QDyure"] = `[]`
	cs := connect(t, st, "default")
	res := call(t, cs, "share_update", map[string]any{"notebook": idA, "public": false})
	if res.IsError || st.count("QDyure") != 1 {
		t.Fatalf("res = %s, calls %d", text(res), st.count("QDyure"))
	}
}

func TestShareUpdateBadInput(t *testing.T) {
	cs := connect(t, newStub(), "default")
	for _, args := range []map[string]any{
		{"notebook": idA},
		{"notebook": idA, "users": []map[string]any{{"email": "b@x.com", "role": "boss"}}},
	} {
		if e := envelope(t, call(t, cs, "share_update", args)); e.Code != "INVALID_ARGUMENT" {
			t.Fatalf("envelope = %+v", e)
		}
	}
	e := envelope(t, call(t, cs, "share_update", map[string]any{"notebook": idA, "public": false, "view_level": "chat_only"}))
	if e.Code != "UNSUPPORTED_BY_TRANSPORT" {
		t.Fatalf("envelope = %+v", e)
	}
}
