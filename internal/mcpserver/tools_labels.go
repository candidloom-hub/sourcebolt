package mcpserver

import (
	"context"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/resolve"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type LabelListIn struct {
	Notebook string `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Scope    string `json:"scope,omitempty" jsonschema:"source_labels (default); collections are not available yet"`
}

type LabelListOut struct {
	Labels []nlm.Label `json:"labels"`
	Total  int         `json:"total"`
}

type LabelUpdateIn struct {
	Notebook string   `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Scope    string   `json:"scope,omitempty" jsonschema:"source_labels (default)"`
	Action   string   `json:"action" jsonschema:"rename, set_emoji, add_sources or remove_sources (auto and create are not available yet)"`
	Label    string   `json:"label" jsonschema:"label id or name"`
	Name     string   `json:"name,omitempty" jsonschema:"new name for rename"`
	Emoji    string   `json:"emoji,omitempty" jsonschema:"emoji for set_emoji"`
	Items    []string `json:"items,omitempty" jsonschema:"source ids, id prefixes or titles for add_sources / remove_sources"`
}

type LabelUpdateOut struct {
	Status string `json:"status"`
	Label  string `json:"label"`
	Action string `json:"action"`
}

func (s *server) registerLabels(m *mcp.Server) {
	register(s, m, "core", &mcp.Tool{
		Name:        "label_list",
		Description: "List a notebook's source labels with emoji and the ids of the sources in each. Collections are not available yet.",
		Annotations: readOnly("List labels"),
	}, s.labelList)

	register(s, m, "core", &mcp.Tool{
		Name:        "label_update",
		Description: "Rename a label, set its emoji, or add/remove sources. Actions auto and create (and notebook collections) are not available yet and return UNSUPPORTED_BY_TRANSPORT. Payload shapes are not live-verified.",
		Annotations: writeAnn("Update label", false, false, false),
	}, s.labelUpdate)
}

func checkLabelScope(scope string) error {
	switch scope {
	case "", "source_labels":
		return nil
	case "collections":
		return unsupported("labels", "collections are not supported yet")
	}
	return badInput("scope must be source_labels or collections, got %q", scope)
}

func (s *server) labelList(ctx context.Context, in LabelListIn) (LabelListOut, error) {
	if err := checkLabelScope(in.Scope); err != nil {
		return LabelListOut{}, err
	}
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return LabelListOut{}, err
	}
	ls, err := s.NLM.Labels(ctx, nb)
	if err != nil {
		return LabelListOut{}, err
	}
	return LabelListOut{Labels: ls, Total: len(ls)}, nil
}

func (s *server) labelUpdate(ctx context.Context, in LabelUpdateIn) (LabelUpdateOut, error) {
	if err := checkLabelScope(in.Scope); err != nil {
		return LabelUpdateOut{}, err
	}
	mut := nlm.LabelMutation{}
	switch in.Action {
	case "auto", "create", "add_notebooks", "remove_notebooks":
		return LabelUpdateOut{}, unsupported("labels.create", "label action %q is not supported yet", in.Action)
	case "rename":
		if in.Name == "" {
			return LabelUpdateOut{}, badInput("rename needs name")
		}
		mut.Name = in.Name
	case "set_emoji":
		if in.Emoji == "" {
			return LabelUpdateOut{}, badInput("set_emoji needs emoji")
		}
		mut.Emoji = in.Emoji
	case "add_sources", "remove_sources":
		if len(in.Items) == 0 {
			return LabelUpdateOut{}, badInput("%s needs items", in.Action)
		}
	default:
		return LabelUpdateOut{}, badInput("action must be rename, set_emoji, add_sources or remove_sources, got %q", in.Action)
	}
	if in.Label == "" {
		return LabelUpdateOut{}, badInput("label is required")
	}
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return LabelUpdateOut{}, err
	}
	ls, err := s.NLM.Labels(ctx, nb)
	if err != nil {
		return LabelUpdateOut{}, err
	}
	items := make([]resolve.Item, len(ls))
	for i, l := range ls {
		items[i] = resolve.Item{ID: l.ID, Title: l.Name}
	}
	it, err := resolve.Resolve(in.Label, items)
	if err != nil {
		return LabelUpdateOut{}, err
	}
	mut.ID = it.ID
	if in.Action == "add_sources" || in.Action == "remove_sources" {
		ids, err := s.lookupSourceIDs(ctx, nb, in.Items)
		if err != nil {
			return LabelUpdateOut{}, err
		}
		if in.Action == "add_sources" {
			mut.AddSources = ids
		} else {
			mut.RemoveSources = ids
		}
	}
	err = s.NLM.MutateLabels(ctx, nb, []nlm.LabelMutation{mut})
	s.invalidateNotebook(nb)
	if err != nil {
		return LabelUpdateOut{}, err
	}
	return LabelUpdateOut{Status: "updated", Label: it.ID, Action: in.Action}, nil
}
