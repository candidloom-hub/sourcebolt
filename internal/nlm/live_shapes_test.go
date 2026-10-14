package nlm

import (
	"encoding/json/v2"
	"testing"
)

// fakeNB is the stable fake id the capture sanitizer gives the test notebook.
const fakeNB = "00000000-0000-4000-8000-000000000001"

// TestLiveRequestShapes pins every payload builder to the request the
// NotebookLM web app actually sent (live capture 2026-10-13, notebook.google.com).
func TestLiveRequestShapes(t *testing.T) {
	cases := []struct {
		rpc, fixture string
		params       any
	}{
		{"wXbhsf", "live_2026-10-13_01", listNotebooksParams()},
		{"rLM1Ne", "live_2026-10-13_01", getNotebookParams(fakeNB)},
		{"VfAZjd", "live_2026-10-13_01", guideParams(fakeNB)},
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

func TestLiveNotebookDecode(t *testing.T) {
	ft := newFake(loadFixture(t, "rLM1Ne", "live_2026-10-13_03"), loadFixture(t, "VfAZjd", "live_2026-10-13_01"))
	d, err := New(ft).GetNotebook(t.Context(), fakeNB)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != fakeNB || len(d.Sources) != 1 || d.Sources[0].ID == "" || d.Created.IsZero() || !d.Owned {
		t.Fatalf("detail = %+v", d)
	}
	list := newFake(loadFixture(t, "wXbhsf", "live_2026-10-13_01"))
	nbs, err := New(list).ListNotebooks(t.Context())
	if err != nil || len(nbs) == 0 || nbs[0].ID != fakeNB || nbs[0].Modified.IsZero() {
		t.Fatalf("list = %+v err=%v", nbs, err)
	}
}
