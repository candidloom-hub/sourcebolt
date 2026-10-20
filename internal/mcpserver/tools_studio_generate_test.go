package mcpserver

import (
	"encoding/json/v2"
	"slices"
	"strings"
	"testing"
)

// studioSchema returns studio_generate's input schema as a generic map.
func studioSchema(t *testing.T) map[string]any {
	t.Helper()
	cs := connect(t, newStub(), "default")
	res, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name == "studio_generate" {
			b, _ := json.Marshal(tool.InputSchema)
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			return m
		}
	}
	t.Fatal("studio_generate not listed")
	return nil
}

func enumOf(t *testing.T, schema map[string]any, path ...string) []string {
	t.Helper()
	cur := schema
	for _, p := range path {
		props, _ := cur["properties"].(map[string]any)
		next, ok := props[p].(map[string]any)
		if !ok {
			t.Fatalf("no property %v in schema", path)
		}
		cur = next
	}
	var out []string
	en, _ := cur["enum"].([]any)
	for _, e := range en {
		out = append(out, e.(string))
	}
	return out
}

// The schema offers only the option values a live capture has sent.
func TestStudioGenerateSchemaEnums(t *testing.T) {
	s := studioSchema(t)
	for _, tc := range []struct {
		path []string
		want []string
	}{
		{[]string{"kind"}, []string{"audio", "data_table", "flashcards", "infographic", "interactive_report", "mind_map", "quiz", "report", "slide_deck", "video"}},
		{[]string{"audio", "format"}, []string{"deep_dive", "critique"}},
		{[]string{"audio", "length"}, []string{"short", "default"}},
		{[]string{"video", "format"}, []string{"cinematic", "short"}},
		{[]string{"slide_deck", "format"}, []string{"detailed_deck", "presenter_slides"}},
		{[]string{"slide_deck", "length"}, []string{"default", "short"}},
		{[]string{"quiz", "quantity"}, []string{"standard", "more"}},
		{[]string{"quiz", "difficulty"}, []string{"easy", "medium"}},
		{[]string{"flashcards", "quantity"}, []string{"fewer", "standard"}},
		{[]string{"flashcards", "difficulty"}, []string{"medium", "hard"}},
		{[]string{"flashcards", "images"}, []string{"include", "text_only"}},
		{[]string{"infographic", "orientation"}, []string{"landscape", "portrait"}},
		{[]string{"infographic", "detail"}, []string{"concise", "standard"}},
		{[]string{"infographic", "style"}, []string{"auto", "sketch_note"}},
	} {
		if got := enumOf(t, s, tc.path...); !slices.Equal(got, tc.want) {
			t.Errorf("%v enum = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestStudioGenerateEveryKind(t *testing.T) {
	for _, tc := range []struct {
		args map[string]any
		want string // fragment of the R7cb6c params
	}{
		{map[string]any{"kind": "audio", "instructions": "focus", "audio": map[string]any{"format": "critique", "length": "short"}}, `[null,["focus",1,null,[["s1"]],"en",null,3]]`},
		{map[string]any{"kind": "video", "instructions": "focus", "language": "en_in", "video": map[string]any{"format": "short"}}, `[null,null,[[["s1"]],"en_in","focus",null,4]]`},
		{map[string]any{"kind": "video"}, `[null,null,[[["s1"]],null,null,null,3]]]]`},
		{map[string]any{"kind": "video", "video": map[string]any{"generate_later": true}}, `[null,null,[[["s1"]],null,null,null,3]]],null,null,null,[1]]`},
		{map[string]any{"kind": "audio"}, `[null,[null,2,null,[["s1"]],"en",null,1]]`},
		{map[string]any{"kind": "interactive_report"}, `[null,[null,"en"]]`},
		{map[string]any{"kind": "report", "instructions": "p", "report": map[string]any{"title": "T", "description": "D"}}, `[null,["T","D",null,[["s1"]],"en","p"]]`},
		{map[string]any{"kind": "slide_deck", "instructions": "d", "slide_deck": map[string]any{"format": "presenter_slides"}}, `[["d","en",2,3]]`},
		{map[string]any{"kind": "mind_map", "instructions": "m"}, `[null,[4,null,"m","en"]]`},
		{map[string]any{"kind": "quiz", "instructions": "q", "quiz": map[string]any{"quantity": "more", "difficulty": "easy"}}, `[null,[2,null,"q","en",null,null,null,[3,1]]]`},
		{map[string]any{"kind": "flashcards", "instructions": "f"}, `[null,[1,null,"f","en",null,null,[2,2,[]]]]`},
		{map[string]any{"kind": "infographic", "infographic": map[string]any{"orientation": "portrait"}}, `[[null,"en",null,2,2,1]]`},
		{map[string]any{"kind": "data_table", "instructions": "d"}, `[null,["d","en"]]`},
	} {
		st := newStub()
		st.payloads["EylDcb"] = quotaPayload
		st.payloads["R7cb6c"] = `[["art-1","t",1,[],2]]`
		cs := connect(t, st, "default")
		args := map[string]any{"notebook": idA, "sources": []string{"Doc"}}
		for k, v := range tc.args {
			args[k] = v
		}
		res := call(t, cs, "studio_generate", args)
		if res.IsError {
			t.Fatalf("%v: %s", tc.args["kind"], text(res))
		}
		var got string
		for _, c := range st.calls {
			if c.ID == "R7cb6c" {
				b, _ := json.Marshal(c.Params)
				got = string(b)
			}
		}
		if !strings.Contains(got, tc.want) {
			t.Fatalf("%v: params %s lack %s", tc.args["kind"], got, tc.want)
		}
	}
}

// Unproven values are refused before the quota check and before any
// create call; options of another kind are invalid input.
func TestStudioGenerateRefusesBeforeQuota(t *testing.T) {
	for _, tc := range []struct {
		args map[string]any
		code string
		says string
	}{
		{map[string]any{"kind": "audio", "instructions": "f", "audio": map[string]any{"format": "brief"}}, "UNSUPPORTED_BY_TRANSPORT", "format"},
		{map[string]any{"kind": "slide_deck", "instructions": "f", "slide_deck": map[string]any{"length": "long"}}, "INVALID_ARGUMENT", "length"},
		{map[string]any{"kind": "infographic", "infographic": map[string]any{"orientation": "square"}}, "UNSUPPORTED_BY_TRANSPORT", "orientation"},
		{map[string]any{"kind": "report", "instructions": "p"}, "UNSUPPORTED_BY_TRANSPORT", "title"},
		// One unproven value per mapped field, so each field reaches nlm.
		{map[string]any{"kind": "audio", "instructions": "f", "audio": map[string]any{"length": "long"}}, "UNSUPPORTED_BY_TRANSPORT", "length"},
		{map[string]any{"kind": "video", "instructions": "f", "video": map[string]any{"format": "explainer"}}, "UNSUPPORTED_BY_TRANSPORT", "format"},
		{map[string]any{"kind": "report", "instructions": "p", "report": map[string]any{"title": "T"}}, "UNSUPPORTED_BY_TRANSPORT", "description"},
		{map[string]any{"kind": "report", "instructions": "p", "report": map[string]any{"description": "D"}}, "UNSUPPORTED_BY_TRANSPORT", "title"},
		{map[string]any{"kind": "slide_deck", "instructions": "f", "slide_deck": map[string]any{"format": "outline"}}, "INVALID_ARGUMENT", "format"},
		{map[string]any{"kind": "quiz", "instructions": "q", "quiz": map[string]any{"quantity": "fewer"}}, "UNSUPPORTED_BY_TRANSPORT", "quantity"},
		{map[string]any{"kind": "quiz", "instructions": "q", "quiz": map[string]any{"difficulty": "hard"}}, "UNSUPPORTED_BY_TRANSPORT", "difficulty"},
		{map[string]any{"kind": "flashcards", "instructions": "q", "flashcards": map[string]any{"quantity": "more"}}, "UNSUPPORTED_BY_TRANSPORT", "quantity"},
		{map[string]any{"kind": "flashcards", "instructions": "q", "flashcards": map[string]any{"difficulty": "easy"}}, "UNSUPPORTED_BY_TRANSPORT", "difficulty"},
		{map[string]any{"kind": "flashcards", "instructions": "q", "flashcards": map[string]any{"images": "some"}}, "INVALID_ARGUMENT", "images"},
		{map[string]any{"kind": "infographic", "infographic": map[string]any{"detail": "detailed"}}, "UNSUPPORTED_BY_TRANSPORT", "detail"},
		{map[string]any{"kind": "infographic", "infographic": map[string]any{"style": "watercolor"}}, "INVALID_ARGUMENT", "style"},
		{map[string]any{"kind": "quiz", "instructions": "q", "audio": map[string]any{"format": "critique"}}, "INVALID_ARGUMENT", "audio"},
		{map[string]any{"kind": "interactive_report", "instructions": "q"}, "UNSUPPORTED_BY_TRANSPORT", "instructions"},
	} {
		st := newStub()
		st.payloads["EylDcb"] = quotaPayload
		cs := connect(t, st, "default")
		args := map[string]any{"notebook": idA}
		for k, v := range tc.args {
			args[k] = v
		}
		e := envelope(t, call(t, cs, "studio_generate", args))
		if e.Code != tc.code || !strings.Contains(e.Error, tc.says) {
			t.Errorf("%v: envelope = %+v", tc.args, e)
		}
		if st.count("EylDcb") != 0 || st.count("R7cb6c") != 0 {
			t.Errorf("%v: quota %d, create %d calls", tc.args, st.count("EylDcb"), st.count("R7cb6c"))
		}
	}
}
