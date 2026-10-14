package rpc

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/zalando/go-keyring"
	"golang.org/x/time/rate"
)

// TestConnectionReuse is B6: over 50 sequential calls (plus the one-time
// token bootstrap) to an HTTP/2 TLS server, every request after the first
// must reuse the pooled connection (target > 95%) and only one TLS
// handshake may happen.
func TestConnectionReuse(t *testing.T) {
	keyring.MockInit()
	page, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "_page", "home_dq.html"))
	if err != nil {
		t.Fatal(err)
	}
	var h2 atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 {
			h2.Add(1)
		}
		if r.URL.Path == "/" {
			w.Write(page)
			return
		}
		w.Write([]byte(okBody))
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)

	host := session.Host(srv.Listener.Addr().String())
	jar := session.NewJar()
	jar.Add(session.Cookie{Name: "SID", Value: "s", Domain: "127.0.0.1", Path: "/"})
	p := &session.Profile{Name: "bench", Host: host, Jar: jar}
	tr := New(Options{
		Profile: p,
		HTTP:    srv.Client(),
		Limiter: rate.NewLimiter(rate.Inf, 1),
		Sleep:   func(ctx context.Context, d time.Duration) error { return ctx.Err() },
	})

	var gotConn, reused, handshakes atomic.Int32
	trace := &httptrace.ClientTrace{
		GotConn: func(i httptrace.GotConnInfo) {
			gotConn.Add(1)
			if i.Reused {
				reused.Add(1)
			}
		},
		TLSHandshakeDone: func(tls.ConnectionState, error) { handshakes.Add(1) },
	}
	ctx := httptrace.WithClientTrace(context.Background(), trace)

	const calls = 50
	for i := range calls {
		if _, err := tr.Do(ctx, transport.Call{RPC: "notebooks.list", ID: "wXbhsf", Params: []any{}, SourcePath: "/", Idempotent: true}); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}

	total, re := int(gotConn.Load()), int(reused.Load())
	if total < calls+1 { // 50 POSTs + the bootstrap GET
		t.Fatalf("saw %d requests, want at least %d", total, calls+1)
	}
	ratio := float64(re) / float64(total-1) // the first request cannot reuse anything
	t.Logf("requests=%d reused=%d reuse-after-first=%.1f%% tls-handshakes=%d http2-requests=%d",
		total, re, ratio*100, handshakes.Load(), h2.Load())
	if ratio <= 0.95 {
		t.Errorf("connection reuse after first request = %.1f%%, want > 95%%", ratio*100)
	}
	if n := handshakes.Load(); n != 1 {
		t.Errorf("TLS handshakes = %d, want 1", n)
	}
	if int(h2.Load()) != total {
		t.Errorf("HTTP/2 requests = %d of %d", h2.Load(), total)
	}
}
