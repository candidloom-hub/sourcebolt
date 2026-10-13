package rpc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

func freshLogin(t *testing.T, store session.Store, value string) {
	t.Helper()
	p, err := store.Load("test")
	if err != nil {
		t.Fatal(err)
	}
	p.Jar.Replace([]session.Cookie{{Name: "SID", Value: value, Domain: "127.0.0.1", Path: "/"}})
	if err := store.Save(p); err != nil {
		t.Fatal(err)
	}
}

func TestRotationKeepsFreshLogin(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "NID", Value: "n", Path: "/"})
		io.WriteString(w, okBody)
	})
	tr, store := newTransport(t, f)
	freshLogin(t, store, "fresh") // `sourcebolt login` while serve runs
	if _, err := tr.Do(context.Background(), listCall); err != nil {
		t.Fatal(err)
	}
	p, _ := store.Load("test")
	h := p.Jar.HeaderFor(f.host)
	if !strings.Contains(h, "SID=fresh") || !strings.Contains(h, "NID=n") {
		t.Fatalf("stored header = %q: rotation must not resurrect the old SID", h)
	}
	if got := tr.o.Profile.Jar.HeaderFor(f.host); !strings.Contains(got, "SID=fresh") {
		t.Fatalf("running transport must adopt the fresh login, has %q", got)
	}
}

func TestAuthReloadsNewLogin(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Cookie"), "SID=fresh") {
			io.WriteString(w, okBody)
			return
		}
		io.WriteString(w, ")]}'\n16")
	})
	tr, store := newTransport(t, f)
	if err := tr.Warm(context.Background()); err != nil {
		t.Fatal(err)
	}
	freshLogin(t, store, "fresh")
	if _, err := tr.Do(context.Background(), listCall); err != nil {
		t.Fatalf("auth failure must reload the newer profile from disk: %v", err)
	}
}

func TestDialErrorHidesQuery(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		conn.Close()
	})
	tr, _ := newTransport(t, f)
	_, err := tr.Do(context.Background(), listCall)
	if err == nil {
		t.Fatal("want error")
	}
	if s := err.Error(); strings.Contains(s, "f.sid") || strings.Contains(s, "-7712345") || strings.Contains(s, "rpcids") {
		t.Fatalf("error leaks the request query: %s", s)
	}
}

func TestCallTimeout(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	tr, _ := newTransport(t, f)
	tr.o.CallTimeout = 100 * time.Millisecond
	start := time.Now()
	_, err := tr.Do(context.Background(), listCall)
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindTransport {
		t.Fatalf("got %v", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("stalled call took %v", d)
	}
}

func TestBootstrapDoesNotHoldLock(t *testing.T) {
	release := make(chan struct{})
	got := make(chan struct{}, 1)
	f := newFake(t, nil)
	f.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- struct{}{}
		<-release
	})
	tr, _ := newTransport(t, f)
	defer close(release)
	go tr.Warm(context.Background())
	<-got
	done := make(chan struct{})
	go func() { tr.setFSID("x"); close(done) }()
	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("a stalled bootstrap blocks every other transport operation")
	}
}

type failingStore struct{ session.Store }

func (failingStore) Save(*session.Profile) error { return errors.New("disk full") }

func TestSaveErrorReported(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "NID", Value: "n", Path: "/"})
		io.WriteString(w, okBody)
	})
	tr, store := newTransport(t, f)
	tr.o.Store = failingStore{store}
	var mu sync.Mutex
	var reported []error
	tr.o.OnSaveError = func(err error) { mu.Lock(); reported = append(reported, err); mu.Unlock() }
	if _, err := tr.Do(context.Background(), listCall); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(reported) == 0 {
		t.Fatal("a failed profile save must be reported, not swallowed")
	}
}

func TestConcurrentCallsDuringRotation(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "SIDCC", Value: strings.Repeat("x", n%5+1), Path: "/"})
		io.WriteString(w, okBody)
	})
	tr, _ := newTransport(t, f)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := tr.Do(context.Background(), listCall); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}
