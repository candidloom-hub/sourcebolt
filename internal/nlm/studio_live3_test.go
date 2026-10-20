package nlm

import (
	"context"
	"testing"
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
