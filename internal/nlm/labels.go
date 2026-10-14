package nlm

import (
	"context"
	"fmt"

	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Label is one source label of a notebook.
type Label struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Emoji     string   `json:"emoji,omitempty"`
	SourceIDs []string `json:"source_ids"`
}

// LabelMutation edits one label. Empty fields are left unchanged.
type LabelMutation struct {
	ID            string
	Name          string
	Emoji         string
	AddSources    []string
	RemoveSources []string
}

// Live capture 2026-10-13 (I3xc3c): params [HDR, notebook_id].
func labelsParams(nb string) []any { return []any{requestOptions(), nb} }

// shape: catalog consensus, not live-verified (le8sX):
// [OPTS, nb, label_id, [[name_emoji, sources_add, sources_remove]]]
// rename name_emoji=[name]; emoji [null,emoji]; add [[sid]]; remove [[sid]].
// OPTS is the HDR block since the sibling I3xc3c uses it live.
func mutateLabelParams(nb, label string, nameEmoji, add, remove any) []any {
	entry := []any{nameEmoji, add, remove}
	for len(entry) > 1 && entry[len(entry)-1] == nil { // catalog forms drop trailing nulls
		entry = entry[:len(entry)-1]
	}
	return []any{requestOptions(), nb, label, []any{entry}}
}

func sidList(id string) any { return []any{[]any{id}} }

// Labels lists a notebook's labels (I3xc3c). Rows: [name,[[sid]..],label_id,emoji].
// A notebook without labels answers `[]`.
func (c *Client) Labels(ctx context.Context, nb string) ([]Label, error) {
	p, err := c.do(ctx, "labels.list", labelsParams(nb), callOpts{sourcePath: notebookPath(nb), idempotent: true, null: wire.NullAllowed})
	if err != nil {
		return nil, err
	}
	out := []Label{}
	for _, r := range wire.Elems(p, 0) {
		l := Label{Name: wire.Str(r, 0), ID: wire.Str(r, 2), Emoji: wire.Str(r, 3), SourceIDs: []string{}}
		for _, s := range wire.Elems(r, 1) {
			if sid := wire.Str(s, 0); sid != "" {
				l.SourceIDs = append(l.SourceIDs, sid)
			}
		}
		out = append(out, l)
	}
	return out, nil
}

// MutateLabels applies label edits (le8sX; catalog consensus, not live-verified).
// The server honors only the first id per call, so every source id goes out
// in its own call: rename/emoji first, then additions, then removals. All
// mutations are validated before the first call. Calls are not retried.
func (c *Client) MutateLabels(ctx context.Context, nb string, muts []LabelMutation) error {
	for _, m := range muts {
		if m.ID == "" {
			return fmt.Errorf("nlm: label mutation needs an id")
		}
		if m.Name == "" && m.Emoji == "" && len(m.AddSources) == 0 && len(m.RemoveSources) == 0 {
			return fmt.Errorf("nlm: label %s mutation changes nothing", m.ID)
		}
	}
	opts := callOpts{sourcePath: notebookPath(nb), null: wire.NullAllowed}
	send := func(params []any) error {
		_, err := c.do(ctx, "labels.mutate", params, opts)
		return err
	}
	for _, m := range muts {
		if m.Name != "" {
			if err := send(mutateLabelParams(nb, m.ID, []any{m.Name}, nil, nil)); err != nil {
				return err
			}
		}
		if m.Emoji != "" {
			if err := send(mutateLabelParams(nb, m.ID, []any{nil, m.Emoji}, nil, nil)); err != nil {
				return err
			}
		}
		for _, sid := range m.AddSources {
			if err := send(mutateLabelParams(nb, m.ID, nil, sidList(sid), nil)); err != nil {
				return err
			}
		}
		for _, sid := range m.RemoveSources {
			if err := send(mutateLabelParams(nb, m.ID, nil, nil, sidList(sid))); err != nil {
				return err
			}
		}
	}
	return nil
}
