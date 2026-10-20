package mcpserver

import (
	"context"
	"errors"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/resolve"
	"github.com/candidloom-hub/sourcebolt/internal/safety"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type DeleteIn struct {
	Kind     string   `json:"kind" jsonschema:"notebook, source, note, artifact, chat_history, label or collection"`
	Notebook string   `json:"notebook,omitempty" jsonschema:"notebook id, prefix or title; required for every kind except notebook"`
	IDs      []string `json:"ids" jsonschema:"ids (or exact titles) of the items to delete; for chat_history, conversation ids"`
	Confirm  bool     `json:"confirm,omitempty" jsonschema:"must be true to delete; without it only a preview is returned"`
}

type DeleteItem struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
	// Conversations (kind chat_history) have no title: their times and,
	// when the whole history could be read, the first question are shown.
	Created       time.Time `json:"created,omitzero"`
	Updated       time.Time `json:"updated,omitzero"`
	FirstQuestion string    `json:"first_question,omitempty"`
	Status        string    `json:"status,omitempty"` // deleted | failed | unsupported (empty in a preview)
	Error         string    `json:"error,omitempty"`
}

type DeleteOut struct {
	Status   string       `json:"status"` // preview | deleted | partial
	Kind     string       `json:"kind"`
	Items    []DeleteItem `json:"items"`
	Guidance string       `json:"guidance,omitempty"`
}

func (s *server) registerDelete(m *mcp.Server) {
	register(s, m, "core", &mcp.Tool{
		Name:        "delete",
		Description: "Delete notebooks, sources, notes, artifacts, chat history, labels or collections. Without confirm:true it only returns a preview of what would be deleted and changes nothing. Items are matched strictly (exact id, unique id prefix or exact title) against a fresh list. source, note and chat_history need notebook; chat_history deletes the whole history of the given conversations; it reaches only the newest 20 conversations of a notebook, and its preview shows their created and updated times and, when the whole history can be read, the first question. Kinds artifact, label and collection are not available yet and return UNSUPPORTED_BY_TRANSPORT.",
		Annotations: writeAnn("Delete", true, true, false),
	}, s.delete)
}

func (s *server) delete(ctx context.Context, in DeleteIn) (DeleteOut, error) {
	switch in.Kind {
	case "notebook":
	case "source", "note", "chat_history":
		if in.Notebook == "" {
			return DeleteOut{}, badInput("notebook is required to delete a %s", in.Kind)
		}
	case "artifact", "label", "collection":
		// Checked before any lookup so no fake preview is shown for a kind
		// that cannot be deleted.
		return DeleteOut{}, unsupported("delete."+in.Kind, "deleting %s is not supported yet (payload not pinned by a live capture)", in.Kind)
	default:
		return DeleteOut{}, badInput("kind must be notebook, source, note, artifact, chat_history, label or collection, got %q", in.Kind)
	}
	if len(in.IDs) == 0 {
		return DeleteOut{}, badInput("ids is empty")
	}
	// Resolve against a fresh list with strict rules (no title substrings):
	// a stale or loose match must never pick the item that gets deleted.
	nb := ""
	var items []resolve.Item
	var extra map[string]DeleteItem
	var fullPage bool
	var err error
	if in.Kind == "notebook" {
		items, err = s.notebookItems(ctx, true)
	} else {
		var it resolve.Item
		if it, err = s.resolveNotebookStrict(ctx, in.Notebook); err == nil {
			nb = it.ID
			items, extra, fullPage, err = s.deletableItems(ctx, in.Kind, nb)
		}
	}
	if err != nil {
		return DeleteOut{}, err
	}
	var titles []DeleteItem
	ids := make([]string, 0, len(in.IDs))
	seen := map[string]bool{}
	for _, ref := range in.IDs {
		it, err := resolveStrict(ref, items, true)
		var nf *resolve.ErrNotFound
		if errors.As(err, &nf) && fullPage {
			return DeleteOut{}, unsupported("delete.chat_history",
				"conversation %q is not among the newest %d conversations of this notebook; older ones cannot be listed yet (hPTbtc paging is not pinned by a live capture)", ref, nlm.ChatSessionsPage)
		}
		if err != nil {
			return DeleteOut{}, err
		}
		if seen[it.ID] { // two refs naming the same item
			continue
		}
		seen[it.ID] = true
		ids = append(ids, it.ID)
		d := extra[it.ID]
		d.ID, d.Title = it.ID, it.Title
		if in.Kind == "chat_history" {
			d.FirstQuestion = s.firstQuestion(ctx, nb, it.ID)
		}
		titles = append(titles, d)
	}
	if !in.Confirm {
		return DeleteOut{Status: "preview", Kind: in.Kind, Items: titles,
			Guidance: "Nothing was deleted. Repeat the call with confirm:true to delete these items."}, nil
	}
	results, err := s.NLM.Delete(ctx, in.Kind, nb, ids)
	if in.Kind == "notebook" {
		for _, id := range ids {
			s.invalidateNotebook(id)
		}
	} else {
		s.invalidateNotebook(nb)
	}
	if err != nil {
		return DeleteOut{}, err
	}
	return deleteOutcome(in.Kind, titles, results)
}

// deletableItems lists the items of one notebook-scoped kind, fresh from the
// server: sources (rLM1Ne), notes (cFji9) or conversations (hPTbtc, no
// titles). For conversations extra carries created and updated times, and
// fullPage reports a full first page (older conversations may exist but
// cannot be listed).
func (s *server) deletableItems(ctx context.Context, kind, nb string) (items []resolve.Item, extra map[string]DeleteItem, fullPage bool, err error) {
	switch kind {
	case "source":
		srcs, err := s.NLM.ListSources(ctx, nb)
		if err != nil {
			return nil, nil, false, err
		}
		for _, x := range srcs {
			items = append(items, resolve.Item{ID: x.ID, Title: x.Title})
		}
	case "note":
		notes, err := s.NLM.ListNotes(ctx, nb)
		if err != nil {
			return nil, nil, false, err
		}
		for _, n := range notes {
			items = append(items, resolve.Item{ID: n.ID, Title: n.Title})
		}
	case "chat_history":
		convs, err := s.NLM.ChatSessions(ctx, nb)
		if err != nil {
			return nil, nil, false, err
		}
		extra = make(map[string]DeleteItem, len(convs))
		for _, c := range convs {
			items = append(items, resolve.Item{ID: c.ID})
			extra[c.ID] = DeleteItem{Created: c.Created, Updated: c.Updated}
		}
		fullPage = len(convs) >= nlm.ChatSessionsPage
	}
	return items, extra, fullPage, nil
}

// firstQuestionPages caps the khqZz pages read to find a first question.
const firstQuestionPages = 5

// firstQuestion returns a conversation's first user question, best effort:
// "" unless the whole history was read within firstQuestionPages pages
// (khqZz turn layout and cursor are catalog consensus, not live-verified)
// or on any error.
func (s *server) firstQuestion(ctx context.Context, nb, conv string) string {
	cursor := ""
	for range firstQuestionPages {
		turns, next, err := s.NLM.History(ctx, nb, conv, 20, cursor)
		if err != nil {
			return ""
		}
		if next == "" { // last page: oldest turns, oldest first
			for _, t := range turns {
				if t.Role == "user" {
					return t.Text
				}
			}
			return ""
		}
		cursor = next
	}
	return ""
}

// deleteOutcome merges per-id results into the tool output. When every id
// failed, the first error is returned so the call surfaces as an error
// envelope; nothing was changed then.
func deleteOutcome(kind string, titles []DeleteItem, results []nlm.DeleteResult) (DeleteOut, error) {
	byID := make(map[string]nlm.DeleteResult, len(results))
	for _, r := range results {
		byID[r.ID] = r
	}
	out := DeleteOut{Kind: kind, Items: make([]DeleteItem, 0, len(titles))}
	deleted := 0
	var firstErr error
	for _, it := range titles {
		r, ok := byID[it.ID]
		if !ok {
			r = nlm.DeleteResult{ID: it.ID, Status: "failed", Err: errors.New("no result for this id")}
		}
		it.Status = r.Status
		if r.Err != nil {
			it.Error = safety.Redact(r.Err.Error())
			if firstErr == nil {
				firstErr = r.Err
			}
		} else {
			deleted++
		}
		out.Items = append(out.Items, it)
	}
	switch {
	case deleted == len(out.Items):
		out.Status = "deleted"
	case deleted == 0:
		return DeleteOut{}, firstErr
	default:
		out.Status = "partial"
		out.Guidance = "Some items were not deleted; see each item's status and error."
	}
	return out, nil
}
