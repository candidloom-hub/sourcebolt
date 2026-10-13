package mcpserver

import (
	"context"
	"strings"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/resolve"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	listTTL      = 30 * time.Second
	defaultLimit = 50
)

type NotebookListIn struct {
	Query   string `json:"query,omitempty" jsonschema:"case-insensitive filter on notebook titles"`
	Scope   string `json:"scope,omitempty" jsonschema:"mine, shared or all (default all)"`
	Limit   int    `json:"limit,omitempty" jsonschema:"max notebooks to return (default 50)"`
	Offset  int    `json:"offset,omitempty" jsonschema:"skip this many notebooks"`
	Refresh bool   `json:"refresh,omitempty" jsonschema:"bypass the 30 s cache"`
}

type NotebookListOut struct {
	Notebooks []nlm.Notebook `json:"notebooks"`
	Total     int            `json:"total"`
}

type NotebookOpenIn struct {
	Notebook string `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
}

func (s *server) registerNotebooks(m *mcp.Server) {
	register(s, m, "core", &mcp.Tool{
		Name:        "notebook_list",
		Description: "List NotebookLM notebooks with id, title, source count and ownership. Filter by title and scope. Cached 30 s.",
		Annotations: readOnly("List notebooks"),
	}, s.notebookList)

	register(s, m, "core", &mcp.Tool{
		Name:        "notebook_open",
		Description: "Open one notebook by id or title: its sources with status, summary and suggested questions.",
		Annotations: readOnly("Open notebook"),
	}, s.notebookOpen)
}

func (s *server) notebooks(ctx context.Context, refresh bool) ([]nlm.Notebook, error) {
	key := s.Profile + "/notebooks"
	if refresh {
		s.Cache.Invalidate(key)
	}
	v, err := s.Cache.Get(ctx, key, listTTL, func(ctx context.Context) (any, error) {
		return s.NLM.ListNotebooks(ctx)
	})
	if err != nil {
		return nil, err
	}
	return v.([]nlm.Notebook), nil
}

func (s *server) notebookList(ctx context.Context, in NotebookListIn) (NotebookListOut, error) {
	switch in.Scope {
	case "", "all", "mine", "shared":
	default:
		return NotebookListOut{}, badInput("scope must be mine, shared or all, got %q", in.Scope)
	}
	all, err := s.notebooks(ctx, in.Refresh)
	if err != nil {
		return NotebookListOut{}, err
	}
	q := strings.ToLower(strings.TrimSpace(in.Query))
	matched := make([]nlm.Notebook, 0, len(all))
	for _, n := range all {
		if q != "" && !strings.Contains(strings.ToLower(n.Title), q) {
			continue
		}
		if (in.Scope == "mine" && !n.Owned) || (in.Scope == "shared" && n.Owned) {
			continue
		}
		matched = append(matched, n)
	}
	limit := in.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	start := min(max(in.Offset, 0), len(matched))
	end := min(start+limit, len(matched))
	return NotebookListOut{Notebooks: matched[start:end], Total: len(matched)}, nil
}

// resolveNotebook turns an id, prefix or title into a notebook id using
// the cached list; an exact id skips the list entirely.
func (s *server) resolveNotebook(ctx context.Context, ref string) (string, error) {
	if looksLikeID(ref) {
		return ref, nil
	}
	all, err := s.notebooks(ctx, false)
	if err != nil {
		return "", err
	}
	items := make([]resolve.Item, len(all))
	for i, n := range all {
		items[i] = resolve.Item{ID: n.ID, Title: n.Title}
	}
	it, err := resolve.Resolve(ref, items)
	return it.ID, err
}

func looksLikeID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch {
		case i == 8 || i == 13 || i == 18 || i == 23:
			if c != '-' {
				return false
			}
		case (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'):
		default:
			return false
		}
	}
	return true
}

func (s *server) notebookOpen(ctx context.Context, in NotebookOpenIn) (nlm.NotebookDetail, error) {
	id, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return nlm.NotebookDetail{}, err
	}
	return s.NLM.GetNotebook(ctx, id)
}
