package nlm

import (
	"context"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Note is one saved note of a notebook.
type Note struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Content string    `json:"content"`
	Updated time.Time `json:"updated,omitzero"`
}

// NoteInput creates a note (empty ID) or rewrites one (set ID).
type NoteInput struct {
	ID, Title, Content string
}

// Live capture 2026-10-13 (cFji9): params [notebook_id, null, null, HDR].
func listNotesParams(nb string) []any { return []any{nb, nil, nil, requestOptions()} }

// createNoteParams is pinned by live capture #2 (2026-10-20, CYK0Xb from the
// "Copied text" flow, which now creates a note): [nb, content,
// [1,null×5,1], null, title, null, HDR]. A chat answer saved as a note uses
// kind 2 plus the answer's passages and rich block at [3] and [5]; see
// answerNoteParams.
func createNoteParams(nb, title, content string) []any {
	return []any{nb, content, []any{1, nil, nil, nil, nil, nil, 1}, nil, title, nil, requestOptions()}
}

// mutateNoteParams is pinned by live capture #2 (2026-10-20, cYAfTb title
// edit): [nb, note, [[[content, title, [], 0, null, 1]]], HDR].
func mutateNoteParams(nb, id, title, content string) []any {
	return []any{nb, id, []any{[]any{[]any{content, title, []any{}, 0, nil, 1}}}, requestOptions()}
}

// ListNotes lists a notebook's notes (cFji9). Rows are [id,[id,content,meta,null,title]];
// deleted notes stay as tombstones [id,null,2] and are filtered out. The
// Updated time is read best-effort from meta[2] as [seconds,nanos] and may
// be zero (live-unverified).
func (c *Client) ListNotes(ctx context.Context, nb string) ([]Note, error) {
	p, err := c.do(ctx, "notes.list", listNotesParams(nb), callOpts{sourcePath: notebookPath(nb), idempotent: true, null: wire.NullAllowed})
	if err != nil {
		return nil, err
	}
	out := []Note{}
	for _, r := range wire.Elems(p, 0) {
		if n, ok := noteFromRow(r); ok {
			out = append(out, n)
		}
	}
	return out, nil
}

// noteFromRow decodes one cFji9 row if it is a note. Live notes (2026-10-13,
// 2026-10-20) have meta [kind, owner, updated, null, null, created, true]
// with kind 1 (text note) or 2 (saved chat answer) and meta[3] null. The
// catalog's note-backed mind maps are created with meta
// [2,null,null,5,[[sid]..]]: a non-null meta[3], or a kind other than 1 or
// 2, marks a row that is not a note, and it is skipped.
func noteFromRow(r []byte) (Note, bool) {
	id := wire.Str(r, 0)
	if id == "" || len(wire.Elems(r, 1)) == 0 { // tombstone [id,null,2]
		return Note{}, false
	}
	if m, ok := wire.At(r, 1, 2, 3); ok && string(m) != "null" {
		return Note{}, false
	}
	if kind, ok := wire.Int(r, 1, 2, 0); ok && kind != 1 && kind != 2 {
		return Note{}, false
	}
	return Note{ID: id, Content: wire.Str(r, 1, 1), Title: wire.Str(r, 1, 4), Updated: stamp(r, 1, 2, 2)}, true
}

// SaveNote creates a note (empty ID: one CYK0Xb call with title and
// content, answered by the new note row [[id, content, meta, null, title]])
// or rewrites an existing one (cYAfTb). Neither call is retried: a resent
// create would leave a second note.
func (c *Client) SaveNote(ctx context.Context, nb string, n NoteInput) (Note, error) {
	opts := callOpts{sourcePath: notebookPath(nb), null: wire.NullAllowed}
	if n.ID != "" {
		if _, err := c.do(ctx, "notes.mutate", mutateNoteParams(nb, n.ID, n.Title, n.Content), opts); err != nil {
			return Note{ID: n.ID}, err
		}
		return Note{ID: n.ID, Title: n.Title, Content: n.Content}, nil
	}
	p, err := c.do(ctx, "notes.create", createNoteParams(nb, n.Title, n.Content), opts)
	if err != nil {
		return Note{}, err
	}
	id := wire.Str(p, 0, 0)
	if id == "" {
		return Note{}, &transport.Error{Kind: transport.KindDrift, RPC: "notes.create", Message: "create answered without a note id"}
	}
	return Note{ID: id, Title: n.Title, Content: n.Content}, nil
}
