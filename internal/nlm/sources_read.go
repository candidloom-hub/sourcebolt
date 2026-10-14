package nlm

import (
	"context"
	"encoding/json/jsontext"
	"strings"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// SourceText returns a source's full extracted text (markdown selects the
// markdown variant). Paging is the caller's concern.
//
// shape: catalog consensus, not live-verified. Request [[sid],[2],[2]] for
// text, [[sid],[3],[3]] for markdown. Response: [0]=[[sid],title,meta],
// text blocks at [3][0], HTML at [4][1]. Each block's leaf strings are
// joined, blocks are separated by a newline; when there are no blocks the
// HTML string is returned instead.
func (c *Client) SourceText(ctx context.Context, nb, sid string, markdown bool) (string, error) {
	if err := needSID(sid, "sources.load"); err != nil {
		return "", err
	}
	p, err := c.do(ctx, "sources.load", sourceLoadParams(sid, markdown), callOpts{
		sourcePath: notebookPath(nb), idempotent: true,
	})
	if err != nil {
		return "", err
	}
	root, err := wire.Parse(p)
	if err != nil {
		return "", &transport.Error{Kind: transport.KindDrift, RPC: "sources.load", Message: "response is not valid JSON"}
	}
	var blocks []string
	for _, b := range root.At(3, 0).Elems() {
		if s := leafText(b); s != "" {
			blocks = append(blocks, s)
		}
	}
	if len(blocks) > 0 {
		return strings.Join(blocks, "\n"), nil
	}
	return root.At(4, 1).Str(), nil
}

func sourceLoadParams(sid string, markdown bool) []any {
	mode := 2
	if markdown {
		mode = 3
	}
	return []any{[]any{sid}, []any{mode}, []any{mode}}
}

// leafText concatenates every string leaf under n, depth first.
func leafText(n wire.Node) string {
	if s := n.Str(); s != "" {
		return s
	}
	var sb strings.Builder
	for _, e := range n.Elems() {
		sb.WriteString(leafText(e))
	}
	return sb.String()
}

// RenameSource sets a source's title.
//
// shape: catalog consensus, not live-verified: [null,[sid],[[[title]]]].
func (c *Client) RenameSource(ctx context.Context, nb, sid, title string) error {
	if err := needSID(sid, "sources.rename"); err != nil {
		return err
	}
	if strings.TrimSpace(title) == "" {
		return &transport.Error{Kind: transport.KindInvalidArgument, RPC: "sources.rename", Message: "title must not be empty"}
	}
	_, err := c.do(ctx, "sources.rename", []any{nil, []any{sid}, []any{[]any{[]any{title}}}}, callOpts{
		sourcePath: notebookPath(nb), idempotent: true, null: wire.NullAllowed,
	})
	return err
}

// SourceFresh reports whether a source is up to date.
//
// shape: catalog consensus, not live-verified: [null,[sid],[2]]. A URL
// source answers [] when fresh; a Drive source answers [[null,bool,[id]]]
// where the bool is read as "fresh" (assumption, same catalog row).
func (c *Client) SourceFresh(ctx context.Context, nb, sid string) (bool, error) {
	if err := needSID(sid, "sources.freshness"); err != nil {
		return false, err
	}
	p, err := c.do(ctx, "sources.freshness", []any{nil, []any{sid}, []any{2}}, callOpts{
		sourcePath: notebookPath(nb), idempotent: true,
	})
	if err != nil {
		return false, err
	}
	return freshFromPayload(p)
}

func freshFromPayload(p jsontext.Value) (bool, error) {
	root, err := wire.Parse(p)
	if err != nil {
		return false, &transport.Error{Kind: transport.KindDrift, RPC: "sources.freshness", Message: "response is not valid JSON"}
	}
	row := root.At(0)
	if row.IsNull() {
		return true, nil // [] : URL source, fresh
	}
	if row.At(1).IsNull() {
		return false, &transport.Error{Kind: transport.KindDrift, RPC: "sources.freshness", Message: "unrecognized freshness response"}
	}
	b, _ := wire.At(p, 0, 1) // Node has no bool accessor
	return string(b) == "true", nil
}

// SourceGuide is blocked: tr032e has conflicting request shapes and no live
// capture (rpc-catalog §10 item 10).
func (c *Client) SourceGuide(ctx context.Context, nb, sid string) (summary string, keywords []string, err error) {
	return "", nil, &transport.Error{Kind: transport.KindUnsupported, RPC: "sources.guide",
		Message: "source guide is blocked on a live capture of tr032e (rpc-catalog §10 item 10)"}
}

// RefreshSource is blocked: FLmJqe and dtT1F have conflicting request shapes
// and no live capture (rpc-catalog §10 item 9).
func (c *Client) RefreshSource(ctx context.Context, nb, sid string) error {
	return &transport.Error{Kind: transport.KindUnsupported, RPC: "sources.refresh",
		Message: "source refresh is blocked on a live capture of FLmJqe/dtT1F (rpc-catalog §10 item 9)"}
}

func needSID(sid, rpc string) error {
	if strings.TrimSpace(sid) == "" {
		return &transport.Error{Kind: transport.KindInvalidArgument, RPC: rpc, Message: "source id must not be empty"}
	}
	return nil
}
