package nlm

import (
	"context"
	"encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

func TestListNotesLive(t *testing.T) {
	live := loadFixture(t, "cFji9", "live_2026-10-13_01")
	if got, _ := json.Marshal(listNotesParams(fakeNB)); live.Synthetic || !sameJSON(t, got, live.Request) {
		t.Fatalf("builder %s want %s", got, live.Request)
	}
	ns, err := New(newFake(live)).ListNotes(context.Background(), fakeNB)
	if err != nil || len(ns) != 0 {
		t.Fatalf("live empty = %v %v", ns, err)
	}
}

func TestListNotesFiltersTombstones(t *testing.T) {
	ft := newFake(inlineFixture("cFji9",
		`[[["n1",["n1","Body one",[null,null,[1700000000,5]],null,"One"]],["n2",null,2],["n3",["n3","Body three",null,null,"Three"]]]]`))
	ns, err := New(ft).ListNotes(context.Background(), nb1)
	if err != nil || len(ns) != 2 {
		t.Fatalf("notes = %+v %v", ns, err)
	}
	if ns[0].ID != "n1" || ns[0].Title != "One" || ns[0].Content != "Body one" || ns[0].Updated.Unix() != 1700000000 {
		t.Fatalf("note0 = %+v", ns[0])
	}
	if ns[1].ID != "n3" || ns[1].Title != "Three" || !ns[1].Updated.IsZero() {
		t.Fatalf("note1 = %+v", ns[1])
	}
	c, _ := ft.callFor("cFji9")
	if !c.Idempotent || c.SourcePath != "/notebook/"+nb1 {
		t.Fatalf("call = %+v", c)
	}
}

func TestSaveNoteCreatesThenSetsTitle(t *testing.T) {
	cr, mu := loadFixture(t, "CYK0Xb", "create"), loadFixture(t, "cYAfTb", "mutate")
	ft := newFake(cr, mu)
	n, err := New(ft).SaveNote(context.Background(), nb1, NoteInput{Title: "My title", Content: "Body text"})
	if err != nil {
		t.Fatal(err)
	}
	if p := paramsJSON(t, ft, "CYK0Xb"); !sameJSON(t, []byte(p), cr.Request) {
		t.Fatalf("create %s", p)
	}
	if p := paramsJSON(t, ft, "cYAfTb"); !sameJSON(t, []byte(p), mu.Request) {
		t.Fatalf("mutate %s", p)
	}
	if n.ID != "note-0001" || n.Title != "My title" || n.Content != "Body text" {
		t.Fatalf("note = %+v", n)
	}
	for _, id := range []string{"CYK0Xb", "cYAfTb"} {
		if c, _ := ft.callFor(id); c.Idempotent {
			t.Fatalf("%s must not be retried: %+v", id, c)
		}
	}
}

func TestSaveNoteUpdateSkipsCreate(t *testing.T) {
	ft := newFake(loadFixture(t, "cYAfTb", "mutate"))
	n, err := New(ft).SaveNote(context.Background(), nb1, NoteInput{ID: "note-0001", Title: "T", Content: "C"})
	if err != nil || n.ID != "note-0001" {
		t.Fatalf("note = %+v %v", n, err)
	}
	if _, ok := ft.callFor("CYK0Xb"); ok {
		t.Fatal("update must not create")
	}
	if p := paramsJSON(t, ft, "cYAfTb"); !strings.Contains(p, `"note-0001"`) || !strings.Contains(p, `[["C","T",[],0]]`) {
		t.Fatalf("mutate %s", p)
	}
}

func TestSaveNoteEmptyIDFromCreate(t *testing.T) {
	_, err := New(newFake(inlineFixture("CYK0Xb", `[]`))).SaveNote(context.Background(), nb1, NoteInput{Title: "t"})
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindDrift {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteNotesBlocked(t *testing.T) {
	ft := newFake()
	err := New(ft).DeleteNotes(context.Background(), nb1, []string{"n1"})
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindUnsupported || !strings.Contains(te.Message, "AH0mwd") {
		t.Fatalf("err = %v", err)
	}
	if len(ft.calls) != 0 {
		t.Fatal("blocked delete must not call")
	}
}
