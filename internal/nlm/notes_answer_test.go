package nlm

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// streamSnaps decodes a live chat stream fixture.
func streamSnaps(t *testing.T, name string) []wire.Snapshot {
	t.Helper()
	fx := loadFixture(t, "GenerateFreeFormStreamed", name)
	d := wire.NewStreamDecoder(bytes.NewReader([]byte(fx.ResponseBody)))
	var out []wire.Snapshot
	for {
		s, err := d.Next()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
}

// liveAnswer is the answer Ask returns for a captured stream.
func liveAnswer(t *testing.T, stream string) Answer {
	t.Helper()
	fs := &fakeStreamer{fakeTransport: newFake(), snaps: streamSnaps(t, stream)}
	a, err := New(fs).Ask(context.Background(), fakeNB, "q", AskOptions{SourceIDs: studioSources3, ConversationID: "conv"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// Saving a chat answer as a note (CYK0Xb) re-sends the stream's final
// answer: its text, its passages and its rich answer block, re-encoded the
// way the web app does it. Both captures that saved an answer (#2 and #3)
// are reproduced exactly from the decoded stream. The title is the
// caller's; the server answers with its own.
func TestAnswerNoteParamsLive(t *testing.T) {
	for _, tc := range []struct {
		stream, note, nb, title string
	}{
		{"live3_2026-10-20_01", "live3_2026-10-20_01", fakeNB, "text-556"},
		{"live_2026-10-20_01", "live_2026-10-20_01", fakeNB2, "text-75"},
	} {
		a := liveAnswer(t, tc.stream)
		p, err := answerNoteParams(tc.nb, a, tc.title)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := json.Marshal(p)
		if want := loadFixture(t, "CYK0Xb", tc.note).Request; !sameJSON(t, got, want) {
			t.Fatalf("%s: params differ\n got %.600s\nwant %.600s", tc.note, got, want)
		}
	}
}

func TestSaveAnswerNoteLive3(t *testing.T) {
	a := liveAnswer(t, "live3_2026-10-20_01")
	ft := newFake(loadFixture(t, "CYK0Xb", "live3_2026-10-20_01"))
	n, err := New(ft).SaveAnswerNote(context.Background(), fakeNB, a, "text-556")
	if err != nil {
		t.Fatal(err)
	}
	if n.ID != "00000000-0000-4000-8000-000000000286" || n.Title != "text-555" || n.Content != "text-496" || n.Updated.IsZero() {
		t.Fatalf("note = %+v", n)
	}
	c, _ := ft.callFor("CYK0Xb")
	if c.RPC != "notes.create" || c.Idempotent || c.SourcePath != "/notebook/"+fakeNB {
		t.Fatalf("call = %+v", c)
	}
}

// An answer without the stream's answer block (a stream that never sent
// its final flag, or an answer read from the page) cannot be saved.
func TestSaveAnswerNoteNeedsAnswerBlock(t *testing.T) {
	ft := newFake()
	_, err := New(ft).SaveAnswerNote(context.Background(), fakeNB, Answer{Text: "hi"}, "t")
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindUnsupported || len(ft.calls) != 0 {
		t.Fatalf("err = %v, calls %d", err, len(ft.calls))
	}
}

// Format slots 0, 1 and 7 were seen set in the captures; 2 and 8 only as
// 0. They are encoded the same way (a boolean as 0/1), by analogy.
func TestNoteFormatFlags(t *testing.T) {
	got, _ := json.Marshal(noteFormat([]any{true, false, true, "u", nil, nil, nil, true, true}))
	if string(got) != `[1,0,1,"u",null,null,null,1,1]` {
		t.Fatalf("format = %s", got)
	}
	got, _ = json.Marshal(noteFormat(nil))
	if string(got) != `[0,0,0,null,null,null,null,0,0]` {
		t.Fatalf("empty format = %s", got)
	}
}

// The answer block stays out of the JSON a tool returns.
func TestAnswerBlockNotSerialized(t *testing.T) {
	a := liveAnswer(t, "live3_2026-10-20_01")
	b, _ := json.Marshal(a)
	if bytes.Contains(b, []byte("text-270")) {
		t.Fatalf("answer JSON carries the rich block: %.300s", b)
	}
}
