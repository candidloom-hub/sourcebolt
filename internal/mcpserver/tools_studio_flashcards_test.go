package mcpserver

import (
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
)

func TestStudioGenerateFlashcards(t *testing.T) {
	st := newStub()
	st.payloads["EylDcb"] = quotaPayload
	st.payloads["R7cb6c"] = `[["art-9","Q3 plan",4,[],2,null,null,null,null,[null,[1]]]]`
	cs := connect(t, st, "default")
	args := map[string]any{"notebook": idA, "kind": "flashcards", "sources": []string{"Doc"}, "instructions": "topic one",
		"flashcards": map[string]any{"quantity": "fewer", "difficulty": "hard"}}
	var a nlm.Artifact
	res := call(t, cs, "studio_generate", args)
	if err := json.Unmarshal([]byte(text(res)), &a); err != nil || a.ID != "art-9" || a.Kind != nlm.StudioFlashcards || a.Status != nlm.ArtifactPending {
		t.Fatalf("out = %s", text(res))
	}
	if st.count("EylDcb") != 1 || st.count("R7cb6c") != 1 {
		t.Fatalf("calls: quota %d, create %d", st.count("EylDcb"), st.count("R7cb6c"))
	}
	for _, c := range st.calls {
		if c.ID == "R7cb6c" {
			b, _ := json.Marshal(c.Params)
			for _, want := range []string{`"topic one"`, `[1,3,[1]]`, `[[["s1"]]]`, `"en"`} {
				if !strings.Contains(string(b), want) {
					t.Fatalf("params %s lack %s", b, want)
				}
			}
		}
	}
}

func TestStudioGenerateFlashcardsLimits(t *testing.T) {
	st := newStub()
	st.payloads["EylDcb"] = quotaPayload
	st.payloads["R7cb6c"] = `[["art-9","Q3 plan",4,[],2]]`
	cs := connect(t, st, "default")
	for _, tc := range []struct {
		args map[string]any
		code string
	}{
		{map[string]any{"notebook": idA, "kind": "flashcards", "instructions": "t", "flashcards": map[string]any{"quantity": "standard"}}, "UNSUPPORTED_BY_TRANSPORT"},
		{map[string]any{"notebook": idA, "kind": "flashcards", "instructions": "t", "flashcards": map[string]any{"difficulty": "medium"}}, "UNSUPPORTED_BY_TRANSPORT"},
		{map[string]any{"notebook": idA, "kind": "flashcards"}, "UNSUPPORTED_BY_TRANSPORT"},
		{map[string]any{"notebook": idA, "kind": "flashcards", "instructions": "t", "flashcards": map[string]any{"quantity": "heaps"}}, "INVALID_ARGUMENT"},
		{map[string]any{"notebook": idA, "kind": "quiz", "instructions": "t", "flashcards": map[string]any{"quantity": "fewer"}}, "INVALID_ARGUMENT"},
	} {
		if e := envelope(t, call(t, cs, "studio_generate", tc.args)); e.Code != tc.code {
			t.Fatalf("%v: envelope = %+v", tc.args, e)
		}
	}
	if n := st.count("R7cb6c"); n != 0 {
		t.Fatalf("refused requests made %d create calls", n)
	}
}
