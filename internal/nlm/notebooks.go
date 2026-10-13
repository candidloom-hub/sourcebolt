package nlm

import (
	"context"
	"encoding/json/jsontext"
	"sync"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/safety"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Notebook is one row of the notebook list.
type Notebook struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Emoji       string    `json:"emoji,omitempty"`
	SourceCount int       `json:"source_count"`
	Owned       bool      `json:"owned"`
	Created     time.Time `json:"created,omitzero"`
	Modified    time.Time `json:"modified,omitzero"`
}

// SourceStatus is the raw processing status at source[3][1]. Its value
// mapping is pinned by live capture (rpc-catalog §10).
type SourceStatus int

// Source is one source inside a notebook.
type Source struct {
	ID     string       `json:"id"`
	Title  string       `json:"title"`
	Type   string       `json:"type,omitempty"`
	Status SourceStatus `json:"status"`
}

// NotebookDetail is the composite read behind notebook_open.
type NotebookDetail struct {
	Notebook
	Sources   []Source `json:"sources"`
	Summary   string   `json:"summary,omitempty"`
	Suggested []string `json:"suggested_questions,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

// Payload shapes pinned by live capture (2026-10-13, notebook.google.com).
// See testdata/fixtures/<rpc>/live_*.json and TestLiveRequestShapes.
func listNotebooksParams() []any {
	return []any{nil, 1, nil, requestOptions(), nil, []any{[]any{nil, nil, []any{}}, []any{[]any{}}, []any{nil, []any{}}}}
}

func getNotebookParams(id string) []any {
	return []any{id, nil, requestOptions(), nil, 0, []any{[]any{nil, nil, []any{}}}}
}

func guideParams(id string) []any { return []any{id, requestOptions()} }

// ListNotebooks returns the account's notebooks (wXbhsf).
func (c *Client) ListNotebooks(ctx context.Context) ([]Notebook, error) {
	p, err := c.do(ctx, "notebooks.list", listNotebooksParams(), callOpts{idempotent: true, null: wire.NullAllowed})
	if err != nil {
		return nil, err
	}
	rows := wire.Elems(p, 0)
	out := make([]Notebook, 0, len(rows))
	for _, r := range rows {
		out = append(out, parseNotebookRow(r))
	}
	return out, nil
}

// row: [title, sources, id, emoji, null, meta]; meta[0] 1 mine / 2 shared,
// meta[5] modified [s, ns], meta[8] created [s, ns].
func parseNotebookRow(r jsontext.Value) Notebook {
	n := Notebook{
		Title:       wire.Str(r, 0),
		SourceCount: len(wire.Elems(r, 1)),
		ID:          wire.Str(r, 2),
		Emoji:       wire.Str(r, 3),
	}
	if own, ok := wire.Int(r, 5, 0); ok {
		n.Owned = own == 1
	}
	n.Modified = stamp(r, 5, 5)
	n.Created = stamp(r, 5, 8)
	return n
}

func stamp(v jsontext.Value, path ...int) time.Time {
	s, ok := wire.Int(v, append(path, 0)...)
	if !ok {
		return time.Time{}
	}
	ns, _ := wire.Int(v, append(path, 1)...)
	return time.Unix(s, ns)
}

// GetNotebook reads a notebook with its sources (rLM1Ne) and, in parallel,
// its summary and suggested questions (VfAZjd). A failing guide degrades to
// an empty summary with a warning; a failing main read fails the call.
func (c *Client) GetNotebook(ctx context.Context, id string) (NotebookDetail, error) {
	var (
		wg    sync.WaitGroup
		guide jsontext.Value
		gerr  error
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		guide, gerr = c.do(ctx, "notebooks.guide", guideParams(id), callOpts{sourcePath: notebookPath(id), idempotent: true, null: wire.NullAllowed})
	}()
	p, err := c.do(ctx, "notebooks.get", getNotebookParams(id), callOpts{sourcePath: notebookPath(id), idempotent: true, null: wire.NullAllowed})
	wg.Wait()
	if err != nil {
		return NotebookDetail{}, err
	}
	d := NotebookDetail{Notebook: Notebook{ID: id}, Sources: []Source{}}
	if row, ok := wire.At(p, 0); ok {
		d.Notebook = parseNotebookRow(row)
		for _, s := range wire.Elems(row, 1) {
			d.Sources = append(d.Sources, parseSource(s))
		}
		d.SourceCount = len(d.Sources)
		if d.ID == "" {
			d.ID = id
		}
	}
	if gerr != nil {
		d.Warnings = append(d.Warnings, "summary unavailable: "+safety.Redact(gerr.Error()))
	} else {
		d.Summary = wire.Str(guide, 0, 0, 0)
		for _, q := range wire.Elems(guide, 0, 1) {
			if s := wire.Str(q, 0); s != "" {
				d.Suggested = append(d.Suggested, s)
			}
		}
	}
	return d, nil
}

// source: [[sid], title, meta, [null, status]]
func parseSource(s jsontext.Value) Source {
	st, _ := wire.Int(s, 3, 1)
	return Source{ID: wire.Str(s, 0, 0), Title: wire.Str(s, 1), Status: SourceStatus(st)}
}
