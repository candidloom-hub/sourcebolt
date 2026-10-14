package nlm

import (
	"encoding/json/v2"
	"errors"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

// TestLiveStudioOptionsShape pins the sqTeoe builder to the live capture.
func TestLiveStudioOptionsShape(t *testing.T) {
	fx := loadFixture(t, "sqTeoe", "live_2026-10-13_01")
	if fx.Synthetic {
		t.Fatal("must be a live capture")
	}
	got, _ := json.Marshal(studioChoicesParams())
	if !sameJSON(t, got, fx.Request) {
		t.Fatalf("got %s want %s", got, fx.Request)
	}
}

func TestSuggestStudioOptionsLive(t *testing.T) {
	ft := newFake(loadFixture(t, "sqTeoe", "live_2026-10-13_01"))
	got, err := New(ft).Suggest(t.Context(), fakeNB, "studio_options", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4+4+2+3 {
		t.Fatalf("len = %d: %+v", len(got), got)
	}
	// row [1,"text-15","text-16"]: code, then title, then description
	if got[0].Title != "text-15" || got[0].Description != "text-16" || got[0].Code != 1 || got[0].Group != 0 {
		t.Fatalf("first = %+v", got[0])
	}
	last := got[len(got)-1]
	if last.Group != 3 || last.Title != "text-39" || last.Description != "text-40" {
		t.Fatalf("last = %+v", last)
	}
	c, _ := ft.callFor("sqTeoe")
	if c.RPC != "studio.choices" || !c.Idempotent {
		t.Fatalf("call = %+v", c)
	}
}

func TestSuggestChat(t *testing.T) {
	ft := newFake(loadFixture(t, "otmP3b", "suggestions"))
	got, err := New(ft).Suggest(t.Context(), fakeNB, "chat", []string{"src-0001"}, "text-query")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Title != "text-title-1" || got[1].Prompt != "text-prompt-2" {
		t.Fatalf("got %+v", got)
	}
	c, _ := ft.callFor("otmP3b")
	p, _ := json.Marshal(c.Params)
	if !sameJSON(t, p, loadFixture(t, "otmP3b", "suggestions").Request) {
		t.Fatalf("params %s", p)
	}
}

func TestSuggestChatNoQueryNoSources(t *testing.T) {
	ft := newFake(loadFixture(t, "otmP3b", "suggestions"))
	if _, err := New(ft).Suggest(t.Context(), fakeNB, "chat", nil, ""); err != nil {
		t.Fatal(err)
	}
	c, _ := ft.callFor("otmP3b")
	p, _ := json.Marshal(c.Params)
	want := `[[2,null,[1],[1,null,null,null,null,null,null,null,null,null,[1,3]]],"` + fakeNB + `",[],4,null,null]`
	if !sameJSON(t, p, []byte(want)) {
		t.Fatalf("params %s", p)
	}
}

func TestSuggestReport(t *testing.T) {
	ft := newFake(loadFixture(t, "ciyUvf", "suggestions"))
	got, err := New(ft).Suggest(t.Context(), fakeNB, "report", []string{"src-0001"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Title != "text-title-1" || got[0].Description != "text-desc-1" ||
		got[0].Prompt != "text-prompt-1" || got[0].Audience != 1 || got[1].Audience != 2 {
		t.Fatalf("got %+v", got)
	}
	c, _ := ft.callFor("ciyUvf")
	p, _ := json.Marshal(c.Params)
	if !sameJSON(t, p, loadFixture(t, "ciyUvf", "suggestions").Request) {
		t.Fatalf("params %s", p)
	}
	// no sources: the optional third element is dropped
	ft2 := newFake(loadFixture(t, "ciyUvf", "suggestions"))
	if _, err := New(ft2).Suggest(t.Context(), fakeNB, "report", nil, ""); err != nil {
		t.Fatal(err)
	}
	c2, _ := ft2.callFor("ciyUvf")
	p2, _ := json.Marshal(c2.Params)
	if !sameJSON(t, p2, []byte(`[[2],"`+fakeNB+`"]`)) {
		t.Fatalf("params %s", p2)
	}
}

func TestSuggestRejectsSurfaces(t *testing.T) {
	for _, s := range []string{"followups", "bogus", ""} {
		ft := newFake()
		_, err := New(ft).Suggest(t.Context(), fakeNB, s, nil, "")
		var te *transport.Error
		if !errors.As(err, &te) {
			t.Fatalf("%q: err = %v", s, err)
		}
		if s == "followups" && te.Kind != transport.KindUnsupported || s != "followups" && te.Kind != transport.KindInvalidArgument {
			t.Errorf("%q: kind = %v", s, te.Kind)
		}
		if len(ft.calls) != 0 {
			t.Errorf("%q: made upstream calls", s)
		}
	}
}
