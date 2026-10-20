package mcpserver

import (
	"context"
	"strings"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/resolve"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// writeAnn builds the annotations of a non-read-only tool.

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
		Description: "Create a note, or update an existing one (note = id or title). from_turn (save a chat turn as a note) is not available yet and returns UNSUPPORTED_BY_TRANSPORT.",
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
	var nb string
	var err error
	if in.Note == "" {
		nb, err = s.resolveNotebook(ctx, in.Notebook)
	} else {
		// Updating overwrites a note: strict notebook match on a fresh list.
		var it resolve.Item
		it, err = s.resolveNotebookStrict(ctx, in.Notebook)
		nb = it.ID
	}
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
		it, err := resolveStrict(in.Note, items, false) // exact id or exact title only
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
