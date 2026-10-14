package mcpserver

import (
	"context"
	"fmt"

	"github.com/candidloom-hub/sourcebolt/internal/resolve"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Helpers shared by several tool groups.

func writeAnn(title string, destructive, idempotent, openWorld bool) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: false, DestructiveHint: &destructive, IdempotentHint: idempotent, OpenWorldHint: &openWorld}
}

// unsupported is the error of a documented tool option that is not available yet.
func unsupported(rpc, format string, a ...any) error {
	return &transport.Error{Kind: transport.KindUnsupported, RPC: rpc, Message: fmt.Sprintf(format, a...)}
}

// lookupSourceIDs resolves source references (id, prefix or title) of one notebook.
func (s *server) lookupSourceIDs(ctx context.Context, nb string, refs []string) ([]string, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	srcs, err := s.NLM.ListSources(ctx, nb)
	if err != nil {
		return nil, err
	}
	items := make([]resolve.Item, len(srcs))
	for i, x := range srcs {
		items[i] = resolve.Item{ID: x.ID, Title: x.Title}
	}
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		it, err := resolve.Resolve(r, items)
		if err != nil {
			return nil, err
		}
		out = append(out, it.ID)
	}
	return out, nil
}

// invalidateNotebook drops the cached notebook list and one notebook's entries.
func (s *server) invalidateNotebook(id string) {
	s.Cache.Invalidate(s.Profile + "/notebooks")
	if id != "" {
		s.Cache.Invalidate(s.Profile + "/notebook/" + id)
	}
}
