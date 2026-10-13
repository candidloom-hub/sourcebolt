// Package transport defines how the domain client reaches NotebookLM.
// Implementations: rpc (web batchexecute), browser (in-page fetch, P4),
// enterprise (official REST, after 1.0). A Router picks between them.
package transport

import (
	"context"
	"encoding/json/jsontext"

	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Call is one RPC. The domain client resolves ID from the logical RPC name.
type Call struct {
	RPC          string // logical name, e.g. "notebooks.list"
	ID           string // wire id, e.g. "wXbhsf"
	Params       any    // positional payload, JSON-encodable
	SourcePath   string // "/" or "/notebook/{id}"
	Idempotent   bool   // safe to retry after the request was sent
	PendingCodes []int  // gRPC codes meaning "accepted, maybe applied" (3, 9 on add-source)
	Null         wire.NullPolicy
	HL           string // overrides the page language (Studio generation language)
}

// Result is the decoded payload plus the transport that produced it.
type Result struct {
	Payload   jsontext.Value
	Transport string // "rpc" | "browser" | "enterprise"
}

// Transport executes calls.
type Transport interface {
	Name() string
	Do(ctx context.Context, c Call) (Result, error)
	Supports(rpc string) bool
}
