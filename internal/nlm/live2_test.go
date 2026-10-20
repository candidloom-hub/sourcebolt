package nlm

import (
	"encoding/json/v2"
	"strings"
	"testing"
)

// Fake ids the capture sanitizer gave live capture #2 (2026-10-20). The
// numbering is per fixture, so the same number can name different objects in
// different fixtures.
const (
	fakeNB2  = "00000000-0000-4000-8000-000000000008" // notebook of most #2 fixtures
	fakeSrc1 = "00000000-0000-4000-8000-000000000001"
)

// TestLive2RequestShapes pins payload builders to the requests the web app
// sent in live capture #2 (2026-10-20, notebook.google.com).
func TestLive2RequestShapes(t *testing.T) {
	cases := []struct {
		rpc, fixture string
		params       any
	}{
		{"s0tc2d", "live_2026-10-20_01", renameNotebookParams(fakeNB2, "text-78")},
		{"hizoJc", "live_2026-10-20_01", sourceLoadParams(fakeSrc1, false)},
	}
	for _, c := range cases {
		fx := loadFixture(t, c.rpc, c.fixture)
		if fx.Synthetic {
			t.Fatalf("%s/%s must be a live capture", c.rpc, c.fixture)
		}
		got, _ := json.Marshal(c.params)
		if !sameJSON(t, got, fx.Request) {
			t.Errorf("%s builder\n got  %s\n want %s", c.rpc, got, fx.Request)
		}
	}
}

func TestLive2SourceTextDecode(t *testing.T) {
	txt, err := New(newFake(loadFixture(t, "hizoJc", "live_2026-10-20_01"))).SourceText(t.Context(), fakeNB2, fakeSrc1, false)
	if err != nil {
		t.Fatal(err)
	}
	// One line per text block; the capture trimmed the block list to 25.
	if !strings.HasPrefix(txt, "text-4\nWow.\ntext-5\n") || strings.Count(txt, "\n") != 24 {
		t.Fatalf("text = %q", txt)
	}
}
