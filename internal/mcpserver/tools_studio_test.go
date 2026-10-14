package mcpserver

import (
	"encoding/json/v2"
	"testing"
)

const artifactsPayload = `[[["a1","Overview",1,null,3],["a2","Brief",2,null,4]]]`

func TestStudioListFilterAndContent(t *testing.T) {
	st := newStub()
	st.payloads["gArtLc"] = artifactsPayload
	st.payloads["v9rmvd"] = `[[null,null,null,null,null,null,null,null,null,["<p>hi</p>"]]]`
	cs := connect(t, st, "default")
	if a := toolAnn(t, cs, "studio_list"); !a.ReadOnlyHint {
		t.Errorf("annotations = %+v", a)
	}
	var out StudioListOut
	if err := json.Unmarshal([]byte(text(call(t, cs, "studio_list", map[string]any{"notebook": idA, "status": "ready"}))), &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 || out.Artifacts[0].ID != "a1" {
		t.Fatalf("out = %+v", out)
	}
	out = StudioListOut{}
	if err := json.Unmarshal([]byte(text(call(t, cs, "studio_list", map[string]any{"notebook": idA, "artifact": "overview"}))), &out); err != nil {
		t.Fatal(err)
	}
	if out.Content == nil || out.Content.HTML != "<p>hi</p>" {
		t.Fatalf("content = %+v", out)
	}
	if e := envelope(t, call(t, cs, "studio_list", map[string]any{"notebook": idA, "status": "weird"})); e.Code != "INVALID_ARGUMENT" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestStudioGenerateQuotaExhausted(t *testing.T) {
	st := newStub()
	st.payloads["EylDcb"] = `[1,[[null,null,null,null,1,[100,0],100,0]]]`
	cs := connect(t, st, "default")
	if a := toolAnn(t, cs, "studio_generate"); a.ReadOnlyHint || a.DestructiveHint == nil || *a.DestructiveHint {
		t.Errorf("annotations = %+v", a)
	}
	e := envelope(t, call(t, cs, "studio_generate", map[string]any{"notebook": idA, "kind": "audio"}))
	if e.Code != "QUOTA" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestStudioGenerateUnsupportedAndBadKind(t *testing.T) {
	st := newStub()
	st.payloads["EylDcb"] = quotaPayload
	cs := connect(t, st, "default")
	e := envelope(t, call(t, cs, "studio_generate", map[string]any{"notebook": idA, "kind": "audio"}))
	if e.Code != "UNSUPPORTED_BY_TRANSPORT" {
		t.Fatalf("envelope = %+v", e)
	}
	if e := envelope(t, call(t, cs, "studio_generate", map[string]any{"notebook": idA, "kind": "poem"})); e.Code != "INVALID_ARGUMENT" {
		t.Fatalf("envelope = %+v", e)
	}
}
