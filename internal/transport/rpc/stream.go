package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

const streamPath = "/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed"

// quotaMessage is shown when the stream is rejected with RESOURCE_EXHAUSTED
// (a UserDisplayableError detail); the server sends no text of its own.
const quotaMessage = "NotebookLM declined to answer: this account has reached its chat limit. Try again later."

var (
	errFirstChunk = errors.New("no first chunk before the deadline")
	errIdle       = errors.New("stream idle between chunks too long")
	errTotal      = errors.New("stream exceeded its total time limit")
)

var streamCall = transport.Call{RPC: "chat.ask", ID: "GenerateFreeFormStreamed"}

// Stream posts one chat question to GenerateFreeFormStreamed and decodes the
// answer incrementally. onSnap (may be nil) sees every snapshot, thinking
// chunks included. The result is the final answer snapshot: the one marked
// final, else the last non-thinking snapshot.
//
// A stream is never retried: once the body is sent the question may have
// been answered (and billed) server side. Auth failures drop the cached
// tokens so the next call re-bootstraps. Limits (Options): first chunk,
// idle gap between chunks, total duration, answer text size.
func (t *Transport) Stream(ctx context.Context, nb string, params any, onSnap func(wire.Snapshot)) (wire.Snapshot, error) {
	c := streamCall
	c.SourcePath = notebookSourcePath(nb)
	if err := t.o.Limiter.Wait(ctx); err != nil {
		return wire.Snapshot{}, t.fail(c, transport.KindTransport, false, err)
	}
	tok, err := t.ensureTokens(ctx, false)
	if err != nil {
		return wire.Snapshot{}, err
	}
	body, err := wire.EncodeStream(params, tok.AT)
	if err != nil {
		return wire.Snapshot{}, t.fail(c, transport.KindInternal, false, err)
	}

	cctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	sctx, stop := context.WithTimeoutCause(cctx, t.o.StreamTotal, errTotal)
	defer stop()
	var started atomic.Bool
	timer := time.AfterFunc(t.o.StreamFirstChunk, func() {
		if started.Load() {
			cancel(errIdle)
		} else {
			cancel(errFirstChunk)
		}
	})
	defer timer.Stop()

	q := url.Values{}
	q.Set("bl", tok.BL)
	q.Set("hl", tok.HL)
	q.Set("_reqid", strconv.FormatInt(t.reqid.Add(100000), 10))
	if tok.FSID != "" {
		q.Set("f.sid", tok.FSID)
	}
	q.Set("rt", "c")
	host := t.host()
	req, err := http.NewRequestWithContext(sctx, http.MethodPost, host.Origin()+streamPath+"?"+q.Encode(), strings.NewReader(body))
	if err != nil {
		return wire.Snapshot{}, t.fail(c, transport.KindInternal, false, err)
	}
	t.setHeaders(req, host)
	resp, err := t.o.HTTP.Do(req)
	if err != nil {
		return wire.Snapshot{}, t.streamFail(ctx, sctx, c, err)
	}
	defer resp.Body.Close()
	t.mergeCookies(resp)
	if e := t.statusErr(c, resp); e != nil {
		if e.Kind == transport.KindAuth {
			t.dropTokens()
		}
		return wire.Snapshot{}, e
	}

	d := wire.NewStreamDecoder(&idleReader{r: resp.Body, timer: timer, idle: t.o.StreamIdle, started: &started})
	var final, last *wire.Snapshot
	for {
		s, err := d.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if fs := d.Trailers().FSID; fs != "" {
				t.setFSID(fs)
			}
			if sctx.Err() != nil {
				return wire.Snapshot{}, t.streamFail(ctx, sctx, c, err)
			}
			var we *wire.Error
			if errors.As(err, &we) {
				e := t.wireErr(err, c)
				if e.Kind == transport.KindAuth {
					t.dropTokens()
				}
				return wire.Snapshot{}, e
			}
			return wire.Snapshot{}, t.fail(c, transport.KindTransport, false, err)
		}
		if s.Status != nil {
			return wire.Snapshot{}, t.rejection(c, s.Status)
		}
		if len(s.Text) > t.o.StreamMaxAnswer {
			return wire.Snapshot{}, t.fail(c, transport.KindInternal, false, fmt.Errorf("answer exceeds the %d-byte cap", t.o.StreamMaxAnswer))
		}
		if onSnap != nil {
			onSnap(s)
		}
		if s.Thinking {
			continue
		}
		if s.Final {
			final = &s
		}
		last = &s
	}
	if fs := d.Trailers().FSID; fs != "" {
		t.setFSID(fs)
	}
	switch {
	case final != nil:
		return *final, nil
	case last != nil:
		return *last, nil
	}
	return wire.Snapshot{}, t.fail(c, transport.KindDrift, false, errors.New("stream ended without an answer"))
}

// rejection maps a null-answer frame's google.rpc.Status.
func (t *Transport) rejection(c transport.Call, st *wire.Status) *transport.Error {
	e := t.wireErr(&wire.Error{Code: st.Code, ID: c.ID, Status: st}, c)
	if e.Kind == transport.KindQuota && e.Message == "" {
		e.Message = quotaMessage
	}
	return e
}

// streamFail builds the error for a stream cut by a context: the caller's
// own cancellation wins, then the stream limits, then the I/O error.
func (t *Transport) streamFail(parent, sctx context.Context, c transport.Call, err error) *transport.Error {
	if perr := parent.Err(); perr != nil {
		return t.fail(c, transport.KindTransport, false, fmt.Errorf("%w: %w", perr, stripURL(err)))
	}
	if cause := context.Cause(sctx); cause != nil {
		return t.fail(c, transport.KindTransport, false, cause)
	}
	return t.fail(c, transport.KindTransport, false, err)
}

// dropTokens adopts a newer login from disk and forces a re-bootstrap on the
// next call (the stream itself is not resent).
func (t *Transport) dropTokens() {
	t.reloadIfNewer()
	t.mu.Lock()
	t.tokens = nil
	t.mu.Unlock()
}

func notebookSourcePath(nb string) string {
	if nb == "" {
		return "/"
	}
	return "/notebook/" + nb
}

// idleReader re-arms the stream timer whenever bytes arrive: the first read
// switches it from the first-chunk deadline to the idle deadline.
type idleReader struct {
	r       io.Reader
	timer   *time.Timer
	idle    time.Duration
	started *atomic.Bool
}

func (ir *idleReader) Read(p []byte) (int, error) {
	n, err := ir.r.Read(p)
	if n > 0 {
		ir.started.Store(true)
		ir.timer.Reset(ir.idle)
	}
	return n, err
}
