package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// minIDPrefix matches the resolve package's shortest accepted id prefix.
const minIDPrefix = 6

// resolveStrict picks one item for a mutation: exact id, unique id prefix
// (6+ chars) or exact title (case-insensitive). Unlike resolve.Resolve it
// never accepts a title substring. A ref that only matches loosely fails with
// NOT_FOUND and lists the loose matches as suggestions; several strict
// matches fail with AMBIGUOUS. Reads keep using resolve.Resolve.
func resolveStrict(ref string, items []resolve.Item) (resolve.Item, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return resolve.Item{}, &resolve.ErrNotFound{Ref: ref}
	}
	for _, it := range items {
		if it.ID == ref {
			return it, nil
		}
	}
	lower := strings.ToLower(ref)
	for _, match := range []func(resolve.Item) bool{
		func(it resolve.Item) bool {
			return len(ref) >= minIDPrefix && strings.HasPrefix(strings.ToLower(it.ID), lower)
		},
		func(it resolve.Item) bool { return strings.EqualFold(it.Title, ref) },
	} {
		var hits []resolve.Item
		for _, it := range items {
			if match(it) {
				hits = append(hits, it)
			}
		}
		switch len(hits) {
		case 0:
			continue
		case 1:
			return hits[0], nil
		default:
			return resolve.Item{}, &resolve.ErrAmbiguous{Ref: ref, Candidates: hits}
		}
	}
	nf := &resolve.ErrNotFound{Ref: ref}
	var amb *resolve.ErrAmbiguous
	var loose *resolve.ErrNotFound
	switch it, err := resolve.Resolve(ref, items); {
	case err == nil:
		nf.Suggestions = []resolve.Item{it}
	case errors.As(err, &amb):
		nf.Suggestions = amb.Candidates
	case errors.As(err, &loose):
		nf.Suggestions = loose.Suggestions
	}
	return resolve.Item{}, nf
}

// notebookItems lists notebooks as resolve items; refresh bypasses the cache.
func (s *server) notebookItems(ctx context.Context, refresh bool) ([]resolve.Item, error) {
	all, err := s.notebooks(ctx, refresh)
	if err != nil {
		return nil, err
	}
	items := make([]resolve.Item, len(all))
	for i, n := range all {
		items[i] = resolve.Item{ID: n.ID, Title: n.Title}
	}
	return items, nil
}

// resolveNotebookStrict resolves a notebook for a mutation against a freshly
// fetched notebook list (never the cache), with resolveStrict rules.
func (s *server) resolveNotebookStrict(ctx context.Context, ref string) (resolve.Item, error) {
	items, err := s.notebookItems(ctx, true)
	if err != nil {
		return resolve.Item{}, err
	}
	return resolveStrict(ref, items)
}

// invalidateNotebook drops the cached notebook list and one notebook's entries.
func (s *server) invalidateNotebook(id string) {
	s.Cache.Invalidate(s.Profile + "/notebooks")
	if id != "" {
		s.Cache.Invalidate(s.Profile + "/notebook/" + id)
	}
}
