//go:build browser

package browser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// fakeBatch serves a page with fake WIZ_global_data tokens and a fake
// batchexecute endpoint that records each request. Fake values only.
type fakeBatch struct {
	srv      *httptest.Server
	mu       sync.Mutex
	reqs     []*http.Request
	bodies   []string
	inFlight atomic.Int32
	maxIn    atomic.Int32
	delay    time.Duration
}

func newFakeBatch(t *testing.T, delay time.Duration) *fakeBatch {
	t.Helper()
	f := &fakeBatch{delay: delay}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "SID", Value: "fake-sid", Path: "/"})
		fmt.Fprint(w, `<html lang="en"><body>app<script>window.WIZ_global_data = {"SNlM0e":"fake-at:1","FdrFJe":"-7","cfb2h":"boq_fake_bl","oPEP7c":"user@example.test"};</script></body></html>`)
	})
	mux.HandleFunc(batchPath, func(w http.ResponseWriter, r *http.Request) {
		n := f.inFlight.Add(1)
		defer f.inFlight.Add(-1)
		for {
			m := f.maxIn.Load()
			if n <= m || f.maxIn.CompareAndSwap(m, n) {
				break
			}
		}
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.reqs, f.bodies = append(f.reqs, r.Clone(context.Background())), append(f.bodies, string(body))
		f.mu.Unlock()
		time.Sleep(f.delay)
		io.WriteString(w, okBatch)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeBatch) host() session.Host {
	return session.Host(strings.TrimPrefix(f.srv.URL, "http://"))
}

// newLocalTransport builds the real transport (Launcher + tab acquirer)
// pointed at the fake server, counting browser launches.
func newLocalTransport(t *testing.T, f *fakeBatch, idle time.Duration) (*fetchTransport, *Launcher, *atomic.Int32) {
	t.Helper()
	o := Options{ProfileDir: profileDir(t), Headless: true, Idle: idle}
	l := NewLauncher(o)
	var launches atomic.Int32
	l.launch = func(ctx context.Context, o Options) (*Browser, error) {
		launches.Add(1)
		return Launch(ctx, o)
	}
	t.Cleanup(func() { l.Close() })
	tr := newFetchTransport(tabAcquirer(l.Get), f.host())
	tr.origin = f.srv.URL
	return tr, l, &launches
}

func TestFetchLocalRequestMatchesRPC(t *testing.T) {
	f := newFakeBatch(t, 0)
	tr, _, _ := newLocalTransport(t, f, 0)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	c := transport.Call{RPC: "notebooks.list", ID: "wXbhsf", Params: []any{nil, 1, nil, []any{2}}, SourcePath: "/", Idempotent: true}
	res, err := tr.Do(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Payload) != `[["t"]]` || res.Transport != "browser" {
		t.Fatalf("result %s via %q", res.Payload, res.Transport)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.reqs) != 1 {
		t.Fatalf("requests = %d", len(f.reqs))
	}
	r := f.reqs[0]
	if r.Method != http.MethodPost {
		t.Errorf("method %s", r.Method)
	}
	wantQ := url.Values{"rpcids": {"wXbhsf"}, "source-path": {"/"}, "f.sid": {"-7"}, "bl": {"boq_fake_bl"}, "hl": {"en"}, "rt": {"c"}}
	if r.URL.RawQuery != wantQ.Encode() {
		t.Errorf("query\n got %s\nwant %s", r.URL.RawQuery, wantQ.Encode())
	}
	wantBody, _ := wire.EncodeBatch([]wire.BatchItem{{ID: c.ID, Params: c.Params}}, "fake-at:1")
	if f.bodies[0] != wantBody {
		t.Errorf("body\n got %s\nwant %s", f.bodies[0], wantBody)
	}
	for k, v := range map[string]string{
		"Content-Type":  "application/x-www-form-urlencoded;charset=UTF-8",
		"X-Same-Domain": "1",
		"Origin":        f.srv.URL,
		"Referer":       f.srv.URL + "/",
		"Cookie":        "SID=fake-sid",
	} {
		if got := r.Header.Get(k); got != v {
			t.Errorf("header %s = %q, want %q", k, got, v)
		}
	}
}

func TestFetchLocalConcurrentCallersShareOneBrowser(t *testing.T) {
	f := newFakeBatch(t, 200*time.Millisecond)
	tr, _, launches := newLocalTransport(t, f, 0)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	c := transport.Call{RPC: "notebooks.list", ID: "wXbhsf", Params: []any{nil, 1}, Idempotent: true}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range 2 {
		wg.Go(func() { _, errs[i] = tr.Do(ctx, c) })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("caller %d: %v", i, err)
		}
	}
	if n := launches.Load(); n != 1 {
		t.Errorf("browser launches = %d, want 1", n)
	}
	if n := len(f.bodies); n != 2 {
		t.Errorf("requests = %d, want 2", n)
	}
	if m := f.maxIn.Load(); m != 1 {
		t.Errorf("max in-flight = %d, want 1 (per-tab FIFO lock)", m)
	}
}

func TestFetchLocalRelaunchAfterIdleReap(t *testing.T) {
	f := newFakeBatch(t, 0)
	tr, l, launches := newLocalTransport(t, f, 500*time.Millisecond)
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	c := transport.Call{RPC: "notebooks.list", ID: "wXbhsf", Params: []any{nil, 1}, Idempotent: true}
	if _, err := tr.Do(ctx, c); err != nil {
		t.Fatal(err)
	}
	b, err := l.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-b.Done():
	case <-time.After(20 * time.Second):
		t.Fatal("idle browser was not reaped")
	}
	if _, err := tr.Do(ctx, c); err != nil {
		t.Fatal(err)
	}
	if n := launches.Load(); n != 2 {
		t.Errorf("launches = %d, want 2 (relaunch after reap)", n)
	}
}
