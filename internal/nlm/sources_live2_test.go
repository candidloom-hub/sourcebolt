package nlm

import (
	"context"
	"encoding/json/v2"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
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
	ft := newFake(add, loadFixture(t, "rLM1Ne", "live_2026-10-20_01"))
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
