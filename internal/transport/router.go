package transport

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Router sends every call to the primary transport and switches to the
// fallback only when the fallback policy allows it:
//
//   - the primary error is a *Error whose Kind.AllowsFallback() (Transport,
//     Drift, CookieBinding); never RateLimited, Quota, Auth, Permission,
//     NotFound, InvalidArgument, Pending, Unsupported or Internal;
//   - the caller's context is still live;
//   - the call is idempotent, or the primary failed before the request body
//     was sent (Error.Sent false). A non-idempotent call that may already
//     have been applied is never replayed on another transport.
//
// The fallback is constructed lazily, on the first call that needs it, and
// at most once; its result (including a construction error) is kept.
type Router struct {
	primary     Transport
	newFallback func(ctx context.Context) (Transport, error)

	once  sync.Once
	fb    Transport
	fbErr error

	mu   sync.Mutex
	last string
}

// NewRouter wraps primary. fallback may be nil (no fallback). It runs at
// most once, under a lock that concurrent fallers wait on, with a context
// detached from the first caller's cancellation, so it must be cheap and
// bounded: the browser transport defers launching to its first call.
func NewRouter(primary Transport, fallback func(ctx context.Context) (Transport, error)) *Router {
	return &Router{primary: primary, newFallback: fallback}
}

// Name is the name of the transport that answered the most recent call, or
// the primary's before any success, so callers that record Name() after a
// call (the chat stream path in nlm) report "rpc" or "browser", never the
// router itself. Per-call truth is Result.Transport.
func (r *Router) Name() string {
	if l := r.Last(); l != "" {
		return l
	}
	return r.primary.Name()
}

// streamer is the stream method set of nlm.Streamer, repeated here because
// nlm imports transport.
type streamer interface {
	Stream(ctx context.Context, nb string, params any, onSnap func(wire.Snapshot)) (wire.Snapshot, error)
}

// Stream streams a chat answer on the primary transport. There is no
// fallback: the browser transport has no stream path yet, and a stream that
// failed part-way must not be replayed. It makes Router satisfy
// nlm.Streamer. If the primary cannot stream the result is KindUnsupported.
func (r *Router) Stream(ctx context.Context, nb string, params any, onSnap func(wire.Snapshot)) (wire.Snapshot, error) {
	st, ok := r.primary.(streamer)
	if !ok {
		return wire.Snapshot{}, &Error{Kind: KindUnsupported, RPC: "chat.ask", Transport: r.primary.Name(), Message: "transport cannot stream chat answers"}
	}
	snap, err := st.Stream(ctx, nb, params, onSnap)
	if err == nil {
		r.setLast(r.primary.Name())
	}
	return snap, err
}

// Supports reports whether the primary, or an already built fallback,
// handles rpc. It never constructs the fallback.
func (r *Router) Supports(rpc string) bool {
	if r.primary.Supports(rpc) {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fb != nil && r.fb.Supports(rpc)
}

// Last is the name of the transport that answered the most recent
// successful call ("" before any success). Under concurrency it is a hint;
// Result.Transport is the per-call truth.
func (r *Router) Last() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last
}

func (r *Router) setLast(name string) {
	r.mu.Lock()
	r.last = name
	r.mu.Unlock()
}

// Do runs c on the primary and, if the policy allows, on the fallback.
func (r *Router) Do(ctx context.Context, c Call) (Result, error) {
	res, err := r.primary.Do(ctx, c)
	if err == nil {
		r.answered(&res, r.primary)
		return res, nil
	}
	var pe *Error
	if !errors.As(err, &pe) || !r.mayFallBack(ctx, c, pe) {
		return Result{}, err
	}
	fb, ferr := r.fallback(ctx)
	if ferr != nil {
		return Result{}, both(pe, err, "fallback unavailable", ferr)
	}
	res, ferr = fb.Do(ctx, c)
	if ferr != nil {
		return Result{}, both(pe, err, "fallback ("+fb.Name()+") failed", ferr)
	}
	r.answered(&res, fb)
	return res, nil
}

func (r *Router) mayFallBack(ctx context.Context, c Call, pe *Error) bool {
	switch {
	case r.newFallback == nil, ctx.Err() != nil, !pe.Kind.AllowsFallback():
		return false
	case pe.Sent && !c.Idempotent:
		return false
	}
	return true
}

func (r *Router) fallback(ctx context.Context) (Transport, error) {
	r.once.Do(func() {
		fb, err := r.newFallback(context.WithoutCancel(ctx))
		if err == nil && fb == nil {
			err = errors.New("fallback constructor returned no transport")
		}
		r.mu.Lock()
		r.fb, r.fbErr = fb, err
		r.mu.Unlock()
	})
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fb, r.fbErr
}

func (r *Router) answered(res *Result, t Transport) {
	if res.Transport == "" {
		res.Transport = t.Name()
	}
	r.setLast(res.Transport)
}

// both reports a fallback failure without hiding the primary one: the
// result is a *Error with the primary's Kind (callers and the MCP envelope
// classify by it), and errors.Is/As reach both underlying errors.
func both(pe *Error, primary error, what string, second error) *Error {
	out := *pe
	out.Message = fmt.Sprintf("primary (%s) failed: %s; %s: %s", pe.Transport, primary.Error(), what, second.Error())
	out.Cause = errors.Join(primary, second)
	return &out
}
