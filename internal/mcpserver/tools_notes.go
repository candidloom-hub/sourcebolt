package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/resolve"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// writeAnn builds the annotations of a non-read-only tool.
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

type NoteListIn struct {
	Notebook        string `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	IncludeMindMaps bool   `json:"include_mind_maps,omitempty" jsonschema:"accepted for compatibility; mind maps are not listed yet"`
}

type NoteListOut struct {
	Notes []nlm.Note `json:"notes"`
	Total int        `json:"total"`
}

type NoteSaveIn struct {
	Notebook string `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Note     string `json:"note,omitempty" jsonschema:"existing note id or title to update; omit to create a new note"`
	Title    string `json:"title,omitempty" jsonschema:"note title"`
	Content  string `json:"content,omitempty" jsonschema:"note body"`
	FromTurn string `json:"from_turn,omitempty" jsonschema:"save a chat turn as a note (not available yet)"`
}

func (s *server) registerNotes(m *mcp.Server) {
	register(s, m, "core", &mcp.Tool{
		Name:        "note_list",
		Description: "List the notes of a notebook with id, title and content. Deleted notes are not shown.",
		Annotations: readOnly("List notes"),
	}, s.noteList)

	register(s, m, "core", &mcp.Tool{
		Name:        "note_save",
		Description: "Create a note, or update an existing one (note = id or title). from_turn (save a chat turn as a note) is not available yet and returns UNSUPPORTED_BY_TRANSPORT. The payload shape is not live-verified.",
		Annotations: writeAnn("Save note", false, false, false),
	}, s.noteSave)
}

func (s *server) noteList(ctx context.Context, in NoteListIn) (NoteListOut, error) {
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return NoteListOut{}, err
	}
	notes, err := s.NLM.ListNotes(ctx, nb)
	if err != nil {
		return NoteListOut{}, err
	}
	return NoteListOut{Notes: notes, Total: len(notes)}, nil
}

func (s *server) noteSave(ctx context.Context, in NoteSaveIn) (nlm.Note, error) {
	if in.FromTurn != "" {
		return nlm.Note{}, unsupported("notes.from_turn", "saving a chat turn as a note is not supported yet")
	}
	if in.Note == "" && strings.TrimSpace(in.Title) == "" && strings.TrimSpace(in.Content) == "" {
		return nlm.Note{}, badInput("give a title or content to create a note")
	}
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return nlm.Note{}, err
	}
	ni := nlm.NoteInput{Title: in.Title, Content: in.Content}
	if in.Note != "" {
		notes, err := s.NLM.ListNotes(ctx, nb)
		if err != nil {
			return nlm.Note{}, err
		}
		items := make([]resolve.Item, len(notes))
		for i, n := range notes {
			items[i] = resolve.Item{ID: n.ID, Title: n.Title}
		}
		it, err := resolve.Resolve(in.Note, items)
		if err != nil {
			return nlm.Note{}, err
		}
		ni.ID = it.ID
		for _, n := range notes { // keep fields the caller did not change
			if n.ID == it.ID {
				if ni.Title == "" {
					ni.Title = n.Title
				}
				if ni.Content == "" {
					ni.Content = n.Content
				}
			}
		}
	}
	out, err := s.NLM.SaveNote(ctx, nb, ni)
	s.invalidateNotebook(nb)
	return out, err
}
