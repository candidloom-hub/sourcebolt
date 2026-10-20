package mcpserver

import (
	"context"
	"fmt"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/safety"
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

// SharePartOut is the outcome of one of the two sharing calls (link
// setting, users) and the changes it carried.
type SharePartOut struct {
	Part    string   `json:"part"`   // link | users
	Status  string   `json:"status"` // applied | failed
	Changes []string `json:"changes"`
	Error   string   `json:"error,omitempty"`
}

type ShareUpdateOut struct {
	Status          string         `json:"status"` // preview | applied | partial
	NotebookID      string         `json:"notebook_id"`
	NotebookTitle   string         `json:"notebook_title,omitempty"`
	RequiresConfirm bool           `json:"requires_confirm,omitempty"`
	Changes         []string       `json:"changes"`
	Parts           []SharePartOut `json:"parts,omitempty"`
	Guidance        string         `json:"guidance,omitempty"`
}

func (s *server) registerSharing(m *mcp.Server) {
	register(s, m, "core", &mcp.Tool{
		Name:        "share_status",
		Description: "Show who a notebook is shared with and whether the link is public.",
		Annotations: readOnly("Share status"),
	}, s.shareStatus)

	register(s, m, "core", &mcp.Tool{
		Name:        "share_update",
		Description: "Change notebook sharing: link visibility (public) and per-user roles (viewer, editor, remove). Widening access or removing people requires confirm:true; without it a preview is returned and nothing is sent. The link setting and the user changes are sent as two calls; if only one applies, status is partial and parts says which applied and which failed. Payload shapes are not live-verified. view_level and artifact sharing are not available yet.",
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
	// Changes are kept per part so a partial outcome can say which ones landed.
	var linkChanges, userChanges []string
	needConfirm := false
	if in.Public != nil {
		if *in.Public {
			linkChanges = append(linkChanges, "make the notebook viewable by anyone with the link")
			needConfirm = true
		} else {
			linkChanges = append(linkChanges, "restrict the link to invited people")
		}
	}
	for _, u := range in.Users {
		if u.Email == "" {
			return ShareUpdateOut{}, badInput("every users entry needs an email")
		}
		switch u.Role {
		case "viewer", "editor":
			userChanges = append(userChanges, fmt.Sprintf("give %s %s access", u.Email, u.Role))
		case "remove":
			userChanges = append(userChanges, fmt.Sprintf("remove %s", u.Email))
		default:
			return ShareUpdateOut{}, badInput("role for %q must be viewer, editor or remove, got %q", u.Email, u.Role)
		}
		needConfirm = true
	}
	changes := append(append([]string{}, linkChanges...), userChanges...)
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
	parts, err := s.NLM.UpdateShare(ctx, nb, u)
	if err != nil {
		return ShareUpdateOut{}, err
	}
	s.invalidateNotebook(nb)
	out := ShareUpdateOut{NotebookID: nb, NotebookTitle: it.Title, Changes: changes}
	applied := 0
	var firstErr error
	for _, p := range parts {
		po := SharePartOut{Part: p.Part, Status: p.Status, Changes: linkChanges}
		if p.Part == "users" {
			po.Changes = userChanges
		}
		if p.Err != nil {
			po.Error = safety.Redact(p.Err.Error())
			if firstErr == nil {
				firstErr = p.Err
			}
		} else {
			applied++
		}
		out.Parts = append(out.Parts, po)
	}
	// Same rule as delete: all applied is "applied", none applied is the
	// first error as an envelope (nothing changed), anything between is
	// "partial" with per-part outcomes.
	switch {
	case applied == len(out.Parts):
		out.Status = "applied"
	case applied == 0:
		return ShareUpdateOut{}, firstErr
	default:
		out.Status = "partial"
		out.Guidance = "Only some sharing changes were applied; see each part's status and error. Check share_status before retrying the failed part."
	}
	return out, nil
}
