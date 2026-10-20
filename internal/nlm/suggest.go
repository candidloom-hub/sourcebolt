package nlm

import (
	"context"
	"encoding/json/jsontext"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Suggestion is one suggested prompt, report idea or studio preset.
// Which fields are set depends on the surface:
//
//	chat:          Title, Prompt
//	report:        Title, Description, Prompt, Audience (1 general, 2 expert)
//	studio_options: Group (0-3, position in the response), Code (type/preset
//	               code, 0 when the row has none), Title, Description
type Suggestion struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	Audience    int    `json:"audience,omitempty"`
	Group       int    `json:"group,omitzero"`
	Code        int    `json:"code,omitempty"`
}

// chatSuggestMode is the otmP3b mode for the chat surface (4).
const chatSuggestMode = 4

// chatSuggestParams: the envelope is pinned by live capture #2 (2026-10-20,
// otmP3b): [HDR, nb, [[sid]..], mode] without a query, [.., mode, null,
// query] with one. The capture used mode 9 (flashcards dialog); chat mode 4
// is catalog-only (TL).
func chatSuggestParams(nb string, sources []string, query string) []any {
	p := []any{requestOptions(), nb, sourceRefs(sources), chatSuggestMode}
	if query != "" {
		p = append(p, nil, query)
	}
	return p
}

// shape: catalog consensus, not live-verified. [[2], nb(, [[sid]..])]
func reportSuggestParams(nb string, sources []string) []any {
	p := []any{[]any{2}, nb}
	if len(sources) > 0 {
		p = append(p, sourceRefs(sources))
	}
	return p
}

func sourceRefs(ids []string) []any {
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		out = append(out, []any{id})
	}
	return out
}

// Suggest returns suggestions for one surface: "chat" (otmP3b), "report"
// (ciyUvf) or "studio_options" (sqTeoe; ignores sources and query).
// "followups" (OcvKNc) is blocked: single-source in the catalog, no live capture.
func (c *Client) Suggest(ctx context.Context, nb, surface string, sources []string, query string) ([]Suggestion, error) {
	opts := callOpts{sourcePath: notebookPath(nb), idempotent: true, null: wire.NullAllowed}
	switch surface {
	case "chat":
		p, err := c.do(ctx, "chat.suggest", chatSuggestParams(nb, sources, query), opts)
		if err != nil {
			return nil, err
		}
		var out []Suggestion
		for _, r := range wire.Elems(p, 0) {
			out = append(out, Suggestion{Title: wire.Str(r, 0), Prompt: wire.Str(r, 1)})
		}
		return out, nil
	case "report":
		p, err := c.do(ctx, "studio.report_suggestions", reportSuggestParams(nb, sources), opts)
		if err != nil {
			return nil, err
		}
		var out []Suggestion
		for _, r := range wire.Elems(p, 0) {
			aud, _ := wire.Int(r, 5)
			out = append(out, Suggestion{Title: wire.Str(r, 0), Description: wire.Str(r, 1), Prompt: wire.Str(r, 4), Audience: int(aud)})
		}
		return out, nil
	case "studio_options":
		opts.sourcePath = "/"
		p, err := c.do(ctx, "studio.choices", studioChoicesParams(), opts)
		if err != nil {
			return nil, err
		}
		return parseStudioChoices(p), nil
	case "followups":
		return nil, &transport.Error{Kind: transport.KindUnsupported, RPC: "chat.next_steps",
			Message: "followups suggestions are blocked on a live capture of OcvKNc"}
	default:
		return nil, invalidArg("suggest", "surface must be chat, report or studio_options")
	}
}

// parseStudioChoices flattens the sqTeoe response: payload[0][g][0] is a list of
// rows. Rows are [code, title, description] (groups 0-2, live) or
// [title, description, x] (group 3, live). Group meaning (audio, video, slide,
// report) is catalog-only and unverified, so the raw position is exposed.
func parseStudioChoices(p jsontext.Value) []Suggestion {
	var out []Suggestion
	for g, grp := range wire.Elems(p, 0) {
		for _, r := range wire.Elems(grp, 0) {
			if code, ok := wire.Int(r, 0); ok {
				out = append(out, Suggestion{Group: g, Code: int(code), Title: wire.Str(r, 1), Description: wire.Str(r, 2)})
			} else {
				out = append(out, Suggestion{Group: g, Title: wire.Str(r, 0), Description: wire.Str(r, 1)})
			}
		}
	}
	return out
}
