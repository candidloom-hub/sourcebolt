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

// shape: catalog consensus, not live-verified (CYK0Xb, blank note):
// [nb, "", [1], null, "New Note"]. The title is ignored on create.
func createNoteParams(nb string) []any { return []any{nb, "", []any{1}, nil, "New Note"} }

// shape: catalog consensus, not live-verified (cYAfTb): [nb, note, [[[content, title, [], 0]]]].
func mutateNoteParams(nb, id, title, content string) []any {
	return []any{nb, id, []any{[]any{[]any{content, title, []any{}, 0}}}}
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

func noteFromRow(r []byte) (Note, bool) {
	id := wire.Str(r, 0)
	if id == "" || len(wire.Elems(r, 1)) == 0 { // tombstone [id,null,2]
		return Note{}, false
	}
	return Note{ID: id, Content: wire.Str(r, 1, 1), Title: wire.Str(r, 1, 4), Updated: stamp(r, 1, 2, 2)}, true
}

// SaveNote creates a note (empty ID: CYK0Xb blank create, then cYAfTb to set
// title and content) or rewrites an existing one (cYAfTb only). Neither call
// is retried: a resent create would leave a second blank note.
func (c *Client) SaveNote(ctx context.Context, nb string, n NoteInput) (Note, error) {
	opts := callOpts{sourcePath: notebookPath(nb), null: wire.NullAllowed}
	id := n.ID
	if id == "" {
		p, err := c.do(ctx, "notes.create", createNoteParams(nb), opts)
		if err != nil {
			return Note{}, err
		}
		if id = wire.Str(p, 0, 0); id == "" {
			id = wire.Str(p, 0)
		}
		if id == "" {
			return Note{}, &transport.Error{Kind: transport.KindDrift, RPC: "notes.create", Message: "create answered without a note id"}
		}
	}
	if _, err := c.do(ctx, "notes.mutate", mutateNoteParams(nb, id, n.Title, n.Content), opts); err != nil {
		return Note{ID: id}, err
	}
	return Note{ID: id, Title: n.Title, Content: n.Content}, nil
}

// DeleteNotes is blocked: AH0mwd has conflicting shapes (with or without a
// trailing [2]) and mind-map ghost cleanup needs a cFji9 timestamp step.
func (c *Client) DeleteNotes(ctx context.Context, nb string, ids []string) error {
	return blocked("notes.delete", "AH0mwd (trailing [2] conflict, mind-map ghost cleanup)")
}
