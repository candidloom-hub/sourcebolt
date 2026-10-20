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

// TestLive2QuotaDecode: capture #2 is the first with quota in use, so both
// percentages are present (capture #1 had null used, 100 left).
func TestLive2QuotaDecode(t *testing.T) {
	fx := loadFixture(t, "EylDcb", "live_2026-10-20_01")
	if got, _ := json.Marshal(quotaParams()); !sameJSON(t, got, fx.Request) {
		t.Fatalf("params %s want %s", got, fx.Request)
	}
	q, err := New(newFake(fx)).Quota(t.Context())
	if err != nil || len(q.Windows) != 2 {
		t.Fatalf("quota = %+v %v", q, err)
	}
	w5, wk := q.Windows[0], q.Windows[1]
	if w5.Type != "5h" || w5.UsedPct < 0.57 || w5.UsedPct > 0.58 || w5.LeftPct < 99.42 || w5.LeftPct > 99.43 || w5.ResetAt.Unix() != 1792508846 {
		t.Fatalf("5h window = %+v", w5)
	}
	if wk.Type != "weekly" || wk.UsedPct < 0.73 || wk.UsedPct > 0.74 || wk.ResetAt.Unix() != 1792537646 {
		t.Fatalf("weekly window = %+v", wk)
	}
}

// TestLive2NotebookDecode reads the rLM1Ne answer of capture #2: six
// sources, one still processing and a YouTube source titled by its URL.
func TestLive2NotebookDecode(t *testing.T) {
	ft := newFake(loadFixture(t, "rLM1Ne", "live_2026-10-20_01"), loadFixture(t, "VfAZjd", "live_2026-10-13_01"))
	d, err := New(ft).GetNotebook(t.Context(), mixNB)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != mixNB || d.Title != "text-1" || len(d.Sources) != 6 {
		t.Fatalf("detail = %+v", d)
	}
	ids := map[string]bool{}
	for _, s := range d.Sources {
		ids[s.ID] = true
	}
	if !ids[mixWebID] || !ids[mixVideoID] {
		t.Fatalf("sources = %+v", d.Sources)
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
