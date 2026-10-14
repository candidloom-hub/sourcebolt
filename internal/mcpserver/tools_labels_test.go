package mcpserver

import (
	"encoding/json/v2"
	"testing"
)

const labelsPayload = `[[["Intro",[["s1"]],"L1","📘"]]]`

func TestLabelList(t *testing.T) {
	st := newStub()
	st.payloads["I3xc3c"] = labelsPayload
	cs := connect(t, st, "default")
	if a := toolAnn(t, cs, "label_list"); !a.ReadOnlyHint {
		t.Errorf("annotations = %+v", a)
	}
	var out LabelListOut
	if err := json.Unmarshal([]byte(text(call(t, cs, "label_list", map[string]any{"notebook": idA}))), &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 || out.Labels[0].ID != "L1" || out.Labels[0].SourceIDs[0] != "s1" {
		t.Fatalf("out = %+v", out)
	}
	if e := envelope(t, call(t, cs, "label_list", map[string]any{"notebook": idA, "scope": "collections"})); e.Code != "UNSUPPORTED_BY_TRANSPORT" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestLabelUpdate(t *testing.T) {
	st := newStub()
	st.payloads["I3xc3c"] = labelsPayload
	st.payloads["le8sX"] = `[]`
	cs := connect(t, st, "default")
	if a := toolAnn(t, cs, "label_update"); a.ReadOnlyHint || a.DestructiveHint == nil || *a.DestructiveHint {
		t.Errorf("annotations = %+v", a)
	}
	res := call(t, cs, "label_update", map[string]any{"notebook": idA, "action": "rename", "label": "intro", "name": "Start"})
	if res.IsError || st.count("le8sX") != 1 {
		t.Fatalf("rename: %s, calls %d", text(res), st.count("le8sX"))
	}
	res = call(t, cs, "label_update", map[string]any{"notebook": idA, "action": "add_sources", "label": "L1", "items": []string{"Doc"}})
	if res.IsError || st.count("le8sX") != 2 {
		t.Fatalf("add: %s, calls %d", text(res), st.count("le8sX"))
	}
}

func TestLabelUpdateErrors(t *testing.T) {
	cs := connect(t, newStub(), "default")
	if e := envelope(t, call(t, cs, "label_update", map[string]any{"notebook": idA, "action": "auto", "label": "x"})); e.Code != "UNSUPPORTED_BY_TRANSPORT" {
		t.Fatalf("envelope = %+v", e)
	}
	if e := envelope(t, call(t, cs, "label_update", map[string]any{"notebook": idA, "action": "rename", "label": "x"})); e.Code != "INVALID_ARGUMENT" {
		t.Fatalf("envelope = %+v", e)
	}
}
