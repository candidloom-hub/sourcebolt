package rpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/candidloom-hub/sourcebolt/bench/benchdata"
	"github.com/candidloom-hub/sourcebolt/internal/session"
	"golang.org/x/time/rate"
)

// BenchmarkDoList40KB is the RPC client's own overhead per call: one
// batchexecute round trip over HTTP/2 + TLS to an in-process server that
// answers a ~40 KB notebook list at once (request build, headers, body
// read, decode). The server's share is in the profile too.
func BenchmarkDoList40KB(b *testing.B) {
	page, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "_page", "home_dq.html"))
	if err != nil {
		b.Fatal(err)
	}
	body, _ := benchdata.ListBodyOfSize(40_000)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Write(page)
			return
		}
		w.Write(body)
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	b.Cleanup(srv.Close)
	jar := session.NewJar()
	jar.Add(session.Cookie{Name: "SID", Value: "s", Domain: "127.0.0.1", Path: "/"})
	p := &session.Profile{Name: "bench", Host: session.Host(srv.Listener.Addr().String()), Jar: jar}
	tr := New(Options{Profile: p, HTTP: srv.Client(), Limiter: rate.NewLimiter(rate.Inf, 1)})
	ctx := context.Background()
	if _, err := tr.Do(ctx, listCall); err != nil { // bootstrap tokens, connect
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := tr.Do(ctx, listCall); err != nil {
			b.Fatal(err)
		}
	}
}
