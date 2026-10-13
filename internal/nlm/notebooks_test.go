package nlm

import (
	"context"
	"encoding/json/v2"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

const nb1 = "0b1c2d3e-0000-4000-8000-000000000001"

func TestListNotebooks(t *testing.T) {
	ft := newFake(loadFixture(t, "wXbhsf", "list_mixed"))
	got, err := New(ft).ListNotebooks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	a := got[0]
	if a.ID != nb1 || a.Title != "Q3 plan 📈" || a.Emoji != "📈" || a.SourceCount != 2 || !a.Owned {
		t.Fatalf("first = %+v", a)
	}
	if !a.Modified.Equal(time.Unix(1760000000, 500000000)) || !a.Created.Equal(time.Unix(1750000000, 0)) {
		t.Fatalf("times = %v %v", a.Modified, a.Created)
	}
	if got[1].Owned || got[1].SourceCount != 0 {
		t.Fatalf("second = %+v", got[1])
	}
	c, _ := ft.callFor("wXbhsf")
	if c.RPC != "notebooks.list" || c.SourcePath != "/" || !c.Idempotent {
		t.Fatalf("call = %+v", c)
	}
}

func TestGetNotebookRequest(t *testing.T) {
	fx := loadFixture(t, "rLM1Ne", "get_with_sources")
	ft := newFake(fx, loadFixture(t, "VfAZjd", "guide"))
	d, err := New(ft).GetNotebook(context.Background(), nb1)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := ft.callFor("rLM1Ne")
	params, _ := json.Marshal(c.Params)
	if c.SourcePath != "/notebook/"+nb1 || !sameJSON(t, params, fx.Request) {
		t.Fatalf("call = %+v params=%s", c, params)
	}
	if len(d.Sources) != 2 || d.Sources[0].ID != "src-0001" || d.Sources[0].Title != "Roadmap.pdf" || d.Sources[0].Status != 2 {
		t.Fatalf("sources = %+v", d.Sources)
	}
	if d.Summary != "This notebook covers the Q3 roadmap." || len(d.Suggested) != 2 || d.Suggested[0] != "What ships first?" {
		t.Fatalf("guide = %q %v", d.Summary, d.Suggested)
	}
	if d.Title != "Q3 plan 📈" {
		t.Fatalf("title = %q", d.Title)
	}
}

func TestGetNotebookEmpty(t *testing.T) {
	ft := newFake(loadFixture(t, "rLM1Ne", "get_empty"), loadFixture(t, "VfAZjd", "guide"))
	d, err := New(ft).GetNotebook(context.Background(), "0b1c2d3e-0000-4000-8000-000000000002")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Sources) != 0 {
		t.Fatalf("sources = %+v", d.Sources)
	}
}

func TestGuideDegrades(t *testing.T) {
	ft := newFake(loadFixture(t, "rLM1Ne", "get_with_sources"))
	ft.errs["VfAZjd"] = &transport.Error{Kind: transport.KindTransport, RPC: "notebooks.guide"}
	d, err := New(ft).GetNotebook(context.Background(), nb1)
	if err != nil {
		t.Fatalf("guide failure must not fail the read: %v", err)
	}
	if d.Summary != "" || len(d.Sources) != 2 || len(d.Warnings) != 1 {
		t.Fatalf("detail = %+v", d)
	}
}

func TestGetNotebookFailsOnMainRPC(t *testing.T) {
	ft := newFake(loadFixture(t, "VfAZjd", "guide"))
	ft.errs["rLM1Ne"] = &transport.Error{Kind: transport.KindNotFound, RPC: "notebooks.get"}
	_, err := New(ft).GetNotebook(context.Background(), nb1)
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestLossless(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "fixtures", "*", "*.json"))
	if len(files) == 0 {
		t.Fatal("no fixtures")
	}
	for _, p := range files {
		rpc, name := filepath.Base(filepath.Dir(p)), filepath.Base(p)
		fx := loadFixture(t, rpc, name[:len(name)-len(".json")])
		frames, _, err := wire.Decode([]byte(fx.ResponseBody))
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		payload, err := wire.Pick(frames, fx.RPC, wire.NullAllowed)
		if err != nil || payload == nil {
			continue
		}
		var v any
		if err := json.Unmarshal(payload, &v); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		re, _ := json.Marshal(v)
		if !sameJSON(t, re, payload) {
			t.Fatalf("%s: re-encoded payload differs", p)
		}
	}
}
