package nlm

import (
	"context"
	"encoding/json/v2"
	"errors"
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

func TestSaveNoteEmptyIDFromCreate(t *testing.T) {
	_, err := New(newFake(inlineFixture("CYK0Xb", `[]`))).SaveNote(context.Background(), nb1, NoteInput{Title: "t"})
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindDrift {
		t.Fatalf("err = %v", err)
	}
}
