package rpc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

// okBody answers wXbhsf, so any other call id decodes as drift.

func TestDriftNoResendWhenNonIdempotent(t *testing.T) {
	write := transport.Call{RPC: "sources.add", ID: "izAoDd", Params: []any{"x"}, SourcePath: "/"}
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) { io.WriteString(w, okBody) })
	tr, _ := newTransport(t, f)
	_, err := tr.Do(context.Background(), write)
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindDrift || !te.Sent {
		t.Fatalf("err = %#v, want DRIFT with Sent", err)
	}
	if f.posts.Load() != 1 {
		t.Fatalf("posts = %d, want 1 (no resend of a possibly applied write)", f.posts.Load())
	}
}

func TestDriftRefetchWhenIdempotent(t *testing.T) {
	call := transport.Call{RPC: "x.read", ID: "izAoDd", Params: []any{"x"}, SourcePath: "/", Idempotent: true}
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) { io.WriteString(w, okBody) })
	tr, _ := newTransport(t, f)
	_, err := tr.Do(context.Background(), call)
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindDrift || !te.Sent {
		t.Fatalf("err = %#v, want DRIFT with Sent", err)
	}
	if f.posts.Load() != 2 {
		t.Fatalf("posts = %d, want 2 (one refetch for an idempotent call)", f.posts.Load())
	}
}

func TestStreamNoResendOnDrift(t *testing.T) {
	f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) { io.WriteString(w, ")]}'\n") })
	tr, _ := newTransport(t, f)
	_, err := tr.Stream(context.Background(), "nb1", streamParams, nil)
	if err == nil {
		t.Fatal("want error")
	}
	streamErr(t, err)
	if f.posts.Load() != 1 {
		t.Fatalf("posts = %d, want 1 (stream never resends)", f.posts.Load())
	}
}
