package rpc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

// The shared client must not cap response headers: a chat stream may wait
// minutes for its first byte (StreamFirstChunk), and a client-wide
// ResponseHeaderTimeout would cut it first. Unary calls get their header
// deadline per request (Options.HeaderTimeout).
func TestNewHTTPClientHasNoHeaderTimeout(t *testing.T) {
	tr, ok := NewHTTPClient().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport is %T", NewHTTPClient().Transport)
	}
	if tr.ResponseHeaderTimeout != 0 {
		t.Fatalf("ResponseHeaderTimeout = %v, want 0", tr.ResponseHeaderTimeout)
	}
}

func TestHeaderTimeout(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	tr, _ := newTransport(t, f)
	tr.o.HeaderTimeout = 100 * time.Millisecond
	tr.o.CallTimeout = 10 * time.Second
	tr.o.MaxRetries = 0
	start := time.Now()
	_, err := tr.Do(context.Background(), listCall)
	te := streamErr(t, err)
	if te.Kind != transport.KindTransport || !te.Retriable || !strings.Contains(te.Error(), "response headers") {
		t.Fatalf("err = %v (retriable %v)", err, te.Retriable)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("header wait took %v, want about HeaderTimeout", d)
	}
}

// Headers that arrive after a body has started must not trip the header
// timer: it stops once the response arrives.
func TestHeaderTimeoutStopsAtResponse(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte(okBody))
	})
	tr, _ := newTransport(t, f)
	tr.o.HeaderTimeout = 50 * time.Millisecond
	if _, err := tr.Do(context.Background(), listCall); err != nil {
		t.Fatal(err)
	}
}

func TestStreamIgnoresHeaderTimeout(t *testing.T) {
	body := liveStream(t)
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // slow to answer, like a long think
		writeChunks(w, body, 3, time.Millisecond)
	})
	tr, _ := newTransport(t, f)
	tr.o.HeaderTimeout = 50 * time.Millisecond
	final, err := tr.Stream(context.Background(), "nb1", streamParams, nil)
	if err != nil {
		t.Fatalf("stream cut by the unary header timeout: %v", err)
	}
	if !final.Final || final.Text == "" {
		t.Fatalf("final = %+v", final)
	}
}

// The router skips the browser fallback on a refused connection; that only
// works if the rpc error keeps the errno through fail and stripURL.
func TestRefusedDialIsRecognised(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {})
	tr, _ := newTransport(t, f)
	tr.o.MaxRetries = 0
	f.srv.Close()
	_, err := tr.Do(context.Background(), listCall)
	te := streamErr(t, err)
	if te.Kind != transport.KindTransport || !transport.ConnRefused(err) {
		t.Fatalf("err = %v, ConnRefused = %v", err, transport.ConnRefused(err))
	}
}

// Once the question reached Google it may be answered and billed, so every
// stream error after the body was written must say Sent.
func TestStreamErrorsAfterSendAreSent(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	tr, _ := newTransport(t, f)
	tr.o.StreamFirstChunk = 60 * time.Millisecond
	_, err := tr.Stream(context.Background(), "nb1", streamParams, nil)
	if te := streamErr(t, err); !te.Sent {
		t.Fatalf("first-chunk timeout after send: Sent = false (%v)", err)
	}

	f2 := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {})
	tr2, _ := newTransport(t, f2)
	f2.srv.Close()
	_, err = tr2.Stream(context.Background(), "nb1", streamParams, nil)
	if te := streamErr(t, err); te.Sent {
		t.Fatalf("refused before send: Sent = true (%v)", err)
	}
}

// The header deadline is time to first byte: once the response has begun,
// a slow rest-of-headers must not trip it.
func TestHeaderTimeoutIsTimeToFirstByte(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		buf.WriteString("HTTP/1.1 200 OK\r\n")
		buf.Flush()
		time.Sleep(200 * time.Millisecond)
		fmt.Fprintf(buf, "Content-Length: %d\r\nConnection: close\r\n\r\n%s", len(okBody), okBody)
		buf.Flush()
	})
	tr, _ := newTransport(t, f)
	tr.o.HeaderTimeout = 50 * time.Millisecond
	tr.o.MaxRetries = 0
	if _, err := tr.Do(context.Background(), listCall); err != nil {
		t.Fatalf("response that began before the deadline was cut: %v", err)
	}
}
