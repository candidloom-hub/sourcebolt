package nlm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// createNotebookParams is pinned by live capture (2026-10-13,
// testdata/fixtures/CCqFvf/live_2026-10-13_01.json): the web app sends an
// empty title and the HDR options block. The title is set afterwards.
func createNotebookParams() []any { return []any{"", nil, nil, requestOptions()} }

// renameNotebookParams is pinned by live capture #2 (2026-10-20, s0tc2d
// rename): [nb, [[null, null, null, [null, title]]], HDR].
func renameNotebookParams(id, title string) []any {
	return []any{id, []any{[]any{nil, nil, nil, []any{nil, title}}}, requestOptions()}
}

// shape: catalog consensus, not live-verified (fejl7e): [nb].
func removeRecentParams(id string) []any { return []any{id} }

// CreateNotebook creates a notebook (CCqFvf, never retried). The request is
// sent with an empty title, exactly as the web app does; a non-empty title is
// then applied with RenameNotebook. If that rename fails, the returned
// Notebook still carries the new id so the caller can report it.
//
// emoji is accepted but not sent: neither the live create request nor the
// consensus s0tc2d rename defines an emoji slot (only the TMC proto does), so
// it is ignored.
func (c *Client) CreateNotebook(ctx context.Context, title, emoji string) (Notebook, error) {
	_ = emoji // no wire slot known; see doc comment
	p, err := c.do(ctx, "notebooks.create", createNotebookParams(), callOpts{null: wire.NullAllowed})
	if err != nil {
		return Notebook{}, err
	}
	id := wire.Str(p, 2)
	if id == "" {
		return Notebook{}, &transport.Error{Kind: transport.KindInternal, RPC: "notebooks.create", Message: "create returned no notebook id"}
	}
	nb := Notebook{ID: id, Owned: true}
	if strings.TrimSpace(title) == "" {
		return nb, nil
	}
	if err := c.RenameNotebook(ctx, id, title); err != nil {
		return nb, err
	}
	nb.Title = title
	return nb, nil
}

// RenameNotebook sets a notebook's title (s0tc2d) and reads it back; a title
// that did not stick is a KindInternal error. The mutation is never retried.
func (c *Client) RenameNotebook(ctx context.Context, id, title string) error {
	if id == "" {
		return invalidArg("notebooks.mutate", "no notebook id given")
	}
	if strings.TrimSpace(title) == "" {
		return invalidArg("notebooks.mutate", "title is empty")
	}
	if _, err := c.do(ctx, "notebooks.mutate", renameNotebookParams(id, title), callOpts{sourcePath: notebookPath(id), null: wire.NullAllowed}); err != nil {
		return err
	}
	d, err := c.GetNotebook(ctx, id)
	if err != nil {
		return err
	}
	if d.Title != title {
		return &transport.Error{Kind: transport.KindInternal, RPC: "notebooks.mutate",
			Message: fmt.Sprintf("rename not applied: notebook title is %q, wanted %q", d.Title, title)}
	}
	return nil
}

// RemoveFromRecent drops a notebook from the recently viewed list (fejl7e).
// The server may answer gRPC INTERNAL (code 13) even on success, so only that
// code is treated as success (rpc-catalog, TL). Other KindInternal errors,
// such as HTTP 4xx answers, surface.
func (c *Client) RemoveFromRecent(ctx context.Context, id string) error {
	if id == "" {
		return invalidArg("notebooks.unrecent", "no notebook id given")
	}
	_, err := c.do(ctx, "notebooks.unrecent", removeRecentParams(id), callOpts{null: wire.NullAllowed})
	var te *transport.Error
	if errors.As(err, &te) && te.Kind == transport.KindInternal && te.GRPCCode == 13 {
		return nil
	}
	return err
}
