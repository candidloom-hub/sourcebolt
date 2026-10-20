package nlm

import (
	"context"
	"testing"
)

// fakeNote2 is the note the "Copied text" flow created in live capture #2.
const fakeNote2 = "00000000-0000-4000-8000-000000000017"

// TestSaveNoteCreateLive: a new note is one CYK0Xb call carrying title and
// content, exactly like the web app's "Copied text" flow (live capture #2).
func TestSaveNoteCreateLive(t *testing.T) {
	fx := loadFixture(t, "CYK0Xb", "live_2026-10-20_02")
	ft := newFake(fx)
	n, err := New(ft).SaveNote(context.Background(), fakeNB2, NoteInput{Title: "text-74", Content: "text-76"})
	if err != nil {
		t.Fatal(err)
	}
	if p := paramsJSON(t, ft, "CYK0Xb"); !sameJSON(t, []byte(p), fx.Request) {
		t.Fatalf("create %s\nwant   %s", p, fx.Request)
	}
	if n.ID != fakeNote2 || n.Title != "text-74" || n.Content != "text-76" {
		t.Fatalf("note = %+v", n)
	}
	if len(ft.calls) != 1 {
		t.Fatalf("calls = %d, want 1 (no follow-up cYAfTb)", len(ft.calls))
	}
	if c, _ := ft.callFor("CYK0Xb"); c.Idempotent || c.SourcePath != "/notebook/"+fakeNB2 {
		t.Fatalf("create call = %+v", c)
	}
}

func TestSaveNoteUpdateLive(t *testing.T) {
	fx := loadFixture(t, "cYAfTb", "live_2026-10-20_01")
	ft := newFake(fx)
	n, err := New(ft).SaveNote(context.Background(), fakeNB2, NoteInput{ID: fakeNote2, Title: "text-77", Content: "text-76"})
	if err != nil || n.ID != fakeNote2 {
		t.Fatalf("note = %+v %v", n, err)
	}
	if _, ok := ft.callFor("CYK0Xb"); ok {
		t.Fatal("update must not create")
	}
	if p := paramsJSON(t, ft, "cYAfTb"); !sameJSON(t, []byte(p), fx.Request) {
		t.Fatalf("mutate %s\nwant   %s", p, fx.Request)
	}
	if c, _ := ft.callFor("cYAfTb"); c.Idempotent {
		t.Fatal("mutate must not be retried")
	}
}

func TestListNotesLive2(t *testing.T) {
	// _03: the copied-text note; _01: a chat answer saved as a note.
	for fixture, want := range map[string]Note{
		"live_2026-10-20_03": {ID: fakeNote2, Title: "text-74", Content: "text-76"},
		"live_2026-10-20_01": {ID: "00000000-0000-4000-8000-000000000016", Title: "text-74", Content: "text-73"},
	} {
		ns, err := New(newFake(loadFixture(t, "cFji9", fixture))).ListNotes(context.Background(), fakeNB2)
		if err != nil || len(ns) != 1 {
			t.Fatalf("%s: notes = %+v %v", fixture, ns, err)
		}
		got := ns[0]
		if got.ID != want.ID || got.Title != want.Title || got.Content != want.Content || got.Updated.IsZero() {
			t.Fatalf("%s: note = %+v", fixture, got)
		}
	}
}
