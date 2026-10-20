// Package nlm is the typed NotebookLM domain client. It builds positional
// payloads, sends them through a transport.Transport and decodes results
// into Go types. Payload builders are package-level functions so live
// captures can pin their shapes in one place.
package nlm

import (
	"context"
	"encoding/json/jsontext"
	"fmt"
	"sync"

	"github.com/candidloom-hub/sourcebolt/internal/rpcids"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Client talks to NotebookLM through one transport (usually a Router).
type Client struct {
	t transport.Transport
	// OnTransport, when set, receives the name of the transport that served each call.
	OnTransport func(name string)
	// newThread serializes, per notebook, the asks that start a thread and
	// read its id back (see Ask).
	newThread keyedLock
}

func New(t transport.Transport) *Client { return &Client{t: t} }

// keyedLock is a set of mutexes by key, made on demand and dropped once
// nobody holds or waits for them. The zero value is ready to use.
type keyedLock struct {
	mu sync.Mutex
	m  map[string]*keyedEntry
}

type keyedEntry struct {
	ch   chan struct{} // holds one token while locked
	refs int           // holder plus waiters
}

// Lock waits until key is free or ctx ends, and returns the unlock func.
func (k *keyedLock) Lock(ctx context.Context, key string) (func(), error) {
	k.mu.Lock()
	if k.m == nil {
		k.m = map[string]*keyedEntry{}
	}
	e := k.m[key]
	if e == nil {
		e = &keyedEntry{ch: make(chan struct{}, 1)}
		k.m[key] = e
	}
	e.refs++
	k.mu.Unlock()
	release := func() {
		k.mu.Lock()
		if e.refs--; e.refs == 0 {
			delete(k.m, key)
		}
		k.mu.Unlock()
	}
	select {
	case e.ch <- struct{}{}:
		return func() { <-e.ch; release() }, nil
	case <-ctx.Done():
		release()
		return nil, ctx.Err()
	}
}

type callOpts struct {
	sourcePath string
	idempotent bool
	null       wire.NullPolicy
	pending    []int
	hl         string
}

func notebookPath(id string) string { return "/notebook/" + id }

func (c *Client) do(ctx context.Context, rpc string, params any, o callOpts) (jsontext.Value, error) {
	id, ok := rpcids.Lookup(rpc)
	if !ok {
		return nil, fmt.Errorf("nlm: unknown rpc %q", rpc)
	}
	sp := o.sourcePath
	if sp == "" {
		sp = "/"
	}
	res, err := c.t.Do(ctx, transport.Call{
		RPC: rpc, ID: id, Params: params, SourcePath: sp,
		Idempotent: o.idempotent, Null: o.null, PendingCodes: o.pending, HL: o.hl,
	})
	if err != nil {
		return nil, err
	}
	if c.OnTransport != nil {
		c.OnTransport(res.Transport)
	}
	return res.Payload, nil
}
