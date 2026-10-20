package nlm

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

// fakeDOM records DOM fallback calls.
type fakeDOM struct {
	mu     sync.Mutex
	asks   []string
	urls   []string
	texts  []string
	answer string
	err    error         // returned by every call
	errFor map[int]error // AddURL/AddText error by call index
	n      int
	// during runs inside Ask (tests check what is held there).
	during func(ctx context.Context)
}

func (f *fakeDOM) Ask(ctx context.Context, nb, question string, progress func(string)) (string, error) {
	f.mu.Lock()
	f.asks = append(f.asks, nb+"|"+question)
	during := f.during
	f.mu.Unlock()
	if during != nil {
		during(ctx)
	}
	if progress != nil && f.err == nil {
		progress(f.answer)
	}
	return f.answer, f.err
}

func (f *fakeDOM) add(err error) error {
	defer func() { f.n++ }()
	if e := f.errFor[f.n]; e != nil {
		return e
	}
	return err
}

func (f *fakeDOM) AddURL(ctx context.Context, nb, url string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.urls = append(f.urls, url)
	return f.add(f.err)
}

func (f *fakeDOM) AddText(ctx context.Context, nb, title, content string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.texts = append(f.texts, title+"|"+content)
	return f.add(f.err)
}

func (f *fakeDOM) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.asks) + len(f.urls) + len(f.texts)
}

func drift(sent bool) *transport.Error {
	return &transport.Error{Kind: transport.KindDrift, RPC: "chat.ask", Transport: "rpc", Sent: sent, Message: "unrecognized stream"}
}

// newDOMClient is a client whose stream fails with streamErr, on a notebook
// with sources (rLM1Ne) and a default thread liveConv (hPTbtc).
func newDOMClient(t *testing.T, streamErr error) (*Client, *fakeStreamer, *fakeDOM, *[]string) {
	t.Helper()
	fs := &fakeStreamer{
		fakeTransport: newFake(loadFixture(t, "rLM1Ne", "live_2026-10-13_03"), loadFixture(t, "hPTbtc", "live_2026-10-13_01")),
		err:           streamErr,
	}
	c := New(fs)
	dom := &fakeDOM{answer: "hi"}
	c.DOM = dom
	var names []string
	c.OnTransport = func(n string) { names = append(names, n) }
	return c, fs, dom, &names
}

func TestAskFallsBackToDOMOnDrift(t *testing.T) {
	c, _, dom, names := newDOMClient(t, drift(false))
	var prog []string
	a, err := c.Ask(context.Background(), liveNB, "q", AskOptions{}, func(s string) { prog = append(prog, s) })
	if err != nil {
		t.Fatal(err)
	}
	if a.Text != "hi" || a.ConversationID != liveConv {
		t.Fatalf("answer = %+v", a)
	}
	if !slices.Equal(dom.asks, []string{liveNB + "|q"}) {
		t.Fatalf("DOM asks = %q", dom.asks)
	}
	if len(*names) == 0 || (*names)[len(*names)-1] != DOMTransport {
		t.Fatalf("OnTransport = %q, want last %q", *names, DOMTransport)
	}
	if !slices.Equal(prog, []string{"hi"}) {
		t.Fatalf("progress = %q", prog)
	}
}

func TestAskNoDOMOnRateLimit(t *testing.T) {
	rl := &transport.Error{Kind: transport.KindRateLimited, RPC: "chat.ask"}
	c, _, dom, _ := newDOMClient(t, rl)
	if _, err := c.Ask(context.Background(), liveNB, "q", AskOptions{}, nil); !errors.Is(err, rl) {
		t.Fatalf("err = %v, want the rate-limit error", err)
	}
	if dom.calls() != 0 {
		t.Fatal("DOM called on a rate limit")
	}
}

func TestAskNoDOMWhenSourcesScoped(t *testing.T) {
	d := drift(false)
	c, _, dom, _ := newDOMClient(t, d)
	if _, err := c.Ask(context.Background(), liveNB, "q", AskOptions{SourceIDs: []string{"s1"}}, nil); !errors.Is(err, d) {
		t.Fatalf("err = %v, want the original drift", err)
	}
	if dom.calls() != 0 {
		t.Fatal("DOM called although sources were scoped")
	}
}

func TestAskDOMOnlyOnDefaultThread(t *testing.T) {
	for _, tc := range []struct {
		name string
		o    AskOptions
		dom  bool
	}{
		{"other thread", AskOptions{ConversationID: "conv-x"}, false},
		{"new thread", AskOptions{NewConversation: true}, false},
		{"default thread by id", AskOptions{ConversationID: liveConv}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := drift(false)
			c, _, dom, _ := newDOMClient(t, d)
			a, err := c.Ask(context.Background(), liveNB, "q", tc.o, nil)
			if tc.dom {
				if err != nil || a.Text != "hi" || a.ConversationID != liveConv {
					t.Fatalf("answer = %+v, err = %v", a, err)
				}
				return
			}
			if !errors.Is(err, d) || dom.calls() != 0 {
				t.Fatalf("err = %v, DOM calls = %d; want the original drift and no DOM", err, dom.calls())
			}
		})
	}
}

// D-T5-1 (owner, yes): a drift after the question was sent still falls back.
func TestAskSentDriftPolicy(t *testing.T) {
	c, _, dom, _ := newDOMClient(t, drift(true))
	a, err := c.Ask(context.Background(), liveNB, "q", AskOptions{}, nil)
	if err != nil || a.Text != "hi" {
		t.Fatalf("answer = %+v, err = %v; want the DOM answer (D-T5-1 yes)", a, err)
	}
	if dom.calls() != 1 {
		t.Fatalf("DOM calls = %d", dom.calls())
	}
}

func TestAskDOMErrorReturned(t *testing.T) {
	c, _, dom, _ := newDOMClient(t, drift(false))
	dom.err = &transport.Error{Kind: transport.KindDrift, RPC: "chat.ask", Transport: DOMTransport, Message: `selector "chat.input" not found`}
	_, err := c.Ask(context.Background(), liveNB, "q", AskOptions{}, nil)
	var te *transport.Error
	if !errors.As(err, &te) || te.Transport != DOMTransport {
		t.Fatalf("err = %v, want the DOM error", err)
	}
}

// A null-id ask (no default thread yet) holds the per-notebook thread lock
// through the DOM fallback, so a concurrent first ask cannot read its thread.
func TestAskDOMHoldsThreadLock(t *testing.T) {
	fs := &fakeStreamer{
		fakeTransport: newFake(loadFixture(t, "rLM1Ne", "live_2026-10-13_03"), synthFixture(t, "hPTbtc", `[[]]`)),
		err:           drift(false),
	}
	c := New(fs)
	held := false
	c.DOM = &fakeDOM{answer: "hi", during: func(context.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		unlock, err := c.newThread.Lock(ctx, liveNB)
		if err == nil {
			unlock()
			return
		}
		held = true
	}}
	if _, err := c.Ask(context.Background(), liveNB, "q", AskOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if !held {
		t.Fatal("the thread lock was not held during the DOM fallback")
	}
}

func addFixtures(t *testing.T) *fakeTransport {
	ft := newFake(synthFixture(t, "rLM1Ne", `[["T",[`+srcRowJSON("sid-A", "Page A", "https://example.com/a")+`,`+srcRowJSON("sid-N", "Notes", "")+`],"`+nb1+`"]]`))
	return ft
}

func TestAddSourcesDOMOnDriftNotSent(t *testing.T) {
	ft := addFixtures(t)
	ft.errs["izAoDd"] = &transport.Error{Kind: transport.KindDrift, RPC: "sources.add"}
	c := New(ft)
	dom := &fakeDOM{}
	c.DOM = dom
	var names []string
	c.OnTransport = func(n string) { names = append(names, n) }
	out, err := c.AddSources(context.Background(), nb1, []SourceSpec{
		{URL: "https://example.com/a"},
		{Text: &TextSpec{Title: "Notes", Content: "body"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(dom.urls, []string{"https://example.com/a"}) || !slices.Equal(dom.texts, []string{"Notes|body"}) {
		t.Fatalf("DOM urls=%q texts=%q", dom.urls, dom.texts)
	}
	want := []AddOutcome{{Index: 0, SourceID: "sid-A", Status: AddAdded}, {Index: 1, SourceID: "sid-N", Status: AddAdded}}
	if !slices.Equal(out, want) {
		t.Fatalf("out = %+v, want %+v", out, want)
	}
	if n := countCalls(ft, "izAoDd"); n != 1 {
		t.Fatalf("izAoDd calls = %d, want 1", n)
	}
	if !slices.Contains(names, DOMTransport) {
		t.Fatalf("OnTransport = %q, want %q", names, DOMTransport)
	}
}

func TestAddSourcesDOMPartialFailure(t *testing.T) {
	ft := addFixtures(t)
	ft.errs["izAoDd"] = &transport.Error{Kind: transport.KindDrift, RPC: "sources.add"}
	c := New(ft)
	c.DOM = &fakeDOM{errFor: map[int]error{
		0: &transport.Error{Kind: transport.KindDrift, RPC: "sources.add", Transport: DOMTransport},
		1: &transport.Error{Kind: transport.KindTransport, RPC: "sources.add", Transport: DOMTransport, Sent: true},
	}}
	out, err := c.AddSources(context.Background(), nb1, []SourceSpec{{URL: "https://example.com/x"}, {URL: "https://example.com/y"}})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Status != AddFailed || out[0].Error == "" || out[1].Status != AddPending || out[1].Error == "" {
		t.Fatalf("out = %+v; want failed (not sent) then pending (sent)", out)
	}
}

func TestAddSourcesNoDOMAfterSend(t *testing.T) {
	ft := addFixtures(t)
	sent := &transport.Error{Kind: transport.KindDrift, RPC: "sources.add", Sent: true}
	ft.errs["izAoDd"] = sent
	c := New(ft)
	dom := &fakeDOM{}
	c.DOM = dom
	if _, err := c.AddSources(context.Background(), nb1, []SourceSpec{{URL: "https://example.com/a"}}); !errors.Is(err, sent) {
		t.Fatalf("err = %v, want the original drift", err)
	}
	if dom.calls() != 0 {
		t.Fatal("DOM called after the add was sent")
	}
}

func TestAddSourcesNoDOMOnOtherKinds(t *testing.T) {
	ft := addFixtures(t)
	ft.errs["izAoDd"] = &transport.Error{Kind: transport.KindTransport, RPC: "sources.add"}
	c := New(ft)
	dom := &fakeDOM{}
	c.DOM = dom
	if _, err := c.AddSources(context.Background(), nb1, []SourceSpec{{URL: "https://example.com/a"}}); err == nil {
		t.Fatal("want the transport error")
	}
	if dom.calls() != 0 {
		t.Fatal("DOM called on a non-drift error")
	}
}

func TestAddSourcesDOMNeverForDrive(t *testing.T) {
	ft := addFixtures(t)
	ft.errs["izAoDd"] = &transport.Error{Kind: transport.KindDrift, RPC: "sources.add"}
	c := New(ft)
	dom := &fakeDOM{}
	c.DOM = dom
	_, err := c.AddSources(context.Background(), nb1, []SourceSpec{{URL: "https://example.com/a"}, {Drive: &DriveSpec{DocumentID: "d1"}}})
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindUnsupported {
		t.Fatalf("err = %v, want KindUnsupported", err)
	}
	if dom.calls() != 0 || len(ft.calls) != 0 {
		t.Fatalf("DOM calls = %d, RPC calls = %d; want none", dom.calls(), len(ft.calls))
	}
}
