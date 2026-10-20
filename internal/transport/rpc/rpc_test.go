package rpc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/zalando/go-keyring"
	"golang.org/x/time/rate"
)

const okBody = ")]}'\n\n80\n[[\"wrb.fr\",\"wXbhsf\",\"[[\\\"t\\\"]]\",null,null,null,\"generic\"]]\n"

type fake struct {
	srv   *httptest.Server
	host  session.Host
	posts atomic.Int32
	mu    sync.Mutex
	reqs  []*http.Request
	forms []url.Values
	reply func(n int, w http.ResponseWriter, r *http.Request)
}

func newFake(t *testing.T, reply func(n int, w http.ResponseWriter, r *http.Request)) *fake {
	t.Helper()
	page, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "_page", "home_dq.html"))
	if err != nil {
		t.Fatal(err)
	}
	f := &fake{reply: reply}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Write(page)
			return
		}
		n := int(f.posts.Add(1))
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		f.mu.Lock()
		f.reqs, f.forms = append(f.reqs, r.Clone(context.Background())), append(f.forms, form)
		f.mu.Unlock()
		f.reply(n, w, r)
	}))
	t.Cleanup(f.srv.Close)
	f.host = session.Host(f.srv.Listener.Addr().String())
	return f
}

func newTransport(t *testing.T, f *fake) (*Transport, session.Store) {
	t.Helper()
	keyring.MockInit()
	store := session.NewFileStore(t.TempDir(), session.OSKeyring{})
	jar := session.NewJar()
	jar.Add(session.Cookie{Name: "SID", Value: "s", Domain: "127.0.0.1", Path: "/"})
	p := &session.Profile{Name: "test", Host: f.host, Jar: jar}
	if err := store.Save(p); err != nil {
		t.Fatal(err)
	}
	tr := New(Options{
		Profile:    p,
		Store:      store,
		HTTP:       f.srv.Client(),
		Limiter:    rate.NewLimiter(rate.Inf, 1),
		MaxRetries: 2,
		Sleep:      func(ctx context.Context, d time.Duration) error { return ctx.Err() },
	})
	return tr, store
}

var listCall = transport.Call{RPC: "notebooks.list", ID: "wXbhsf", Params: []any{nil, 1, nil, []any{2}}, SourcePath: "/", Idempotent: true}

func TestQueryParams(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) { io.WriteString(w, okBody) })
	tr, _ := newTransport(t, f)
	res, err := tr.Do(context.Background(), listCall)
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Payload) != `[["t"]]` || res.Transport != "rpc" {
		t.Fatalf("result %+v", res)
	}
	r := f.reqs[0]
	q := r.URL.Query()
	for k, want := range map[string]string{"rpcids": "wXbhsf", "source-path": "/", "f.sid": "-7712345", "bl": "boq_labs-tailwind-frontend_20261008.06_p0", "hl": "en-GB", "rt": "c"} {
		if q.Get(k) != want {
			t.Errorf("query %s = %q, want %q", k, q.Get(k), want)
		}
	}
	if r.URL.Path != "/_/LabsTailwindUi/data/batchexecute" {
		t.Errorf("path %s", r.URL.Path)
	}
	origin := "https://" + string(f.host)
	if r.Header.Get("X-Same-Domain") != "1" || r.Header.Get("Origin") != origin || !strings.HasPrefix(r.Header.Get("Referer"), origin) {
		t.Errorf("headers %v", r.Header)
	}
	if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded;charset=UTF-8" {
		t.Errorf("content-type %q", r.Header.Get("Content-Type"))
	}
	if !strings.Contains(r.Header.Get("Cookie"), "SID=s") {
		t.Errorf("cookie %q", r.Header.Get("Cookie"))
	}
	if f.forms[0].Get("at") != "AJpMv9x:1760400000000" {
		t.Errorf("at = %q", f.forms[0].Get("at"))
	}
}

func TestFSIDRotates(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, strings.Replace(okBody, "\"generic\"]]", "\"generic\"],[\"af.httprm\",9,\"-NEW\",3]]", 1))
	})
	tr, _ := newTransport(t, f)
	tr.Do(context.Background(), listCall)
	tr.Do(context.Background(), listCall)
	if got := f.reqs[1].URL.Query().Get("f.sid"); got != "-NEW" {
		t.Fatalf("second f.sid = %q", got)
	}
}

func TestRetryIdempotentOnly(t *testing.T) {
	flaky := func(n int, w http.ResponseWriter, r *http.Request) {
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		io.WriteString(w, okBody)
	}
	f := newFake(t, flaky)
	tr, _ := newTransport(t, f)
	if _, err := tr.Do(context.Background(), listCall); err != nil {
		t.Fatalf("idempotent: %v", err)
	}
	if f.posts.Load() != 2 {
		t.Fatalf("idempotent posts = %d", f.posts.Load())
	}

	f2 := newFake(t, flaky)
	tr2, _ := newTransport(t, f2)
	write := listCall
	write.Idempotent = false
	_, err := tr2.Do(context.Background(), write)
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindTransport {
		t.Fatalf("non-idempotent: %v", err)
	}
	if f2.posts.Load() != 1 {
		t.Fatalf("non-idempotent posts = %d", f2.posts.Load())
	}
}

func TestRetryAfter(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	tr, _ := newTransport(t, f)
	_, err := tr.Do(context.Background(), listCall)
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindRateLimited || !te.Retriable || te.RetryAfter != time.Second {
		t.Fatalf("got %+v", err)
	}
	if f.posts.Load() != 1 {
		t.Fatalf("rate limits must not auto-retry: posts = %d", f.posts.Load())
	}
}

func TestAuthRefetchOnce(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, ")]}'\n16")
	})
	tr, _ := newTransport(t, f)
	_, err := tr.Do(context.Background(), listCall)
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindAuth {
		t.Fatalf("got %v", err)
	}
	if f.posts.Load() != 2 {
		t.Fatalf("posts = %d, want 2 (one re-bootstrap)", f.posts.Load())
	}
}

func TestSetCookieMerged(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "SIDCC", Value: "new", Path: "/"})
		io.WriteString(w, okBody)
	})
	tr, store := newTransport(t, f)
	if _, err := tr.Do(context.Background(), listCall); err != nil {
		t.Fatal(err)
	}
	p, err := store.Load("test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Jar.HeaderFor(f.host), "SIDCC=new") || p.Rev < 2 {
		t.Fatalf("saved profile rev=%d header=%q", p.Rev, p.Jar.HeaderFor(f.host))
	}
}

func TestCtxCancel(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	tr, _ := newTransport(t, f)
	if err := tr.Warm(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)
	start := time.Now()
	_, err := tr.Do(ctx, listCall)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if d := time.Since(start); d > 300*time.Millisecond {
		t.Fatalf("cancel took %v", d)
	}
}

// TestCtxCancelBeatsBody: when the caller cancels while the answer is being
// read, Do reports the cancel, not what the cut-short body decodes to (an
// empty body reads as AUTH_REQUIRED, which would send the user to sign in).
func TestCtxCancelBeatsBody(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		// Headers first, so the client is already reading the body when
		// the cancel lands; the body then ends empty.
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(10 * time.Millisecond)
		cancel()
	})
	tr, _ := newTransport(t, f)
	if err := tr.Warm(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err := tr.Do(ctx, listCall)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	var te *transport.Error
	if !errors.As(err, &te) || !te.Sent {
		t.Fatalf("want a *transport.Error with Sent set, got %#v", err)
	}
}

// TestSentFlag: Error.Sent is true only when the request reached the
// server, and survives a later failed re-bootstrap.
func TestSentFlag(t *testing.T) {
	addCall := transport.Call{RPC: "sources.add", ID: "izAoDd", Params: []any{"x"}, SourcePath: "/"}
	t.Run("HTTP 500 after the body was sent", func(t *testing.T) {
		f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
		tr, _ := newTransport(t, f)
		_, err := tr.Do(context.Background(), addCall)
		var te *transport.Error
		if !errors.As(err, &te) || te.Kind != transport.KindTransport || !te.Sent {
			t.Fatalf("err = %#v, want TRANSPORT with Sent", err)
		}
	})
	t.Run("bootstrap failure before sending", func(t *testing.T) {
		f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) { io.WriteString(w, okBody) })
		f.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) })
		tr, _ := newTransport(t, f)
		_, err := tr.Do(context.Background(), addCall)
		var te *transport.Error
		if !errors.As(err, &te) || te.Sent {
			t.Fatalf("err = %#v, want Sent=false", err)
		}
		if f.posts.Load() != 0 {
			t.Fatalf("posts = %d", f.posts.Load())
		}
	})
	t.Run("sent drift then failed re-bootstrap stays Sent", func(t *testing.T) {
		// okBody answers wXbhsf, so the izAoDd call decodes as drift.
		f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) { io.WriteString(w, okBody) })
		inner := f.srv.Config.Handler
		var gets atomic.Int32
		f.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" && gets.Add(1) > 1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			inner.ServeHTTP(w, r)
		})
		tr, _ := newTransport(t, f)
		_, err := tr.Do(context.Background(), addCall)
		var te *transport.Error
		if !errors.As(err, &te) || !te.Sent {
			t.Fatalf("err = %#v, want Sent=true (first attempt reached the server)", err)
		}
		if f.posts.Load() != 1 {
			t.Fatalf("posts = %d, want 1", f.posts.Load())
		}
	})
}
