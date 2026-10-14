// Package nlm is the typed NotebookLM domain client. It builds positional
// payloads, sends them through a transport.Transport and decodes results
// into Go types. Payload builders are package-level functions so live
// captures can pin their shapes in one place.
package nlm

import (
	"context"
	"encoding/json/jsontext"
	"fmt"

	"github.com/candidloom-hub/sourcebolt/internal/rpcids"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Client talks to NotebookLM through one transport (usually a Router).
type Client struct {
	t transport.Transport
	// OnTransport, when set, receives the name of the transport that served each call.
	OnTransport func(name string)
}

func New(t transport.Transport) *Client { return &Client{t: t} }

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
