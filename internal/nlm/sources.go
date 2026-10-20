package nlm

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/candidloom-hub/sourcebolt/internal/safety"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// SourceSpec describes one source to add. Exactly one field is set.
type SourceSpec struct {
	URL   string // web page or YouTube video (auto-detected)
	Text  *TextSpec
	Drive *DriveSpec
}

// TextSpec is pasted text.
type TextSpec struct{ Title, Content string }

// DriveSpec is a Google Drive document. AddSources rejects it until the
// request tail is live-captured (rpc-catalog: izAoDd Drive tail conflicting).
type DriveSpec struct{ DocumentID, MIME, Title string }

// Add outcome statuses.
const (
	AddAdded   = "added"
	AddPending = "pending"
	AddFailed  = "failed"
)

// AddOutcome is the per-spec result of AddSources; Index is the position in
// the specs slice.
type AddOutcome struct {
	Index    int
	SourceID string
	Status   string // added|pending|failed
	Error    string
}

var pendingAdd = []int{3, 9}

// isYouTube reports whether raw is a YouTube video URL.
func isYouTube(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	h := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	h = strings.TrimPrefix(h, "m.")
	switch h {
	case "youtube.com":
		return strings.HasPrefix(u.Path, "/watch") || strings.HasPrefix(u.Path, "/shorts/")
	case "youtu.be":
		return len(u.Path) > 1
	}
	return false
}

// urlSpec: shape pinned by live capture izAoDd/live_2026-10-13_01.json.
func urlSpec(u string) []any {
	return []any{nil, nil, []any{u}, nil, nil, nil, nil, nil, nil, nil, 1}
}

// youtubeSpec: URL at slot 7, pinned by live capture #2 (izAoDd/live_2026-10-20_01.json).
func youtubeSpec(u string) []any {
	return []any{nil, nil, nil, nil, nil, nil, nil, []any{u}, nil, nil, 1}
}

// textSpec: shape: catalog consensus, not live-verified.
func textSpec(title, content string) []any {
	return []any{nil, []any{title, content}, nil, 2, nil, nil, nil, nil, nil, nil, 1}
}

// addSourcesParams: [[spec...], notebook_id, HDR]; the tail is pinned by
// the live URL capture.
func addSourcesParams(nb string, specs []any) []any {
	return []any{specs, nb, requestOptions()}
}

func buildSpec(s SourceSpec) (spec []any, key string, err error) {
	n := 0
	if s.URL != "" {
		n++
	}
	if s.Text != nil {
		n++
	}
	if s.Drive != nil {
		n++
	}
	if n != 1 {
		return nil, "", &transport.Error{Kind: transport.KindInvalidArgument, RPC: "sources.add", Message: "exactly one of URL, Text, Drive must be set"}
	}
	switch {
	case s.Drive != nil:
		return nil, "", &transport.Error{Kind: transport.KindUnsupported, RPC: "sources.add", Message: "Drive sources are not supported yet: the request tail needs a live capture"}
	case s.Text != nil:
		return textSpec(s.Text.Title, s.Text.Content), s.Text.Title, nil
	case isYouTube(s.URL):
		return youtubeSpec(s.URL), s.URL, nil
	}
	return urlSpec(s.URL), s.URL, nil
}

// srcRow is a source row with the URL kept for matching.
type srcRow struct {
	ID, Title, URL string
}

// row: [[sid], title, meta, [null,status]]; meta[7][0] is the source URL.
func parseSrcRow(r jsontext.Value) srcRow {
	return srcRow{ID: wire.Str(r, 0, 0), Title: wire.Str(r, 1), URL: wire.Str(r, 2, 7, 0)}
}

func normURL(s string) string { return strings.TrimRight(s, "/") }

// matchRows pairs each spec (by key: URL or text title) with an unused row.
// With positional set and one row per spec, unmatched specs take the
// same-index row. A spec with no row is nil.
func matchRows(keys []string, rows []srcRow, positional bool) []*srcRow {
	used := make([]bool, len(rows))
	out := make([]*srcRow, len(keys))
	for i, k := range keys {
		for j := range rows {
			if used[j] {
				continue
			}
			if (rows[j].URL != "" && normURL(rows[j].URL) == normURL(k)) || rows[j].Title == k {
				used[j], out[i] = true, &rows[j]
				break
			}
		}
	}
	if positional && len(rows) == len(keys) {
		for i := range keys {
			if out[i] == nil && !used[i] {
				used[i], out[i] = true, &rows[i]
			}
		}
	}
	return out
}

// AddSources adds URL, YouTube and text sources in one izAoDd call. The
// call is never resent. If the server answers code 3 or 9 (accepted, maybe
// applied) the notebook is listed and specs are matched by URL or title:
// matched ones report added, the rest pending. A spec absent from a
// successful response is looked up in the notebook (reconcileOmitted).
// Drive specs return KindUnsupported before any
// call. With c.DOM set, a KindDrift that happened before the request was
// sent adds the specs through the page instead (addSourcesDOM).
func (c *Client) AddSources(ctx context.Context, nb string, specs []SourceSpec) ([]AddOutcome, error) {
	if len(specs) == 0 {
		return nil, &transport.Error{Kind: transport.KindInvalidArgument, RPC: "sources.add", Message: "no sources given"}
	}
	wireSpecs := make([]any, len(specs))
	keys := make([]string, len(specs))
	for i, s := range specs {
		w, k, err := buildSpec(s)
		if err != nil {
			return nil, fmt.Errorf("spec %d: %w", i, err)
		}
		wireSpecs[i], keys[i] = w, k
	}
	p, err := c.do(ctx, "sources.add", addSourcesParams(nb, wireSpecs), callOpts{
		sourcePath: notebookPath(nb), idempotent: false, null: wire.NullAllowed, pending: pendingAdd,
	})
	if err != nil {
		if te, ok := isDrift(err); ok && !te.Sent && c.DOM != nil {
			return c.addSourcesDOM(ctx, nb, specs, keys), nil
		}
		var te *transport.Error
		if !errors.As(err, &te) || te.Kind != transport.KindPending {
			return nil, err
		}
		return c.reconcileAdd(ctx, nb, keys, safety.Redact(te.Error())), nil
	}
	var rows []srcRow
	for _, r := range wire.Elems(p, 0) {
		rows = append(rows, parseSrcRow(r))
	}
	out := make([]AddOutcome, len(specs))
	var missing []int
	answered := map[string]bool{}
	for i, m := range matchRows(keys, rows, true) {
		if m == nil {
			missing = append(missing, i)
			continue
		}
		answered[m.ID] = true
		out[i] = AddOutcome{Index: i, SourceID: m.ID, Status: AddAdded}
	}
	if len(missing) > 0 {
		c.reconcileOmitted(ctx, nb, keys, missing, answered, out)
	}
	return out, nil
}

// reconcileOmitted settles specs that a successful izAoDd answer left out.
// Live capture #2 (2026-10-20) added a website and a YouTube video in one
// call and the answer listed only the website, although the video was
// added. So the notebook is listed (rows from the answer excluded) and each
// missing spec matched by URL or title: found = added, not found = failed,
// listing failed = pending.
func (c *Client) reconcileOmitted(ctx context.Context, nb string, keys []string, missing []int, answered map[string]bool, out []AddOutcome) {
	rows, err := c.listRows(ctx, nb)
	if err != nil {
		for _, i := range missing {
			out[i] = AddOutcome{Index: i, Status: AddPending, Error: "omitted from the server response; listing the notebook failed: " + safety.Redact(err.Error())}
		}
		return
	}
	rows = slices.DeleteFunc(rows, func(r srcRow) bool { return answered[r.ID] })
	mk := make([]string, len(missing))
	for j, i := range missing {
		mk[j] = keys[i]
	}
	for j, m := range matchRows(mk, rows, false) {
		i := missing[j]
		out[i] = AddOutcome{Index: i, Status: AddFailed, Error: "omitted from the server response and not in the notebook"}
		if m != nil {
			out[i] = AddOutcome{Index: i, SourceID: m.ID, Status: AddAdded}
		}
	}
}

// addSourcesDOM adds each spec through the DOM fallback, one at a time,
// after the add RPC drifted before it was sent (an add that may have been
// applied is never repeated). A spec the page may have added before failing
// reports pending, one it never sent reports failed. Source ids of added
// specs are read back from the notebook (best effort), only from rows that
// were not there before: a source the notebook already had with the same
// URL or title is not the new one. Without that snapshot ids stay empty.
func (c *Client) addSourcesDOM(ctx context.Context, nb string, specs []SourceSpec, keys []string) []AddOutcome {
	old, snapErr := c.listRows(ctx, nb)
	out := make([]AddOutcome, len(specs))
	var added []int
	for i, s := range specs {
		var err error
		if s.Text != nil {
			err = c.DOM.AddText(ctx, nb, s.Text.Title, s.Text.Content)
		} else {
			err = c.DOM.AddURL(ctx, nb, s.URL)
		}
		if err != nil {
			status := AddFailed
			var te *transport.Error
			if errors.As(err, &te) && te.Sent {
				status = AddPending
			}
			out[i] = AddOutcome{Index: i, Status: status, Error: DOMTransport + ": " + safety.Redact(err.Error())}
			continue
		}
		out[i] = AddOutcome{Index: i, Status: AddAdded}
		added = append(added, i)
	}
	if len(added) == 0 {
		return out
	}
	if c.OnTransport != nil {
		c.OnTransport(DOMTransport)
	}
	if snapErr != nil {
		return out
	}
	existed := make(map[string]bool, len(old))
	for _, r := range old {
		existed[r.ID] = true
	}
	if rows, err := c.listRows(ctx, nb); err == nil {
		rows = slices.DeleteFunc(rows, func(r srcRow) bool { return existed[r.ID] })
		addedKeys := make([]string, len(added))
		for j, i := range added {
			addedKeys[j] = keys[i]
		}
		for j, m := range matchRows(addedKeys, rows, false) {
			if m != nil {
				out[added[j]].SourceID = m.ID
			}
		}
	}
	return out
}

func (c *Client) reconcileAdd(ctx context.Context, nb string, keys []string, cause string) []AddOutcome {
	out := make([]AddOutcome, len(keys))
	rows, err := c.listRows(ctx, nb)
	if err != nil {
		for i := range out {
			out[i] = AddOutcome{Index: i, Status: AddPending, Error: "server answered pending (" + cause + "); listing failed: " + safety.Redact(err.Error())}
		}
		return out
	}
	for i, m := range matchRows(keys, rows, false) {
		if m == nil {
			out[i] = AddOutcome{Index: i, Status: AddPending, Error: "server answered pending (" + cause + "); not yet in the notebook, not resent"}
			continue
		}
		out[i] = AddOutcome{Index: i, SourceID: m.ID, Status: AddAdded}
	}
	return out
}

// listRows reads rLM1Ne only (no guide) and keeps source URLs, which
// GetNotebook drops.
func (c *Client) listRows(ctx context.Context, nb string) ([]srcRow, error) {
	p, err := c.do(ctx, "notebooks.get", getNotebookParams(nb), callOpts{sourcePath: notebookPath(nb), idempotent: true, null: wire.NullAllowed})
	if err != nil {
		return nil, err
	}
	var rows []srcRow
	for _, r := range wire.Elems(p, 0, 1) {
		rows = append(rows, parseSrcRow(r))
	}
	return rows, nil
}

// ListSources lists a notebook's sources via GetNotebook.
func (c *Client) ListSources(ctx context.Context, nb string) ([]Source, error) {
	d, err := c.GetNotebook(ctx, nb)
	if err != nil {
		return nil, err
	}
	return d.Sources, nil
}
