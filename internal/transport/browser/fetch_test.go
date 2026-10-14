package browser

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

const okBatch = ")]}'\n\n80\n[[\"wrb.fr\",\"wXbhsf\",\"[[\\\"t\\\"]]\",null,null,null,\"generic\"]]\n"

var signedIn = tokenState{
	Href: "https://notebook.google.com/", Origin: "https://notebook.google.com",
	AT: "AT-fake:1", FSID: "-42", BL: "boq_labs_fake", HL: "en", Email: true,
}

// fakePage records what the transport asks of the tab. Token reads return
// the current state; fetches decode the request the JS would send.
type fakePage struct {
	mu        sync.Mutex
	state     tokenState
	afterNav  tokenState
	navErr    error
	navs      []string
	reqs      []fetchRequest
	reply     fetchReply
	evalCalls int
}

func (p *fakePage) Eval(ctx context.Context, js string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.evalCalls++
	switch {
	case js == tokensJS:
		return json.Marshal(p.state)
	case strings.HasPrefix(js, fetchJS+"("):
		arg := strings.TrimSuffix(strings.TrimPrefix(js, fetchJS+"("), ")")
		var req fetchRequest
		if err := json.Unmarshal([]byte(arg), &req); err != nil {
			return nil, err
		}
		p.reqs = append(p.reqs, req)
		return json.Marshal(p.reply)
	}
	return nil, errors.New("unexpected script: " + js)
}

func (p *fakePage) Navigate(ctx context.Context, u string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.navs = append(p.navs, u)
	if p.navErr != nil {
		return p.navErr
	}
	p.state = p.afterNav
	return nil
}

func newFakeTransport(p *fakePage) *fetchTransport {
	return newFetchTransport(func(ctx context.Context) (page, func(), error) {
		return p, func() {}, nil
	}, session.HostDefault)
}

var listCall = transport.Call{RPC: "notebooks.list", ID: "wXbhsf", Params: []any{nil, 1, nil, []any{2}}, SourcePath: "/", Idempotent: true}

func TestFetchRequestShape(t *testing.T) {
	p := &fakePage{state: signedIn, reply: fetchReply{Status: 200, Body: okBatch}}
	tr := newFakeTransport(p)
	if tr.Name() != "browser" || !tr.Supports("anything") {
		t.Fatalf("Name/Supports")
	}
	res, err := tr.Do(t.Context(), listCall)
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Payload) != `[["t"]]` || res.Transport != "browser" {
		t.Fatalf("result = %s via %q", res.Payload, res.Transport)
	}
	if len(p.navs) != 0 {
		t.Errorf("navigated %v on a signed-in tab", p.navs)
	}
	if len(p.reqs) != 1 {
		t.Fatalf("fetches = %d", len(p.reqs))
	}
	req := p.reqs[0]
	// Same query as the rpc transport (rpc.go query()).
	wantQ := url.Values{}
	wantQ.Set("rpcids", "wXbhsf")
	wantQ.Set("source-path", "/")
	wantQ.Set("f.sid", "-42")
	wantQ.Set("bl", "boq_labs_fake")
	wantQ.Set("hl", "en")
	wantQ.Set("rt", "c")
	wantURL := "https://notebook.google.com/_/LabsTailwindUi/data/batchexecute?" + wantQ.Encode()
	if req.URL != wantURL {
		t.Errorf("url\n got %s\nwant %s", req.URL, wantURL)
	}
	wantBody, _ := wire.EncodeBatch([]wire.BatchItem{{ID: "wXbhsf", Params: listCall.Params}}, "AT-fake:1")
	if req.Body != wantBody {
		t.Errorf("body\n got %s\nwant %s", req.Body, wantBody)
	}
	if req.Method != "POST" || req.Referrer != "https://notebook.google.com/" || req.Credentials != "same-origin" {
		t.Errorf("method %q referrer %q credentials %q", req.Method, req.Referrer, req.Credentials)
	}
	wantH := map[string]string{"Content-Type": "application/x-www-form-urlencoded;charset=UTF-8", "X-Same-Domain": "1"}
	if len(req.Headers) != len(wantH) {
		t.Errorf("headers = %v", req.Headers)
	}
	for k, v := range wantH {
		if req.Headers[k] != v {
			t.Errorf("header %s = %q, want %q", k, req.Headers[k], v)
		}
	}
	if req.TimeoutMS != int64(defaultFetchTimeout/time.Millisecond) {
		t.Errorf("timeout = %d", req.TimeoutMS)
	}
}

func TestFetchHLOverrideAndSourcePath(t *testing.T) {
	p := &fakePage{state: signedIn, reply: fetchReply{Status: 200, Body: okBatch}}
	c := listCall
	c.HL, c.SourcePath = "fr", "/notebook/abc"
	if _, err := newFakeTransport(p).Do(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(p.reqs[0].URL)
	if q := u.Query(); q.Get("hl") != "fr" || q.Get("source-path") != "/notebook/abc" {
		t.Errorf("query = %v", q)
	}
}

func TestFetchNavigatesWhenTabNotOnHost(t *testing.T) {
	p := &fakePage{
		state:    tokenState{Href: "about:blank", Origin: "null"},
		afterNav: signedIn,
		reply:    fetchReply{Status: 200, Body: okBatch},
	}
	if _, err := newFakeTransport(p).Do(t.Context(), listCall); err != nil {
		t.Fatal(err)
	}
	if len(p.navs) != 1 || p.navs[0] != "https://notebook.google.com/" {
		t.Fatalf("navs = %v", p.navs)
	}
}

func TestFetchUsesLandedNotebookHost(t *testing.T) {
	landed := signedIn
	landed.Href, landed.Origin = "https://notebooklm.google.com/", "https://notebooklm.google.com"
	p := &fakePage{state: tokenState{Href: "about:blank"}, afterNav: landed, reply: fetchReply{Status: 200, Body: okBatch}}
	if _, err := newFakeTransport(p).Do(t.Context(), listCall); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.reqs[0].URL, "https://notebooklm.google.com/_/") || p.reqs[0].Referrer != "https://notebooklm.google.com/" {
		t.Errorf("url %s referrer %s", p.reqs[0].URL, p.reqs[0].Referrer)
	}
}

func TestFetchPageStateErrors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state tokenState
		kind  transport.Kind
	}{
		{"sign-in redirect", tokenState{Href: "https://accounts.google.com/v3/signin/identifier", Origin: "https://accounts.google.com"}, transport.KindAuth},
		{"cookie mismatch", tokenState{Href: "https://accounts.google.com/CookieMismatch", Origin: "https://accounts.google.com"}, transport.KindCookieBinding},
		{"signed-out shell", tokenState{Href: "https://notebook.google.com/", Origin: "https://notebook.google.com"}, transport.KindAuth},
		{"tokens missing", tokenState{Href: "https://notebook.google.com/", Origin: "https://notebook.google.com", Email: true}, transport.KindDrift},
		{"foreign host", tokenState{Href: "https://example.com/", Origin: "https://example.com", AT: "x", Email: true}, transport.KindAuth},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &fakePage{state: tokenState{Href: "about:blank"}, afterNav: tc.state}
			_, err := newFakeTransport(p).Do(t.Context(), listCall)
			var te *transport.Error
			if !errors.As(err, &te) || te.Kind != tc.kind || te.Transport != "browser" || te.Sent {
				t.Fatalf("err = %#v, want %s from browser, Sent=false", err, tc.kind)
			}
			if len(p.reqs) != 0 {
				t.Errorf("fetched despite bad page state")
			}
		})
	}
}

func TestFetchResponseErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		reply      fetchReply
		kind       transport.Kind
		retriable  bool
		retryAfter time.Duration
	}{
		{"429", fetchReply{Status: 429, RetryAfter: "7"}, transport.KindRateLimited, true, 7 * time.Second},
		{"401", fetchReply{Status: 401}, transport.KindAuth, false, 0},
		{"503", fetchReply{Status: 503}, transport.KindTransport, true, 0},
		{"404", fetchReply{Status: 404}, transport.KindInternal, false, 0},
		{"network error", fetchReply{Error: "TypeError: Failed to fetch"}, transport.KindTransport, true, 0},
		{"drift", fetchReply{Status: 200, Body: strings.Replace(okBatch, "wXbhsf", "other1", 1)}, transport.KindDrift, false, 0},
		{"empty body", fetchReply{Status: 200, Body: ")]}'\n"}, transport.KindAuth, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &fakePage{state: signedIn, reply: tc.reply}
			_, err := newFakeTransport(p).Do(t.Context(), listCall)
			var te *transport.Error
			if !errors.As(err, &te) {
				t.Fatalf("err = %v", err)
			}
			if te.Kind != tc.kind || te.Retriable != tc.retriable || te.RetryAfter != tc.retryAfter || !te.Sent || te.Transport != "browser" || te.RPC != "notebooks.list" {
				t.Fatalf("err = %#v", te)
			}
		})
	}
}

func TestFetchReloadsAfterAuthOrDrift(t *testing.T) {
	p := &fakePage{state: signedIn, afterNav: signedIn, reply: fetchReply{Status: 401}}
	tr := newFakeTransport(p)
	if _, err := tr.Do(t.Context(), listCall); err == nil {
		t.Fatal("want error")
	}
	p.reply = fetchReply{Status: 200, Body: okBatch}
	if _, err := tr.Do(t.Context(), listCall); err != nil {
		t.Fatal(err)
	}
	if len(p.navs) != 1 {
		t.Fatalf("navs = %v, want one reload after the 401", p.navs)
	}
	if _, err := tr.Do(t.Context(), listCall); err != nil {
		t.Fatal(err)
	}
	if len(p.navs) != 1 {
		t.Fatalf("navs = %v, want no reload on a healthy tab", p.navs)
	}
}

func TestFetchNavigateAndAcquireErrors(t *testing.T) {
	p := &fakePage{state: tokenState{Href: "about:blank"}, navErr: errors.New("net::ERR_NAME_NOT_RESOLVED")}
	_, err := newFakeTransport(p).Do(t.Context(), listCall)
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindTransport || te.Sent {
		t.Fatalf("navigate error = %#v", err)
	}

	unsupported := &transport.Error{Kind: transport.KindUnsupported, Transport: "browser", Message: "no browser"}
	tr := newFetchTransport(func(ctx context.Context) (page, func(), error) { return nil, nil, unsupported }, session.HostDefault)
	if _, err := tr.Do(t.Context(), listCall); !errors.As(err, &te) || te.Kind != transport.KindUnsupported {
		t.Fatalf("acquire error = %#v", err)
	}
	tr = newFetchTransport(func(ctx context.Context) (page, func(), error) { return nil, nil, ErrClosed }, session.HostDefault)
	if _, err := tr.Do(t.Context(), listCall); !errors.As(err, &te) || te.Kind != transport.KindTransport || !errors.Is(err, ErrClosed) {
		t.Fatalf("closed = %#v", err)
	}
}

func TestFetchReleasesTab(t *testing.T) {
	p := &fakePage{state: signedIn, reply: fetchReply{Status: 503}}
	released := 0
	tr := newFetchTransport(func(ctx context.Context) (page, func(), error) {
		return p, func() { released++ }, nil
	}, session.HostDefault)
	tr.Do(t.Context(), listCall)
	p.reply = fetchReply{Status: 200, Body: okBatch}
	tr.Do(t.Context(), listCall)
	if released != 2 {
		t.Fatalf("released = %d, want 2", released)
	}
}

func TestFetchOptions(t *testing.T) {
	o := FetchOptions(filepath.Join("cfg"), "work")
	if o.ProfileDir != filepath.Join("cfg", "browser", "work") || !o.Headless || o.Idle != DefaultIdle {
		t.Fatalf("options = %+v", o)
	}
}

type launchClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *launchClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *launchClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func TestLauncherCachesLaunchFailure(t *testing.T) {
	boom := errors.New("no browser here")
	var calls atomic.Int32
	l := NewLauncher(Options{ProfileDir: t.TempDir()})
	clk := &launchClock{t: time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)}
	l.now = clk.now
	l.launch = func(context.Context, Options) (*Browser, error) {
		calls.Add(1)
		return nil, boom
	}
	defer l.Close()

	if _, err := l.Get(t.Context()); !errors.Is(err, boom) {
		t.Fatalf("first Get: %v", err)
	}
	clk.advance(30 * time.Second)
	_, err := l.Get(t.Context())
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindTransport || !errors.Is(err, boom) {
		t.Fatalf("cached Get: err = %v, want KindTransport wrapping the launch error", err)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("launch calls within the back-off = %d, want 1", n)
	}
	clk.advance(31 * time.Second)
	l.Get(t.Context())
	if n := calls.Load(); n != 2 {
		t.Fatalf("launch calls after the back-off = %d, want 2", n)
	}
}

func TestLauncherStartupDeadline(t *testing.T) {
	l := NewLauncher(Options{ProfileDir: t.TempDir()})
	l.startTimeout = 50 * time.Millisecond
	cancelled := make(chan struct{})
	l.launch = func(ctx context.Context, _ Options) (*Browser, error) {
		<-ctx.Done() // a hung browser start
		close(cancelled)
		return nil, ctx.Err()
	}
	defer l.Close()
	start := time.Now()
	done := make(chan error, 1)
	go func() {
		_, err := l.Get(t.Context())
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Get succeeded on a hung launch")
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Get still waiting after %v: no startup deadline", time.Since(start))
	}
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("the timed-out launch was not cancelled")
	}
}

func TestLauncherCloseDoesNotWaitForHungLaunch(t *testing.T) {
	l := NewLauncher(Options{ProfileDir: t.TempDir()})
	l.startTimeout = time.Hour
	started := make(chan struct{})
	l.launch = func(ctx context.Context, _ Options) (*Browser, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	got := make(chan error, 1)
	go func() {
		_, err := l.Get(context.Background())
		got <- err
	}()
	<-started
	closed := make(chan struct{})
	go func() { l.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close blocked behind a hung launch")
	}
	select {
	case err := <-got:
		if err == nil {
			t.Fatal("Get succeeded after Close")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Get did not return after Close")
	}
	if _, err := l.Get(t.Context()); !errors.Is(err, ErrClosed) {
		t.Fatalf("Get after Close = %v, want ErrClosed", err)
	}
}
