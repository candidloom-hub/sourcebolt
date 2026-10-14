package mcpserver

import (
	"context"

	"github.com/candidloom-hub/sourcebolt/internal/resolve"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type DeleteIn struct {
	Kind     string   `json:"kind" jsonschema:"notebook, source, note, artifact, chat_history, label or collection"`
	Notebook string   `json:"notebook,omitempty" jsonschema:"notebook id, prefix or title; required for every kind except notebook"`
	IDs      []string `json:"ids" jsonschema:"ids (or titles) of the items to delete"`
	Confirm  bool     `json:"confirm,omitempty" jsonschema:"must be true to delete; without it only a preview is returned"`
}

type DeleteItem struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
}

type DeleteOut struct {
	Status   string       `json:"status"` // preview | deleted
	Kind     string       `json:"kind"`
	Items    []DeleteItem `json:"items"`
	Guidance string       `json:"guidance,omitempty"`
}

func (s *server) registerDelete(m *mcp.Server) {
	register(s, m, "core", &mcp.Tool{
		Name:        "delete",
		Description: "Delete notebooks, sources, notes, artifacts, chat history, labels or collections. Without confirm:true it only returns a preview of what would be deleted and changes nothing. Currently only kind=notebook can be deleted; other kinds return UNSUPPORTED_BY_TRANSPORT.",
		Annotations: writeAnn("Delete", true, true, false),
	}, s.delete)
}

func (s *server) delete(ctx context.Context, in DeleteIn) (DeleteOut, error) {
	switch in.Kind {
	case "notebook", "source", "note", "artifact":
	case "chat_history", "label", "collection":
		return DeleteOut{}, unsupported("delete."+in.Kind, "deleting %s is not supported yet", in.Kind)
	default:
		return DeleteOut{}, badInput("kind must be notebook, source, note, artifact, chat_history, label or collection, got %q", in.Kind)
	}
	if len(in.IDs) == 0 {
		return DeleteOut{}, badInput("ids is empty")
	}
	var nb string
	var items []resolve.Item
	var titles []DeleteItem
	if in.Kind == "notebook" {
		all, err := s.notebooks(ctx, false)
		if err != nil {
			return DeleteOut{}, err
		}
		for _, n := range all {
			items = append(items, resolve.Item{ID: n.ID, Title: n.Title})
		}
	} else {
		if in.Notebook == "" {
			return DeleteOut{}, badInput("notebook is required to delete a %s", in.Kind)
		}
		var err error
		if nb, err = s.resolveNotebook(ctx, in.Notebook); err != nil {
			return DeleteOut{}, err
		}
		if items, err = s.deletable(ctx, in.Kind, nb); err != nil {
			return DeleteOut{}, err
		}
	}
	ids := make([]string, 0, len(in.IDs))
	for _, ref := range in.IDs {
		it, err := resolve.Resolve(ref, items)
		if err != nil {
			return DeleteOut{}, err
		}
		ids = append(ids, it.ID)
		titles = append(titles, DeleteItem{ID: it.ID, Title: it.Title})
	}
	if !in.Confirm {
		return DeleteOut{Status: "preview", Kind: in.Kind, Items: titles,
			Guidance: "Nothing was deleted. Repeat the call with confirm:true to delete these items."}, nil
	}
	err := s.NLM.Delete(ctx, in.Kind, nb, ids)
	s.invalidateNotebook(nb)
	for _, id := range ids {
		if in.Kind == "notebook" {
			s.invalidateNotebook(id)
		}
	}
	if err != nil {
		return DeleteOut{}, err
	}
	return DeleteOut{Status: "deleted", Kind: in.Kind, Items: titles}, nil
}

// deletable lists the items of one notebook-scoped kind for id and title resolution.
func (s *server) deletable(ctx context.Context, kind, nb string) ([]resolve.Item, error) {
	var items []resolve.Item
	switch kind {
	case "source":
		srcs, err := s.NLM.ListSources(ctx, nb)
		if err != nil {
			return nil, err
		}
		for _, x := range srcs {
			items = append(items, resolve.Item{ID: x.ID, Title: x.Title})
		}
	case "note":
		notes, err := s.NLM.ListNotes(ctx, nb)
		if err != nil {
			return nil, err
		}
		for _, n := range notes {
			items = append(items, resolve.Item{ID: n.ID, Title: n.Title})
		}
	case "artifact":
		arts, err := s.NLM.ListArtifacts(ctx, nb)
		if err != nil {
			return nil, err
		}
		for _, a := range arts {
			items = append(items, resolve.Item{ID: a.ID, Title: a.Title})
		}
	}
	return items, nil
}
