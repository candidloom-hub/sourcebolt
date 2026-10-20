package nlm

import (
	"context"
	"encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

// Live capture #3 (2026-10-20) generated one artifact of every Studio type
// in one notebook; gArtLc then listed them while some were still
// generating (_01) and when all but the video were done (_02).
func TestListArtifactsLive3AllKinds(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		want    map[string]Artifact // by id; only Kind and Status are compared
	}{
		{"live3_2026-10-20_01", map[string]Artifact{
			"00000000-0000-4000-8000-000000000010": {Kind: StudioAudio, Status: ArtifactReady},
			"00000000-0000-4000-8000-000000000018": {Kind: StudioDataTable, Status: ArtifactReady},
			"00000000-0000-4000-8000-000000000016": {Kind: StudioFlashcards, Status: ArtifactReady},
			"00000000-0000-4000-8000-000000000015": {Kind: StudioQuiz, Status: ArtifactReady},
			"00000000-0000-4000-8000-000000000017": {Kind: StudioInfographic, Status: ArtifactPending},
			"00000000-0000-4000-8000-000000000013": {Kind: StudioSlides, Status: ArtifactPending},
			"00000000-0000-4000-8000-000000000014": {Kind: StudioMindMap, Status: ArtifactReady},
			"00000000-0000-4000-8000-000000000012": {Kind: StudioReport, Status: ArtifactReady},
			"00000000-0000-4000-8000-000000000011": {Kind: StudioVideo, Status: ArtifactPending},
		}},
		{"live3_2026-10-20_02", map[string]Artifact{
			"00000000-0000-4000-8000-000000000013": {Kind: StudioSlides, Status: ArtifactReady},
			"00000000-0000-4000-8000-000000000017": {Kind: StudioInfographic, Status: ArtifactReady},
			"00000000-0000-4000-8000-000000000010": {Kind: StudioAudio, Status: ArtifactReady},
			"00000000-0000-4000-8000-000000000018": {Kind: StudioDataTable, Status: ArtifactReady},
			"00000000-0000-4000-8000-000000000016": {Kind: StudioFlashcards, Status: ArtifactReady},
			"00000000-0000-4000-8000-000000000015": {Kind: StudioQuiz, Status: ArtifactReady},
			"00000000-0000-4000-8000-000000000014": {Kind: StudioMindMap, Status: ArtifactReady},
			"00000000-0000-4000-8000-000000000012": {Kind: StudioReport, Status: ArtifactReady},
			"00000000-0000-4000-8000-000000000011": {Kind: StudioVideo, Status: ArtifactPending},
		}},
	} {
		as, err := New(newFake(loadFixture(t, "gArtLc", tc.fixture))).ListArtifacts(context.Background(), fakeNB)
		if err != nil {
			t.Fatal(err)
		}
		if len(as) != len(tc.want) {
			t.Fatalf("%s: %d artifacts, want %d", tc.fixture, len(as), len(tc.want))
		}
		for _, a := range as {
			w, ok := tc.want[a.ID]
			if !ok || a.Kind != w.Kind || a.Status != w.Status || a.Created.IsZero() || a.Title == "" {
				t.Errorf("%s: %s = kind %q status %q created %v, want %q %q", tc.fixture, a.ID, a.Kind, a.Status, a.Created, w.Kind, w.Status)
			}
		}
	}
}

// The tool-facing kind names are the studio_generate kind names; the slide
// deck used to list as "slides", which the kind filter never matched.
func TestStudioKindNames(t *testing.T) {
	for k, want := range map[StudioKind]string{
		StudioAudio: "audio", StudioVideo: "video", StudioReport: "report", StudioSlides: "slide_deck",
		StudioMindMap: "mind_map", StudioQuiz: "quiz", StudioFlashcards: "flashcards",
		StudioInfographic: "infographic", StudioDataTable: "data_table",
	} {
		if string(k) != want {
			t.Errorf("kind %q, want %q", k, want)
		}
	}
}

// Sources of every generation in live capture #3.
var studioSources3 = []string{"00000000-0000-4000-8000-000000000004", "00000000-0000-4000-8000-000000000005"}

// liveStudioRequests3 are the requests that reproduce the nine R7cb6c
// calls of live capture #3, in UI order. Free text is the fixture's
// sanitized text; option values are the ones the UI sent.
var liveStudioRequests3 = []struct {
	fixture string
	req     StudioRequest
	id      string
}{
	{"live3_2026-10-20_01", StudioRequest{Kind: StudioAudio, Instructions: "text-66", Format: "critique", Length: "short"}, "00000000-0000-4000-8000-000000000010"},
	{"live3_2026-10-20_02", StudioRequest{Kind: StudioVideo, Instructions: "text-65", Language: "en_in", Format: "short"}, "00000000-0000-4000-8000-000000000011"},
	{"live3_2026-10-20_03", StudioRequest{Kind: StudioReport, Title: "text-85", Description: "text-23", Instructions: "text-86"}, "00000000-0000-4000-8000-000000000012"},
	{"live3_2026-10-20_04", StudioRequest{Kind: StudioSlides, Instructions: "text-89", Format: "presenter_slides", Length: "short"}, "00000000-0000-4000-8000-000000000013"},
	{"live3_2026-10-20_05", StudioRequest{Kind: StudioMindMap, Instructions: "channels"}, "00000000-0000-4000-8000-000000000014"},
	{"live3_2026-10-20_06", StudioRequest{Kind: StudioQuiz, Instructions: "interfaces", Quantity: "more", Difficulty: "easy"}, "00000000-0000-4000-8000-000000000015"},
	{"live3_2026-10-20_07", StudioRequest{Kind: StudioFlashcards, Instructions: "text-200", Quantity: "standard", Difficulty: "medium", Images: "include"}, "00000000-0000-4000-8000-000000000016"},
	{"live3_2026-10-20_08", StudioRequest{Kind: StudioInfographic, Orientation: "portrait", Detail: "concise", Style: "sketch_note"}, "00000000-0000-4000-8000-000000000017"},
	{"live3_2026-10-20_09", StudioRequest{Kind: StudioDataTable, Instructions: "text-214"}, "00000000-0000-4000-8000-000000000018"},
}

func TestGenerateLive3EveryKind(t *testing.T) {
	for _, tc := range liveStudioRequests3 {
		fx := loadFixture(t, "R7cb6c", tc.fixture)
		ft := newFake(fx)
		r := tc.req
		r.SourceIDs = studioSources3
		a, err := New(ft).Generate(context.Background(), fakeNB, r)
		if err != nil {
			t.Fatalf("%s: %v", tc.req.Kind, err)
		}
		if p := paramsJSON(t, ft, "R7cb6c"); !sameJSON(t, []byte(p), fx.Request) {
			t.Fatalf("%s params\n got %s\nwant %s", tc.req.Kind, p, fx.Request)
		}
		if a.ID != tc.id || a.Kind != tc.req.Kind || a.Status != ArtifactPending || a.Title != "text-39" {
			t.Fatalf("%s: artifact = %+v", tc.req.Kind, a)
		}
	}
}

// Capture #3b generated six artifacts with every option left at the web
// app's default and no instructions. The web app also listed a note as a
// source; sourcebolt sends sources only, so the note is removed from the
// captured request before comparing.
const note3b = "00000000-0000-4000-8000-000000000014"

var studioSources3b = []string{"00000000-0000-4000-8000-000000000003", "00000000-0000-4000-8000-000000000004"}

// withoutNote drops the note entries ([null×6,[note,2]] and [note]) from
// a captured request.
func withoutNote(t *testing.T, raw []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	var walk func(any) any
	walk = func(x any) any {
		l, ok := x.([]any)
		if !ok {
			return x
		}
		out := []any{}
		for _, e := range l {
			b, _ := json.Marshal(e)
			if string(b) == `[null,null,null,null,null,null,["`+note3b+`",2]]` || string(b) == `["`+note3b+`"]` {
				continue
			}
			out = append(out, walk(e))
		}
		return out
	}
	b, _ := json.Marshal(walk(v))
	return b
}

func TestGenerateLive3bDefaults(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		req     StudioRequest
		id      string
		status  ArtifactStatus
	}{
		{"live3b_2026-10-20_01", StudioRequest{Kind: StudioAudio}, "00000000-0000-4000-8000-000000000013", ArtifactPending},
		{"live3b_2026-10-20_02", StudioRequest{Kind: StudioVideo, GenerateLater: true}, "00000000-0000-4000-8000-000000000015", ArtifactPending},
		{"live3b_2026-10-20_03", StudioRequest{Kind: StudioSlides}, "00000000-0000-4000-8000-000000000016", ArtifactPending},
		{"live3b_2026-10-20_04", StudioRequest{Kind: StudioQuiz}, "00000000-0000-4000-8000-000000000017", ArtifactPending},
		{"live3b_2026-10-20_05", StudioRequest{Kind: StudioInfographic}, "00000000-0000-4000-8000-000000000018", ArtifactPending},
		{"live3b_2026-10-20_06", StudioRequest{Kind: StudioInteractiveReport}, "00000000-0000-4000-8000-000000000019", ArtifactPending},
	} {
		fx := loadFixture(t, "R7cb6c", tc.fixture)
		ft := newFake(fx)
		r := tc.req
		r.SourceIDs = studioSources3b
		a, err := New(ft).Generate(context.Background(), fakeNB, r)
		if err != nil {
			t.Fatalf("%s: %v", r.Kind, err)
		}
		if p, want := paramsJSON(t, ft, "R7cb6c"), withoutNote(t, fx.Request); !sameJSON(t, []byte(p), want) {
			t.Fatalf("%s params\n got %s\nwant %s", r.Kind, p, want)
		}
		if a.ID != tc.id || a.Kind != r.Kind || a.Status != tc.status {
			t.Fatalf("%s: artifact = %+v", r.Kind, a)
		}
	}
}

// The default values are also accepted by name and give the same request.
func TestGenerateLive3bDefaultsByName(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		req     StudioRequest
	}{
		{"live3b_2026-10-20_01", StudioRequest{Kind: StudioAudio, Format: "deep_dive", Length: "default"}},
		{"live3b_2026-10-20_02", StudioRequest{Kind: StudioVideo, Format: "cinematic", GenerateLater: true}},
		{"live3b_2026-10-20_03", StudioRequest{Kind: StudioSlides, Format: "detailed_deck", Length: "default"}},
		{"live3b_2026-10-20_04", StudioRequest{Kind: StudioQuiz, Quantity: "standard", Difficulty: "medium"}},
		{"live3b_2026-10-20_05", StudioRequest{Kind: StudioInfographic, Orientation: "landscape", Detail: "standard", Style: "auto"}},
	} {
		fx := loadFixture(t, "R7cb6c", tc.fixture)
		ft := newFake(fx)
		r := tc.req
		r.SourceIDs = studioSources3b
		if _, err := New(ft).Generate(context.Background(), fakeNB, r); err != nil {
			t.Fatalf("%s: %v", r.Kind, err)
		}
		if p, want := paramsJSON(t, ft, "R7cb6c"), withoutNote(t, fx.Request); !sameJSON(t, []byte(p), want) {
			t.Fatalf("%s params\n got %s\nwant %s", r.Kind, p, want)
		}
	}
}

// A deferred ("Generate later") request answers with status 7, which lists
// as pending.
func TestStatusDeferredIsPending(t *testing.T) {
	if got := statusFromCode(7); got != ArtifactPending {
		t.Fatalf("status 7 = %q", got)
	}
}

// Option values the web app offers but no capture has sent are
// KindUnsupported and name the option; values the web app does not offer
// and options of another kind are KindInvalidArgument; free text that no
// capture left out (or, for the infographic, put in) is KindUnsupported.
// No call is made for any of them.
func TestGenerateRejectsUnprovenOptions(t *testing.T) {
	topic := "t"
	for _, tc := range []struct {
		r    StudioRequest
		kind transport.Kind
		says string
	}{
		{StudioRequest{Kind: StudioAudio, Instructions: topic, Format: "brief"}, transport.KindUnsupported, "format"},
		{StudioRequest{Kind: StudioAudio, Instructions: topic, Length: "long"}, transport.KindUnsupported, "length"},
		{StudioRequest{Kind: StudioAudio, Instructions: topic, Format: "poem"}, transport.KindInvalidArgument, "format"},
		{StudioRequest{Kind: StudioMindMap}, transport.KindUnsupported, "instructions"},
		{StudioRequest{Kind: StudioFlashcards}, transport.KindUnsupported, "instructions"},
		{StudioRequest{Kind: StudioVideo, Format: "cinematic", Language: "en"}, transport.KindUnsupported, "language"},
		{StudioRequest{Kind: StudioVideo, Format: "short", GenerateLater: true}, transport.KindUnsupported, "generate_later"},
		{StudioRequest{Kind: StudioAudio, GenerateLater: true}, transport.KindInvalidArgument, "generate_later"},
		{StudioRequest{Kind: StudioVideo, Instructions: topic, Format: "explainer"}, transport.KindUnsupported, "format"},
		{StudioRequest{Kind: StudioReport, Instructions: topic}, transport.KindUnsupported, "title"},
		{StudioRequest{Kind: StudioReport, Instructions: topic, Title: "x"}, transport.KindUnsupported, "description"},
		{StudioRequest{Kind: StudioReport, Title: "x", Description: "y"}, transport.KindUnsupported, "instructions"},
		{StudioRequest{Kind: StudioSlides, Instructions: topic, Format: "outline"}, transport.KindInvalidArgument, "format"},
		{StudioRequest{Kind: StudioSlides, Instructions: topic, Length: "long"}, transport.KindInvalidArgument, "length"},
		{StudioRequest{Kind: StudioQuiz, Instructions: topic, Quantity: "fewer"}, transport.KindUnsupported, "quantity"},
		{StudioRequest{Kind: StudioQuiz, Instructions: topic, Difficulty: "hard"}, transport.KindUnsupported, "difficulty"},
		{StudioRequest{Kind: StudioFlashcards, Instructions: topic, Quantity: "more"}, transport.KindUnsupported, "quantity"},
		{StudioRequest{Kind: StudioFlashcards, Instructions: topic, Difficulty: "easy"}, transport.KindUnsupported, "difficulty"},
		{StudioRequest{Kind: StudioFlashcards, Instructions: topic, Images: "some"}, transport.KindInvalidArgument, "images"},
		{StudioRequest{Kind: StudioInfographic, Orientation: "square"}, transport.KindUnsupported, "orientation"},
		{StudioRequest{Kind: StudioInfographic, Detail: "detailed"}, transport.KindUnsupported, "detail"},
		{StudioRequest{Kind: StudioInfographic, Style: "watercolor"}, transport.KindInvalidArgument, "style"},
		{StudioRequest{Kind: StudioInfographic, Instructions: topic}, transport.KindUnsupported, "instructions"},
		{StudioRequest{Kind: StudioMindMap, Instructions: topic, Format: "short"}, transport.KindInvalidArgument, "format"},
		{StudioRequest{Kind: StudioDataTable, Instructions: topic, Quantity: "more"}, transport.KindInvalidArgument, "quantity"},
		{StudioRequest{Kind: StudioDataTable}, transport.KindUnsupported, "instructions"},
		{StudioRequest{Kind: StudioInteractiveReport, Instructions: topic}, transport.KindUnsupported, "instructions"},
		{StudioRequest{Kind: "poem", Instructions: topic}, transport.KindInvalidArgument, "poem"},
		{StudioRequest{Kind: StudioQuiz, Language: "de"}, transport.KindUnsupported, "language"},
		{StudioRequest{Kind: StudioAudio, Language: "en-IN"}, transport.KindUnsupported, "language"},
	} {
		ft := newFake()
		tc.r.SourceIDs = studioSources3
		_, err := New(ft).Generate(context.Background(), fakeNB, tc.r)
		var te *transport.Error
		if !errors.As(err, &te) || te.Kind != tc.kind || !strings.Contains(te.Message, tc.says) {
			t.Errorf("%+v: err = %v, want %s naming %q", tc.r, err, tc.kind, tc.says)
		}
		if len(ft.calls) != 0 {
			t.Errorf("%+v: made %d calls", tc.r, len(ft.calls))
		}
	}
}
