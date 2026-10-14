package mcpserver

import (
	"context"
	"fmt"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ShareStatusIn struct {
	Notebook string `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
}

type ShareUserChange struct {
	Email string `json:"email" jsonschema:"collaborator email"`
	Role  string `json:"role" jsonschema:"viewer, editor or remove"`
}

type ShareUpdateIn struct {
	Notebook  string            `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Public    *bool             `json:"public,omitempty" jsonschema:"true = anyone with the link can view, false = restricted"`
	Users     []ShareUserChange `json:"users,omitempty" jsonschema:"per-user changes"`
	Notify    bool              `json:"notify,omitempty" jsonschema:"email the added people"`
	Message   string            `json:"message,omitempty" jsonschema:"message to include in the notification"`
	ViewLevel string            `json:"view_level,omitempty" jsonschema:"full or chat_only (not available yet)"`
	Artifact  string            `json:"artifact,omitempty" jsonschema:"share one artifact instead of the notebook (not available yet)"`
	Confirm   bool              `json:"confirm,omitempty" jsonschema:"required to widen access or remove people; without it only a preview is returned"`
}

type ShareUpdateOut struct {
	Status          string   `json:"status"` // preview | applied
	NotebookID      string   `json:"notebook_id"`
	NotebookTitle   string   `json:"notebook_title,omitempty"`
	RequiresConfirm bool     `json:"requires_confirm,omitempty"`
	Changes         []string `json:"changes"`
	Guidance        string   `json:"guidance,omitempty"`
}

func (s *server) registerSharing(m *mcp.Server) {
	register(s, m, "core", &mcp.Tool{
		Name:        "share_status",
		Description: "Show who a notebook is shared with and whether the link is public.",
		Annotations: readOnly("Share status"),
	}, s.shareStatus)

	register(s, m, "core", &mcp.Tool{
		Name:        "share_update",
		Description: "Change notebook sharing: link visibility (public) and per-user roles (viewer, editor, remove). Widening access or removing people requires confirm:true; without it a preview is returned and nothing is sent. Payload shapes are not live-verified. view_level and artifact sharing are not available yet.",
		Annotations: writeAnn("Update sharing", true, true, true),
	}, s.shareUpdate)
}

func (s *server) shareStatus(ctx context.Context, in ShareStatusIn) (nlm.ShareStatus, error) {
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return nlm.ShareStatus{}, err
	}
	return s.NLM.GetShare(ctx, nb)
}

func (s *server) shareUpdate(ctx context.Context, in ShareUpdateIn) (ShareUpdateOut, error) {
	if in.ViewLevel != "" || in.Artifact != "" {
		return ShareUpdateOut{}, unsupported("sharing.share", "view_level and artifact sharing are not supported yet")
	}
	if in.Public == nil && len(in.Users) == 0 {
		return ShareUpdateOut{}, badInput("give public or at least one users entry")
	}
	var changes []string
	needConfirm := false
	if in.Public != nil {
		if *in.Public {
			changes = append(changes, "make the notebook viewable by anyone with the link")
			needConfirm = true
		} else {
			changes = append(changes, "restrict the link to invited people")
		}
	}
	for _, u := range in.Users {
		if u.Email == "" {
			return ShareUpdateOut{}, badInput("every users entry needs an email")
		}
		switch u.Role {
		case "viewer", "editor":
			changes = append(changes, fmt.Sprintf("give %s %s access", u.Email, u.Role))
		case "remove":
			changes = append(changes, fmt.Sprintf("remove %s", u.Email))
		default:
			return ShareUpdateOut{}, badInput("role for %q must be viewer, editor or remove, got %q", u.Email, u.Role)
		}
		needConfirm = true
	}
	it, err := s.resolveNotebookStrict(ctx, in.Notebook)
	if err != nil {
		return ShareUpdateOut{}, err
	}
	nb := it.ID
	if needConfirm && !in.Confirm {
		return ShareUpdateOut{Status: "preview", NotebookID: nb, NotebookTitle: it.Title, RequiresConfirm: true, Changes: changes,
			Guidance: "Nothing was changed. Repeat the call with confirm:true to apply."}, nil
	}
	u := nlm.ShareUpdate{Public: in.Public, Notify: in.Notify, Message: in.Message}
	for _, c := range in.Users {
		u.Users = append(u.Users, nlm.ShareChange{Email: c.Email, Role: c.Role})
	}
	if err := s.NLM.UpdateShare(ctx, nb, u); err != nil {
		return ShareUpdateOut{}, err
	}
	s.invalidateNotebook(nb)
	return ShareUpdateOut{Status: "applied", NotebookID: nb, NotebookTitle: it.Title, Changes: changes}, nil
}
