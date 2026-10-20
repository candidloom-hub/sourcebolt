package nlm

import (
	"context"
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

// Empty options take the defaults: the only captured value, or for
// flashcards the web app's own defaults (standard, medium, with images),
// which capture #3 sent.
func TestGenerateLive3Defaults(t *testing.T) {
	for _, tc := range liveStudioRequests3 {
		r := StudioRequest{Kind: tc.req.Kind, Instructions: tc.req.Instructions, Language: tc.req.Language,
			Title: tc.req.Title, Description: tc.req.Description, SourceIDs: studioSources3}
		fx := loadFixture(t, "R7cb6c", tc.fixture)
		ft := newFake(fx)
		if _, err := New(ft).Generate(context.Background(), fakeNB, r); err != nil {
			t.Fatalf("%s: %v", r.Kind, err)
		}
		if p := paramsJSON(t, ft, "R7cb6c"); !sameJSON(t, []byte(p), fx.Request) {
			t.Fatalf("%s params\n got %s\nwant %s", r.Kind, p, fx.Request)
		}
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
		{StudioRequest{Kind: StudioAudio, Instructions: topic, Format: "deep_dive"}, transport.KindUnsupported, "format"},
		{StudioRequest{Kind: StudioAudio, Instructions: topic, Length: "long"}, transport.KindUnsupported, "length"},
		{StudioRequest{Kind: StudioAudio, Instructions: topic, Format: "poem"}, transport.KindInvalidArgument, "format"},
		{StudioRequest{Kind: StudioAudio}, transport.KindUnsupported, "instructions"},
		{StudioRequest{Kind: StudioVideo, Instructions: topic, Format: "explainer"}, transport.KindUnsupported, "format"},
		{StudioRequest{Kind: StudioReport, Instructions: topic}, transport.KindUnsupported, "title"},
		{StudioRequest{Kind: StudioReport, Instructions: topic, Title: "x"}, transport.KindUnsupported, "description"},
		{StudioRequest{Kind: StudioReport, Title: "x", Description: "y"}, transport.KindUnsupported, "instructions"},
		{StudioRequest{Kind: StudioSlides, Instructions: topic, Format: "detailed_deck"}, transport.KindUnsupported, "format"},
		{StudioRequest{Kind: StudioSlides, Instructions: topic, Length: "default"}, transport.KindUnsupported, "length"},
		{StudioRequest{Kind: StudioQuiz, Instructions: topic, Quantity: "standard"}, transport.KindUnsupported, "quantity"},
		{StudioRequest{Kind: StudioQuiz, Instructions: topic, Difficulty: "hard"}, transport.KindUnsupported, "difficulty"},
		{StudioRequest{Kind: StudioFlashcards, Instructions: topic, Quantity: "more"}, transport.KindUnsupported, "quantity"},
		{StudioRequest{Kind: StudioFlashcards, Instructions: topic, Difficulty: "easy"}, transport.KindUnsupported, "difficulty"},
		{StudioRequest{Kind: StudioFlashcards, Instructions: topic, Images: "some"}, transport.KindInvalidArgument, "images"},
		{StudioRequest{Kind: StudioInfographic, Orientation: "landscape"}, transport.KindUnsupported, "orientation"},
		{StudioRequest{Kind: StudioInfographic, Detail: "detailed"}, transport.KindUnsupported, "detail"},
		{StudioRequest{Kind: StudioInfographic, Style: "auto"}, transport.KindInvalidArgument, "style"},
		{StudioRequest{Kind: StudioInfographic, Instructions: topic}, transport.KindUnsupported, "instructions"},
		{StudioRequest{Kind: StudioMindMap, Instructions: topic, Format: "short"}, transport.KindInvalidArgument, "format"},
		{StudioRequest{Kind: StudioDataTable, Instructions: topic, Quantity: "more"}, transport.KindInvalidArgument, "quantity"},
		{StudioRequest{Kind: StudioDataTable}, transport.KindUnsupported, "instructions"},
		{StudioRequest{Kind: StudioInteractiveReport, Instructions: topic}, transport.KindUnsupported, "interactive_report"},
		{StudioRequest{Kind: "poem", Instructions: topic}, transport.KindInvalidArgument, "poem"},
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
