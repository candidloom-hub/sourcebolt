package rpc

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

const heartbeat = "22\n[[\"wrb.fr\",null,\"[]\"]]\n"

func liveStream(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "GenerateFreeFormStreamed", "live_2026-10-13_01.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Body string `json:"response_body"`
	}
	if err := json.Unmarshal(b, &fx); err != nil {
		t.Fatal(err)
	}
	return fx.Body
}

func streamFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "_stream", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// writeChunks sends body in n pieces, flushing and pausing between them.
func writeChunks(w http.ResponseWriter, body string, n int, pause time.Duration) {
	fl := w.(http.Flusher)
	step := (len(body) + n - 1) / n
	for i := 0; i < len(body); i += step {
		end := min(i+step, len(body))
		io.WriteString(w, body[i:end])
		fl.Flush()
		time.Sleep(pause)
	}
}

func streamErr(t *testing.T, err error) *transport.Error {
	t.Helper()
	var te *transport.Error
	if !errors.As(err, &te) {
		t.Fatalf("want *transport.Error, got %T %v", err, err)
	}
	return te
}

var streamParams = []any{[]any{[]any{[]any{"s1"}}}, "q?", nil}

func TestStreamHappyPath(t *testing.T) {
	body := liveStream(t)
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeChunks(w, body, 5, 5*time.Millisecond)
	})
	tr, _ := newTransport(t, f)
	var snaps []wire.Snapshot
	final, err := tr.Stream(context.Background(), "nb1", streamParams, func(s wire.Snapshot) { snaps = append(snaps, s) })
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 9 {
		t.Fatalf("snapshots = %d, want 9", len(snaps))
	}
	if !final.Final || final.Text != snaps[8].Text || final.Text == "" {
		t.Fatalf("final = %+v", final)
	}
	r := f.reqs[0]
	if r.URL.Path != streamPath {
		t.Errorf("path %s", r.URL.Path)
	}
	q := r.URL.Query()
	for k, want := range map[string]string{"f.sid": "-7712345", "bl": "boq_labs-tailwind-frontend_20261008.06_p0", "hl": "en-GB", "rt": "c"} {
		if q.Get(k) != want {
			t.Errorf("query %s = %q, want %q", k, q.Get(k), want)
		}
	}
	if q.Has("rpcids") {
		t.Errorf("stream must not send rpcids")
	}
	if _, err := strconv.Atoi(q.Get("_reqid")); err != nil {
		t.Errorf("_reqid = %q", q.Get("_reqid"))
	}
	if r.Header.Get("X-Same-Domain") != "1" || !strings.Contains(r.Header.Get("Cookie"), "SID=s") {
		t.Errorf("headers %v", r.Header)
	}
	if f.forms[0].Get("at") != "AJpMv9x:1760400000000" {
		t.Errorf("at = %q", f.forms[0].Get("at"))
	}
	if got := f.forms[0].Get("f.req"); got != `[null,"[[[[\"s1\"]]],\"q?\",null]"]` {
		t.Errorf("f.req = %s", got)
	}

	// _reqid advances; the rotated f.sid from the trailer is used next.
	if _, err := tr.Stream(context.Background(), "nb1", streamParams, nil); err != nil {
		t.Fatal(err)
	}
	q2 := f.reqs[1].URL.Query()
	a, _ := strconv.Atoi(q.Get("_reqid"))
	b, _ := strconv.Atoi(q2.Get("_reqid"))
	if b <= a {
		t.Errorf("_reqid %d then %d", a, b)
	}
	if q2.Get("f.sid") != "-000000000000000021" {
		t.Errorf("second f.sid = %q", q2.Get("f.sid"))
	}
}

func TestStreamIdleTimeout(t *testing.T) {
	body := liveStream(t)
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, body[:len(body)/2])
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	tr, _ := newTransport(t, f)
	tr.o.StreamIdle = 60 * time.Millisecond
	start := time.Now()
	got := 0
	_, err := tr.Stream(context.Background(), "nb1", streamParams, func(wire.Snapshot) { got++ })
	te := streamErr(t, err)
	if te.Kind != transport.KindTransport || !strings.Contains(te.Error(), "idle") {
		t.Fatalf("err = %v", err)
	}
	if got == 0 {
		t.Errorf("snapshots before the stall must be delivered")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("idle timeout took %v", d)
	}
}

func TestStreamFirstChunkTimeout(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush() // headers only
		<-r.Context().Done()
	})
	tr, _ := newTransport(t, f)
	tr.o.StreamFirstChunk = 60 * time.Millisecond
	_, err := tr.Stream(context.Background(), "nb1", streamParams, nil)
	te := streamErr(t, err)
	if te.Kind != transport.KindTransport || !strings.Contains(te.Error(), "first chunk") {
		t.Fatalf("err = %v", err)
	}
}

func TestStreamTotalCap(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, ")]}'\n\n")
		for r.Context().Err() == nil {
			io.WriteString(w, heartbeat)
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
		}
	})
	tr, _ := newTransport(t, f)
	tr.o.StreamIdle = time.Second
	tr.o.StreamTotal = 150 * time.Millisecond
	start := time.Now()
	_, err := tr.Stream(context.Background(), "nb1", streamParams, nil)
	te := streamErr(t, err)
	if te.Kind != transport.KindTransport || !strings.Contains(te.Error(), "total") {
		t.Fatalf("err = %v", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("total cap took %v", d)
	}
}

func TestStreamCancelMidStream(t *testing.T) {
	body := liveStream(t)
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, body[:len(body)/3])
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	tr, _ := newTransport(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	_, err := tr.Stream(ctx, "nb1", streamParams, func(wire.Snapshot) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if f.posts.Load() != 1 {
		t.Fatalf("posts = %d", f.posts.Load())
	}
}

func TestStreamQuotaFrame(t *testing.T) {
	body := streamFile(t, "quota_rejection.txt")
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) })
	tr, _ := newTransport(t, f)
	_, err := tr.Stream(context.Background(), "nb1", streamParams, nil)
	te := streamErr(t, err)
	if te.Kind != transport.KindQuota || te.GRPCCode != 8 || te.Message == "" {
		t.Fatalf("err = %+v", te)
	}
	if f.posts.Load() != 1 {
		t.Fatalf("posts = %d", f.posts.Load())
	}
}

func TestStreamNoRetryAfterSend(t *testing.T) {
	for name, reply := range map[string]func(n int, w http.ResponseWriter, r *http.Request){
		"5xx": func(n int, w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) },
		"cut": func(n int, w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, ")]}'\n\n500\n[[\"wrb.fr\",null,\"[[")
			w.(http.Flusher).Flush()
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				conn.Close()
			}
		},
		"auth": func(n int, w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFake(t, reply)
			tr, _ := newTransport(t, f)
			_, err := tr.Stream(context.Background(), "nb1", streamParams, nil)
			if err == nil {
				t.Fatal("want error")
			}
			streamErr(t, err)
			if f.posts.Load() != 1 {
				t.Fatalf("posts = %d, want 1 (no retry after send)", f.posts.Load())
			}
		})
	}
}

func TestStreamCookieMerged(t *testing.T) {
	body := liveStream(t)
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "SIDCC", Value: "rot", Path: "/"})
		io.WriteString(w, body)
	})
	tr, store := newTransport(t, f)
	if _, err := tr.Stream(context.Background(), "nb1", streamParams, nil); err != nil {
		t.Fatal(err)
	}
	p, err := store.Load("test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Jar.HeaderFor(f.host), "SIDCC=rot") {
		t.Fatalf("saved header = %q", p.Jar.HeaderFor(f.host))
	}
}

func TestStreamAnswerCap(t *testing.T) {
	body := liveStream(t)
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) })
	tr, _ := newTransport(t, f)
	tr.o.StreamMaxAnswer = 4 // live texts are skeletonized to "text-N"
	_, err := tr.Stream(context.Background(), "nb1", streamParams, nil)
	te := streamErr(t, err)
	if !strings.Contains(te.Error(), "cap") {
		t.Fatalf("err = %v", err)
	}
}

func TestStreamSignedOut(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) { io.WriteString(w, "<html>sign in</html>") })
	tr, _ := newTransport(t, f)
	_, err := tr.Stream(context.Background(), "nb1", streamParams, nil)
	if te := streamErr(t, err); te.Kind != transport.KindAuth {
		t.Fatalf("err = %v", err)
	}
}
