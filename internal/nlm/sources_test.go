package nlm

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

// synthFixture builds a synthetic (not live) fixture whose response frame
// carries payload (a JSON text) for rpc.
func synthFixture(t *testing.T, rpc, payload string) Fixture {
	t.Helper()
	inner, err := json.Marshal(payload) // JSON string literal of the payload
	if err != nil {
		t.Fatal(err)
	}
	frame := fmt.Sprintf(`[["wrb.fr",%q,%s,null,null,null,"generic"]]`, rpc, inner)
	body := fmt.Sprintf(")]}'\n\n%d\n%s\n", len(frame), frame)
	return Fixture{RPC: rpc, Synthetic: true, Source: "synthetic: sources_test.go", ResponseBody: body}
}

func srcRowJSON(sid, title, u string) string {
	if u == "" {
		return fmt.Sprintf(`[["%s"],"%s",[null,1,[1,2],null,5,null,1],[null,2]]`, sid, title)
	}
	return fmt.Sprintf(`[["%s"],"%s",[null,1,[1,2],null,5,null,1,["%s"]],[null,2]]`, sid, title, u)
}

func callParams(t *testing.T, c transport.Call) []byte {
	t.Helper()
	b, err := json.Marshal(c.Params)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func countCalls(ft *fakeTransport, id string) int {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	n := 0
	for _, c := range ft.calls {
		if c.ID == id {
			n++
		}
	}
	return n
}

func TestAddURLLiveShape(t *testing.T) {
	fx := loadFixture(t, "izAoDd", "live_2026-10-13_01")
	ft := newFake(fx)
	nbLive := "00000000-0000-4000-8000-000000000001"
	out, err := New(ft).AddSources(context.Background(), nbLive, []SourceSpec{{URL: "https://example.com/source-4"}})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := ft.callFor("izAoDd")
	if !sameJSON(t, callParams(t, c), fx.Request) {
		t.Fatalf("params = %s\nwant %s", callParams(t, c), fx.Request)
	}
	if c.Idempotent || c.SourcePath != "/notebook/"+nbLive || len(c.PendingCodes) != 2 || c.PendingCodes[0] != 3 || c.PendingCodes[1] != 9 {
		t.Fatalf("call = %+v", c)
	}
	if len(out) != 1 || out[0].Status != AddAdded || out[0].SourceID != "00000000-0000-4000-8000-000000000010" {
		t.Fatalf("out = %+v", out)
	}
}

func TestAddSpecSlots(t *testing.T) {
	ft := newFake(synthFixture(t, "izAoDd", `[[`+srcRowJSON("s1", "t", "")+`]]`))
	specs := []SourceSpec{
		{URL: "https://example.com/a"},
		{URL: "https://www.youtube.com/watch?v=abc"},
		{Text: &TextSpec{Title: "Notes", Content: "hello"}},
	}
	if _, err := New(ft).AddSources(context.Background(), nb1, specs); err != nil {
		t.Fatal(err)
	}
	c, _ := ft.callFor("izAoDd")
	want := `[[
	  [null,null,["https://example.com/a"],null,null,null,null,null,null,null,1],
	  [null,null,null,null,null,null,null,["https://www.youtube.com/watch?v=abc"],null,null,1],
	  [null,["Notes","hello"],null,2,null,null,null,null,null,null,1]
	],"` + nb1 + `",[2,null,[1],[1,null,null,null,null,null,null,null,null,null,[1,3]]]]`
	if !sameJSON(t, callParams(t, c), []byte(want)) {
		t.Fatalf("params = %s", callParams(t, c))
	}
	if n := countCalls(ft, "izAoDd"); n != 1 {
		t.Fatalf("izAoDd calls = %d, want 1 batch", n)
	}
}

func TestAddYouTubeDetection(t *testing.T) {
	for u, want := range map[string]bool{
		"https://www.youtube.com/watch?v=x": true,
		"https://youtu.be/x":                true,
		"https://youtube.com/shorts/x":      true,
		"https://m.youtube.com/watch?v=x":   true,
		"https://youtube.com/channel/x":     false,
		"https://example.com/watch":         false,
		"https://notyoutube.com/watch?v=x":  false,
	} {
		if got := isYouTube(u); got != want {
			t.Errorf("isYouTube(%q) = %v", u, got)
		}
	}
}

func TestAddDriveUnsupported(t *testing.T) {
	ft := newFake()
	_, err := New(ft).AddSources(context.Background(), nb1, []SourceSpec{{URL: "https://example.com/a"}, {Drive: &DriveSpec{DocumentID: "d", MIME: "application/pdf", Title: "x"}}})
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindUnsupported {
		t.Fatalf("err = %v", err)
	}
	if len(ft.calls) != 0 {
		t.Fatalf("calls = %d, want none", len(ft.calls))
	}
}

func TestAddInvalidSpec(t *testing.T) {
	ft := newFake()
	for _, s := range []SourceSpec{{}, {URL: "https://a", Text: &TextSpec{}}} {
		_, err := New(ft).AddSources(context.Background(), nb1, []SourceSpec{s})
		var te *transport.Error
		if !errors.As(err, &te) || te.Kind != transport.KindInvalidArgument {
			t.Fatalf("err = %v", err)
		}
	}
	if len(ft.calls) != 0 {
		t.Fatal("must not call")
	}
}

func TestPendingReconciles(t *testing.T) {
	ft := newFake(loadFixture(t, "VfAZjd", "guide"))
	// The notebook was empty before the add.
	listingAfter(ft, synthFixture(t, "rLM1Ne", notebookRows()),
		synthFixture(t, "rLM1Ne", `[["T",[`+srcRowJSON("sid-A", "Page A", "https://example.com/a/")+`,`+srcRowJSON("sid-N", "Notes", "")+`],"`+nb1+`"]]`))
	ft.errs["izAoDd"] = &transport.Error{Kind: transport.KindPending, RPC: "sources.add", GRPCCode: 9}
	out, err := New(ft).AddSources(context.Background(), nb1, []SourceSpec{
		{URL: "https://example.com/a"},
		{Text: &TextSpec{Title: "Notes", Content: "c"}},
		{URL: "https://example.com/missing"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := countCalls(ft, "izAoDd"); n != 1 {
		t.Fatalf("izAoDd calls = %d, want exactly 1 (never resend)", n)
	}
	if out[0].Status != AddAdded || out[0].SourceID != "sid-A" || out[1].Status != AddAdded || out[1].SourceID != "sid-N" {
		t.Fatalf("out = %+v", out)
	}
	if out[2].Status != AddPending || out[2].Error == "" {
		t.Fatalf("unmatched = %+v", out[2])
	}
}

func TestPendingListFails(t *testing.T) {
	ft := newFake()
	ft.errs["izAoDd"] = &transport.Error{Kind: transport.KindPending, RPC: "sources.add", GRPCCode: 3}
	ft.errs["rLM1Ne"] = &transport.Error{Kind: transport.KindTransport, RPC: "notebooks.get"}
	out, err := New(ft).AddSources(context.Background(), nb1, []SourceSpec{{URL: "https://example.com/a"}})
	if err != nil || len(out) != 1 || out[0].Status != AddPending {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if n := countCalls(ft, "izAoDd"); n != 1 {
		t.Fatalf("izAoDd calls = %d", n)
	}
}

func TestPartialFailure(t *testing.T) {
	ft := newFake(
		synthFixture(t, "izAoDd", `[[`+srcRowJSON("sid-2", "Two", "https://example.com/2")+`]]`),
		// The item missing from the answer is not in the notebook either.
		synthFixture(t, "rLM1Ne", `[["T",[`+srcRowJSON("sid-2", "Two", "https://example.com/2")+`],"`+nb1+`"]]`),
	)
	out, err := New(ft).AddSources(context.Background(), nb1, []SourceSpec{
		{URL: "https://example.com/1"},
		{URL: "https://example.com/2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Status != AddFailed || out[0].Error == "" || out[0].Index != 0 {
		t.Fatalf("first = %+v", out[0])
	}
	if out[1].Status != AddAdded || out[1].SourceID != "sid-2" || out[1].Index != 1 {
		t.Fatalf("second = %+v", out[1])
	}
}

func TestNoRetryAfterSend(t *testing.T) {
	ft := newFake()
	ft.errs["izAoDd"] = &transport.Error{Kind: transport.KindTransport, RPC: "sources.add"}
	out, err := New(ft).AddSources(context.Background(), nb1, []SourceSpec{{URL: "https://example.com/a"}})
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindTransport || out != nil {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	// The snapshot listing before the add, then the add; no reconcile list, no resend.
	if len(ft.calls) != 2 || ft.calls[0].ID != "rLM1Ne" || ft.calls[1].ID != "izAoDd" {
		t.Fatalf("calls = %+v, want snapshot then one add", ft.calls)
	}
	if c, _ := ft.callFor("izAoDd"); c.Idempotent {
		t.Fatal("add must be non-idempotent")
	}
}

func TestListSources(t *testing.T) {
	ft := newFake(loadFixture(t, "rLM1Ne", "get_with_sources"), loadFixture(t, "VfAZjd", "guide"))
	got, err := New(ft).ListSources(context.Background(), nb1)
	if err != nil || len(got) != 2 || got[0].ID != "src-0001" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
