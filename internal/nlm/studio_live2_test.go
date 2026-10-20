package nlm

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Source ids of the flashcards generation in live capture #2. The web app
// also sent four notes as sources; sourcebolt sends sources only.
var flashcardSources2 = []string{
	"00000000-0000-4000-8000-000000000004",
	"00000000-0000-4000-8000-000000000005",
	"00000000-0000-4000-8000-000000000012",
	"00000000-0000-4000-8000-000000000001",
}

// liveFlashcardsRequest is the captured R7cb6c request without the note
// entries ([null×6,[note_id,2]]) of its source list.
func liveFlashcardsRequest(t *testing.T, fx Fixture) []byte {
	t.Helper()
	var req []any
	if err := json.Unmarshal(fx.Request, &req); err != nil {
		t.Fatal(err)
	}
	spec := req[2].([]any)
	var srcs []any
	for _, s := range spec[3].([]any) {
		if s.([]any)[0] != nil {
			srcs = append(srcs, s)
		}
	}
	if len(srcs) != len(flashcardSources2) {
		t.Fatalf("fixture sources = %d", len(srcs))
	}
	spec[3] = srcs
	b, _ := json.Marshal(req)
	return b
}

func TestGenerateFlashcardsLive(t *testing.T) {
	fx := loadFixture(t, "R7cb6c", "live_2026-10-20_01")
	for _, r := range []StudioRequest{
		{Kind: StudioFlashcards, SourceIDs: flashcardSources2, Instructions: "text-92", Language: "en", Quantity: "fewer", Difficulty: "hard", Images: "text_only"},
		{Kind: StudioFlashcards, SourceIDs: flashcardSources2, Instructions: "text-92", Quantity: "fewer", Difficulty: "hard", Images: "text_only"}, // language defaults to "en"
	} {
		ft := newFake(fx)
		a, err := New(ft).Generate(context.Background(), fakeNB2, r)
		if err != nil {
			t.Fatal(err)
		}
		if p := paramsJSON(t, ft, "R7cb6c"); !sameJSON(t, []byte(p), liveFlashcardsRequest(t, fx)) {
			t.Fatalf("params %s\nwant   %s", p, liveFlashcardsRequest(t, fx))
		}
		if a.ID != "00000000-0000-4000-8000-000000000018" || a.Kind != StudioFlashcards || a.Status != ArtifactPending || a.Title != "text-78" {
			t.Fatalf("artifact = %+v", a)
		}
		c, _ := ft.callFor("R7cb6c")
		if c.Idempotent || c.SourcePath != "/notebook/"+fakeNB2 {
			t.Fatalf("call = %+v", c)
		}
	}
}

// Review item 7: the R7cb6c answer is decoded from the live fixture, the
// call allows a null answer like the other notebook-scoped calls (one
// non-null capture does not prove it is never null), and a null answer is
// KindDrift, not a transport failure.
func TestGenerateFlashcardsNullAnswer(t *testing.T) {
	ft := newFake(loadFixture(t, "R7cb6c", "live_2026-10-20_01"))
	a, err := New(ft).Generate(context.Background(), fakeNB2, StudioRequest{Kind: StudioFlashcards, SourceIDs: flashcardSources2, Instructions: "text-92"})
	if err != nil || a.ID != "00000000-0000-4000-8000-000000000018" || a.Title != "text-78" || a.Kind != StudioFlashcards || a.Status != ArtifactPending {
		t.Fatalf("artifact = %+v %v", a, err)
	}
	if c, _ := ft.callFor("R7cb6c"); c.Null != wire.NullAllowed {
		t.Fatalf("null policy = %v, want NullAllowed", c.Null)
	}
	row := `[["wrb.fr","R7cb6c",null,null,null,null,"generic"]]`
	null := Fixture{RPC: "R7cb6c", ResponseBody: ")]}'\n\n" + fmt.Sprint(len(row)+1) + "\n" + row + "\n"}
	_, err = New(newFake(null)).Generate(context.Background(), fakeNB2, StudioRequest{Kind: StudioFlashcards, SourceIDs: flashcardSources2, Instructions: "t"})
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindDrift {
		t.Fatalf("null answer: err = %v", err)
	}
}

func TestGenerateFlashcardsAllSourcesByDefault(t *testing.T) {
	ft := newFake(loadFixture(t, "rLM1Ne", "live_2026-10-13_03"), loadFixture(t, "R7cb6c", "live_2026-10-20_01"))
	if _, err := New(ft).Generate(context.Background(), fakeNB, StudioRequest{Kind: StudioFlashcards, Instructions: "t"}); err != nil {
		t.Fatal(err)
	}
	ids, err := New(ft).notebookSourceIDs(context.Background(), fakeNB)
	if err != nil || len(ids) != 1 {
		t.Fatalf("ids = %v %v", ids, err)
	}
	var req []any
	if err := json.Unmarshal([]byte(paramsJSON(t, ft, "R7cb6c")), &req); err != nil {
		t.Fatal(err)
	}
	if got, _ := json.Marshal(req[2].([]any)[3]); string(got) != `[[["`+ids[0]+`"]]]` {
		t.Fatalf("sources = %s", got)
	}
}

func TestGenerateUnpinnedOptionsUnsupported(t *testing.T) {
	for name, r := range map[string]StudioRequest{
		"quantity":     {Kind: StudioFlashcards, SourceIDs: flashcardSources2, Instructions: "t", Quantity: "more"},
		"difficulty":   {Kind: StudioFlashcards, SourceIDs: flashcardSources2, Instructions: "t", Difficulty: "easy"},
		"no topic":     {Kind: StudioFlashcards, SourceIDs: flashcardSources2},
		"quiz":         {Kind: StudioQuiz, SourceIDs: flashcardSources2, Instructions: "t", Difficulty: "hard"},
		"audio":        {Kind: StudioAudio, SourceIDs: flashcardSources2},
		"bad quantity": {Kind: StudioFlashcards, SourceIDs: flashcardSources2, Instructions: "t", Quantity: "lots"},
	} {
		ft := newFake()
		_, err := New(ft).Generate(context.Background(), fakeNB2, r)
		want := transport.KindUnsupported
		if name == "bad quantity" {
			want = transport.KindInvalidArgument
		}
		var te *transport.Error
		if !errors.As(err, &te) || te.Kind != want {
			t.Fatalf("%s: err = %v", name, err)
		}
		if len(ft.calls) != 0 {
			t.Fatalf("%s: made %d calls", name, len(ft.calls))
		}
	}
}

func TestGetArtifactLiveRequest(t *testing.T) {
	fx := loadFixture(t, "v9rmvd", "live_2026-10-20_01")
	ft := newFake(fx)
	// The captured artifact was still generating: no interactive content yet.
	_, err := New(ft).GetArtifact(context.Background(), "00000000-0000-4000-8000-000000000018")
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindNotFound {
		t.Fatalf("err = %v", err)
	}
	if p := paramsJSON(t, ft, "v9rmvd"); !sameJSON(t, []byte(p), fx.Request) {
		t.Fatalf("params %s\nwant   %s", p, fx.Request)
	}
}

func TestListArtifactsLive2(t *testing.T) {
	as, err := New(newFake(loadFixture(t, "gArtLc", "live_2026-10-20_01"))).ListArtifacts(context.Background(), fakeNB2)
	if err != nil || len(as) != 1 {
		t.Fatalf("artifacts = %+v %v", as, err)
	}
	a := as[0]
	if a.ID != "00000000-0000-4000-8000-000000000003" || a.Title != "text-34" || a.Kind != StudioFlashcards || a.Status != ArtifactReady || a.Created.Unix() != 1792505772 {
		t.Fatalf("artifact = %+v", a)
	}
}
