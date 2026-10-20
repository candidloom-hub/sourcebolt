package nlm

import (
	"context"
	"encoding/json/v2"
	"slices"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Ids in the izAoDd and rLM1Ne fixtures of live capture #2: the notebook,
// the website source and the YouTube source added in one call.
const (
	mixNB      = "00000000-0000-4000-8000-000000000011"
	mixWebID   = "00000000-0000-4000-8000-000000000005"
	mixVideoID = "00000000-0000-4000-8000-000000000010"
	mixWebURL  = "https://example.com/source-3"
	mixVideo   = "https://example.com/source-6" // a YouTube URL before sanitizing
)

// TestAddSourcesLive2Mixed: the web app added a website and a YouTube video
// in one izAoDd call (URL at slot 2, YouTube at slot 7). The answer listed
// only the website; the video was added anyway (it is in the next rLM1Ne
// read, titled by its URL). An item missing from a successful answer is
// therefore looked up in the notebook before it is reported failed.
func TestAddSourcesLive2Mixed(t *testing.T) {
	add := loadFixture(t, "izAoDd", "live_2026-10-20_01")
	got, _ := json.Marshal(addSourcesParams(mixNB, []any{urlSpec(mixWebURL), youtubeSpec(mixVideo)}))
	if !sameJSON(t, got, add.Request) {
		t.Fatalf("params %s\nwant   %s", got, add.Request)
	}
	after := loadFixture(t, "rLM1Ne", "live_2026-10-20_01")
	ft := newFake(add)
	listingAfter(ft, liveRowsWithout(t, after, mixWebID, mixVideoID), after)
	out, err := New(ft).AddSources(context.Background(), mixNB, []SourceSpec{{URL: mixWebURL}, {URL: mixVideo}})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Status != AddAdded || out[0].SourceID != mixWebID {
		t.Fatalf("web = %+v", out[0])
	}
	if out[1].Status != AddAdded || out[1].SourceID != mixVideoID {
		t.Fatalf("video = %+v", out[1])
	}
	if n := countCalls(ft, "izAoDd"); n != 1 {
		t.Fatalf("izAoDd calls = %d, want 1", n)
	}
}

// liveRowsWithout is the rLM1Ne answer of fx without the given sources: the
// notebook as it was before they were added.
func liveRowsWithout(t *testing.T, fx Fixture, ids ...string) Fixture {
	t.Helper()
	frames, _, err := wire.Decode([]byte(fx.ResponseBody))
	if err != nil {
		t.Fatal(err)
	}
	p, err := wire.Pick(frames, "rLM1Ne", wire.NullAllowed)
	if err != nil {
		t.Fatal(err)
	}
	var root []any
	if err := json.Unmarshal(p, &root); err != nil {
		t.Fatal(err)
	}
	nb := root[0].([]any)
	var keep []any
	for _, s := range nb[1].([]any) {
		if !slices.Contains(ids, s.([]any)[0].([]any)[0].(string)) {
			keep = append(keep, s)
		}
	}
	if len(keep) != len(nb[1].([]any))-len(ids) {
		t.Fatalf("fixture lacks %v", ids)
	}
	nb[1] = keep
	b, _ := json.Marshal(root)
	return synthFixture(t, "rLM1Ne", string(b))
}

// listingAfter makes rLM1Ne answer before until the izAoDd call is made,
// and after from then on, like a notebook that gained the added sources.
func listingAfter(ft *fakeTransport, before, after Fixture) {
	ft.fixtures["rLM1Ne"] = before
	ft.hook = func(_ context.Context, c transport.Call) error {
		if c.ID == "izAoDd" {
			ft.mu.Lock()
			ft.fixtures["rLM1Ne"] = after
			ft.mu.Unlock()
		}
		return nil
	}
}

func notebookRows(rows ...string) string {
	return `[["T",[` + strings.Join(rows, ",") + `],"` + nb1 + `"]]`
}

// Review item 1: a spec matching only a source the notebook had before the
// add is not reported added with the old id; it is pending (the request was
// sent, but nothing proves a new source).
func TestAddSourcesOmittedMatchesOldSourceOnly(t *testing.T) {
	old := srcRowJSON("sid-OLD", "Old", "https://example.com/1")
	two := srcRowJSON("sid-2", "Two", "https://example.com/2")
	ft := newFake(synthFixture(t, "izAoDd", `[[`+two+`]]`))
	listingAfter(ft, synthFixture(t, "rLM1Ne", notebookRows(old)), synthFixture(t, "rLM1Ne", notebookRows(old, two)))
	out, err := New(ft).AddSources(context.Background(), nb1, []SourceSpec{{URL: "https://example.com/1"}, {URL: "https://example.com/2"}})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Status != AddPending || out[0].SourceID != "" || out[0].Error == "" || out[1].SourceID != "sid-2" {
		t.Fatalf("out = %+v", out)
	}
}

// Same rule on the code 3/9 (pending) path.
func TestAddSourcesPendingMatchesOldSourceOnly(t *testing.T) {
	old := srcRowJSON("sid-OLD", "Old", "https://example.com/1")
	ft := newFake()
	ft.errs["izAoDd"] = &transport.Error{Kind: transport.KindPending, RPC: "sources.add", GRPCCode: 9}
	listingAfter(ft, synthFixture(t, "rLM1Ne", notebookRows(old)), synthFixture(t, "rLM1Ne", notebookRows(old)))
	out, err := New(ft).AddSources(context.Background(), nb1, []SourceSpec{{URL: "https://example.com/1"}})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Status != AddPending || out[0].SourceID != "" {
		t.Fatalf("out = %+v", out)
	}
}

// Review item 2: every row the answer listed is excluded from the lookup,
// also one that matched no spec.
func TestAddSourcesOmittedSkipsUnmatchedAnswerRows(t *testing.T) {
	ghost := srcRowJSON("sid-X", "Other", "https://example.com/other")
	ft := newFake(synthFixture(t, "izAoDd", `[[`+ghost+`]]`))
	listingAfter(ft, synthFixture(t, "rLM1Ne", notebookRows()),
		synthFixture(t, "rLM1Ne", notebookRows(srcRowJSON("sid-X", "Other", "https://example.com/1"))))
	out, err := New(ft).AddSources(context.Background(), nb1, []SourceSpec{{URL: "https://example.com/1"}, {URL: "https://example.com/2"}})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Status != AddFailed || out[0].SourceID != "" {
		t.Fatalf("out = %+v; sid-X was in the answer and must not settle an omitted spec", out)
	}
}

// A row the answer already gave to one spec is never handed to a missing
// spec, even when titles collide.
func TestAddSourcesOmittedSkipsAnsweredRows(t *testing.T) {
	row := srcRowJSON("sid-2", "Two", "https://example.com/2")
	ft := newFake(synthFixture(t, "izAoDd", `[[`+row+`]]`), synthFixture(t, "rLM1Ne", `[["T",[`+row+`],"`+nb1+`"]]`))
	out, err := New(ft).AddSources(context.Background(), nb1, []SourceSpec{{URL: "https://example.com/2"}, {Text: &TextSpec{Title: "Two", Content: "c"}}})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].SourceID != "sid-2" || out[1].Status != AddFailed || out[1].SourceID != "" {
		t.Fatalf("out = %+v", out)
	}
}

func TestAddSourcesOmittedListFails(t *testing.T) {
	ft := newFake(synthFixture(t, "izAoDd", `[[`+srcRowJSON("sid-2", "Two", "https://example.com/2")+`]]`))
	ft.errs["rLM1Ne"] = &transport.Error{Kind: transport.KindTransport, RPC: "notebooks.get"}
	out, err := New(ft).AddSources(context.Background(), nb1, []SourceSpec{{URL: "https://example.com/1"}, {URL: "https://example.com/2"}})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Status != AddPending || out[0].Error == "" || out[1].Status != AddAdded {
		t.Fatalf("out = %+v", out)
	}
}
