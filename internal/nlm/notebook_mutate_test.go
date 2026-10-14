package nlm

import (
	"context"
	"errors"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

const mutNB = "00000000-0000-4000-8000-000000000001"

func TestCreateNotebookLive(t *testing.T) {
	fx := loadFixture(t, "CCqFvf", "live_2026-10-13_01")
	ft := newFake(fx)
	nb, err := New(ft).CreateNotebook(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if nb.ID != mutNB {
		t.Fatalf("id = %q", nb.ID)
	}
	if p := paramsJSON(t, ft, "CCqFvf"); !sameJSON(t, []byte(p), fx.Request) {
		t.Fatalf("params %s\nwant %s", p, fx.Request)
	}
	c, _ := ft.callFor("CCqFvf")
	if c.Idempotent {
		t.Fatal("create must never be retried")
	}
	if len(ft.calls) != 1 {
		t.Fatalf("empty title must not rename: %d calls", len(ft.calls))
	}
}

func TestCreateNotebookWithTitleRenamesAndVerifies(t *testing.T) {
	ft := newFake(
		loadFixture(t, "CCqFvf", "live_2026-10-13_01"),
		inlineFixture("s0tc2d", `[]`),
		inlineFixture("rLM1Ne", `[["My plan",[],"`+mutNB+`","",null,[1]]]`),
	)
	nb, err := New(ft).CreateNotebook(context.Background(), "My plan", "")
	if err != nil {
		t.Fatal(err)
	}
	if nb.ID != mutNB || nb.Title != "My plan" {
		t.Fatalf("nb = %+v", nb)
	}
	want := `["` + mutNB + `",[[null,null,null,[null,"My plan"]]]]`
	if p := paramsJSON(t, ft, "s0tc2d"); !sameJSON(t, []byte(p), []byte(want)) {
		t.Fatalf("rename params %s", p)
	}
	if c, _ := ft.callFor("s0tc2d"); c.Idempotent {
		t.Fatal("rename is a mutation: no retry flag")
	}
}

func TestCreateNotebookRenameFailureKeepsID(t *testing.T) {
	ft := newFake(loadFixture(t, "CCqFvf", "live_2026-10-13_01"))
	ft.errs["s0tc2d"] = &transport.Error{Kind: transport.KindTransport, RPC: "notebooks.mutate"}
	nb, err := New(ft).CreateNotebook(context.Background(), "T", "")
	if err == nil || nb.ID != mutNB {
		t.Fatalf("want id with error, got %+v %v", nb, err)
	}
}

func TestCreateNotebookNoID(t *testing.T) {
	ft := newFake(inlineFixture("CCqFvf", `["",null,null]`))
	_, err := New(ft).CreateNotebook(context.Background(), "", "")
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindInternal {
		t.Fatalf("err = %v", err)
	}
}

func TestRenameNotebookMismatch(t *testing.T) {
	ft := newFake(inlineFixture("s0tc2d", `[]`), inlineFixture("rLM1Ne", `[["Other",[],"`+mutNB+`","",null,[1]]]`))
	err := New(ft).RenameNotebook(context.Background(), mutNB, "Wanted")
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindInternal {
		t.Fatalf("err = %v", err)
	}
}

func TestRenameNotebookInvalid(t *testing.T) {
	for _, tc := range []struct{ id, title string }{{"", "x"}, {mutNB, "  "}} {
		err := New(newFake()).RenameNotebook(context.Background(), tc.id, tc.title)
		var te *transport.Error
		if !errors.As(err, &te) || te.Kind != transport.KindInvalidArgument {
			t.Fatalf("%+v: err = %v", tc, err)
		}
	}
}

func TestRemoveFromRecent(t *testing.T) {
	ft := newFake(inlineFixture("fejl7e", `[]`))
	if err := New(ft).RemoveFromRecent(context.Background(), mutNB); err != nil {
		t.Fatal(err)
	}
	if p := paramsJSON(t, ft, "fejl7e"); p != `["`+mutNB+`"]` {
		t.Fatalf("params %s", p)
	}
}

func TestRemoveFromRecentInternalIsSuccess(t *testing.T) {
	ft := newFake()
	ft.errs["fejl7e"] = &transport.Error{Kind: transport.KindInternal, RPC: "notebooks.unrecent"}
	if err := New(ft).RemoveFromRecent(context.Background(), mutNB); err != nil {
		t.Fatal(err)
	}
	ft.errs["fejl7e"] = &transport.Error{Kind: transport.KindAuth, RPC: "notebooks.unrecent"}
	if err := New(ft).RemoveFromRecent(context.Background(), mutNB); err == nil {
		t.Fatal("auth error must surface")
	}
	if err := New(ft).RemoveFromRecent(context.Background(), ""); err == nil {
		t.Fatal("empty id must fail")
	}
}
