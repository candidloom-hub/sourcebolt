package mcpserver

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/resolve"
	"github.com/candidloom-hub/sourcebolt/internal/safety"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// urlResolver resolves hosts for the SSRF pre-flight; tests replace it.
var urlResolver safety.Resolver

const (
	defaultReadChars = 20000
	maxReadChars     = 200000
)

type TextItem struct {
	Title   string `json:"title" jsonschema:"title shown for the pasted text source"`
	Content string `json:"content" jsonschema:"the text to add"`
}

type DriveItem struct {
	DocumentID string `json:"document_id" jsonschema:"Google Drive file id"`
	MIME       string `json:"mime,omitempty" jsonschema:"Drive MIME type"`
}

type FileItem struct {
	Path string `json:"path" jsonschema:"local file path"`
}

type SourceItem struct {
	URL   string     `json:"url,omitempty" jsonschema:"public http(s) page or YouTube video URL (YouTube is auto-detected)"`
	Text  *TextItem  `json:"text,omitempty" jsonschema:"pasted text source"`
	Drive *DriveItem `json:"drive,omitempty" jsonschema:"Google Drive document (not yet available)"`
	File  *FileItem  `json:"file,omitempty" jsonschema:"local file upload (not yet available)"`
}

type SourceAddIn struct {
	Notebook string       `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Items    []SourceItem `json:"items" jsonschema:"sources to add; each item sets exactly one of url, text, drive, file"`
}

type SourceOutcome struct {
	Index    int    `json:"index"`
	SourceID string `json:"source_id,omitempty"`
	Status   string `json:"status"` // added|pending|failed
	Error    string `json:"error,omitempty"`
}

type SourceAddOut struct {
	Notebook  string          `json:"notebook"`
	Requested int             `json:"requested"`
	Succeeded int             `json:"succeeded"`
	Failed    int             `json:"failed"`
	Pending   int             `json:"pending"`
	Results   []SourceOutcome `json:"results"`
}

type SourceListIn struct {
	Notebook string `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Status   string `json:"status,omitempty" jsonschema:"filter: processing, ready, error, preparing or a raw status number"`
}

type SourceView struct {
	nlm.Source
	StatusName string `json:"status_name"`
}

type SourceListOut struct {
	Sources []SourceView `json:"sources"`
	Total   int          `json:"total"`
}

type SourceReadIn struct {
	Notebook string `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Source   string `json:"source" jsonschema:"source id, id prefix or title"`
	Mode     string `json:"mode,omitempty" jsonschema:"fulltext (default), markdown, or guide (the source guide: summary as text plus keywords)"`
	Offset   int    `json:"offset,omitempty" jsonschema:"character offset to start from"`
	MaxChars int    `json:"max_chars,omitempty" jsonschema:"max characters to return (default 20000, cap 200000)"`
}

type SourceReadOut struct {
	SourceID   string `json:"source_id"`
	Text       string `json:"text"`
	Offset     int    `json:"offset"`
	TotalChars int    `json:"total_chars"`
	Truncated  bool   `json:"truncated"`
	NextOffset int    `json:"next_offset,omitempty"`
	// Keywords is set in mode guide only; Text is then the guide's summary.
	Keywords []string `json:"keywords,omitempty"`
}

type SourceUpdateIn struct {
	Notebook       string `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Source         string `json:"source" jsonschema:"source id, id prefix or title"`
	Title          string `json:"title,omitempty" jsonschema:"new title"`
	Refresh        bool   `json:"refresh,omitempty" jsonschema:"re-fetch the source (not yet available)"`
	CheckFreshness bool   `json:"check_freshness,omitempty" jsonschema:"report whether the source is up to date"`
}

type SourceUpdateOut struct {
	SourceID string `json:"source_id"`
	Renamed  bool   `json:"renamed,omitempty"`
	Fresh    *bool  `json:"fresh,omitempty"`
}

func (s *server) registerSources(m *mcp.Server) {
	f := false
	register(s, m, "sources", &mcp.Tool{
		Name:        "source_add",
		Description: "Add sources to a notebook: public URLs (YouTube auto-detected) and pasted text, in one call. Returns a per-item outcome. Drive and file items are not yet available (UNSUPPORTED_BY_TRANSPORT). URLs to private or internal addresses are rejected.",
		Annotations: &mcp.ToolAnnotations{Title: "Add sources", ReadOnlyHint: false, DestructiveHint: &f, IdempotentHint: false, OpenWorldHint: boolPtr(true)},
	}, s.sourceAdd)
	register(s, m, "sources", &mcp.Tool{
		Name:        "source_list",
		Description: "List a notebook's sources with type and processing status; optionally filter by status.",
		Annotations: readOnly("List sources"),
	}, s.sourceList)
	register(s, m, "sources", &mcp.Tool{
		Name:        "source_read",
		Description: "Read a source's full text, markdown, or guide (summary and keywords), paged by character offset (max_chars default 20000; see truncated and next_offset).",
		Annotations: readOnly("Read source"),
	}, s.sourceRead)
	register(s, m, "sources", &mcp.Tool{
		Name:        "source_update",
		Description: "Rename a source (title) and/or check whether it is up to date (check_freshness). refresh is not yet available (UNSUPPORTED_BY_TRANSPORT).",
		Annotations: &mcp.ToolAnnotations{Title: "Update source", ReadOnlyHint: false, DestructiveHint: &f, IdempotentHint: true, OpenWorldHint: &f},
	}, s.sourceUpdate)
}

func boolPtr(b bool) *bool { return &b }

// validateSourceItems runs the argument checks shared by source_add and
// notebook_create before anything is created, and returns the specs built from
// the validated URLs: what safety.CheckPublicURL checked is exactly what is sent.
func validateSourceItems(ctx context.Context, items []SourceItem) ([]nlm.SourceSpec, error) {
	specs := make([]nlm.SourceSpec, len(items))
	for i, it := range items {
		n := 0
		for _, set := range []bool{it.URL != "", it.Text != nil, it.Drive != nil, it.File != nil} {
			if set {
				n++
			}
		}
		if n != 1 {
			return nil, badInput("items[%d]: set exactly one of url, text, drive, file", i)
		}
		switch {
		case it.Drive != nil:
			return nil, unsupported("sources.add", "Drive sources are not supported yet")
		case it.File != nil:
			return nil, unsupported("sources.add", "file upload is not supported yet")
		case it.Text != nil:
			if strings.TrimSpace(it.Text.Content) == "" {
				return nil, badInput("items[%d]: text.content must not be empty", i)
			}
			specs[i] = nlm.SourceSpec{Text: &nlm.TextSpec{Title: it.Text.Title, Content: it.Text.Content}}
		default:
			validated, err := safety.CheckPublicURL(ctx, it.URL, urlResolver)
			if err != nil {
				return nil, badInput("items[%d]: %v", i, err)
			}
			specs[i] = nlm.SourceSpec{URL: validated.String()}
		}
	}
	return specs, nil
}
func (s *server) sourceAdd(ctx context.Context, in SourceAddIn) (SourceAddOut, error) {
	if len(in.Items) == 0 {
		return SourceAddOut{}, badInput("items must not be empty")
	}
	specs, err := validateSourceItems(ctx, in.Items)
	if err != nil {
		return SourceAddOut{}, err
	}
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return SourceAddOut{}, err
	}
	outcomes, err := s.NLM.AddSources(ctx, nb, specs)
	s.invalidateNotebook(nb)
	if err != nil {
		return SourceAddOut{}, err
	}
	out := SourceAddOut{Notebook: nb, Requested: len(in.Items), Results: make([]SourceOutcome, len(outcomes))}
	for i, o := range outcomes {
		out.Results[i] = SourceOutcome{Index: o.Index, SourceID: o.SourceID, Status: o.Status, Error: o.Error}
		switch o.Status {
		case nlm.AddAdded:
			out.Succeeded++
		case nlm.AddPending:
			out.Pending++
		default:
			out.Failed++
		}
	}
	return out, nil
}

// statusName maps the raw status code (rpc-catalog: 1 processing, 2 ready, 3 error, 5 preparing).
func statusName(st nlm.SourceStatus) string {
	switch st {
	case 1:
		return "processing"
	case 2:
		return "ready"
	case 3:
		return "error"
	case 5:
		return "preparing"
	}
	return "status_" + strconv.Itoa(int(st))
}

func (s *server) sourceList(ctx context.Context, in SourceListIn) (SourceListOut, error) {
	want := strings.ToLower(strings.TrimSpace(in.Status))
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return SourceListOut{}, err
	}
	srcs, err := s.NLM.ListSources(ctx, nb)
	if err != nil {
		return SourceListOut{}, err
	}
	out := SourceListOut{Sources: []SourceView{}}
	for _, src := range srcs {
		name := statusName(src.Status)
		if want != "" && want != name && want != strconv.Itoa(int(src.Status)) {
			continue
		}
		out.Sources = append(out.Sources, SourceView{Source: src, StatusName: name})
	}
	out.Total = len(out.Sources)
	return out, nil
}

func (s *server) resolveSource(ctx context.Context, nb, ref string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", badInput("source must not be empty")
	}
	srcs, err := s.NLM.ListSources(ctx, nb)
	if err != nil {
		return "", err
	}
	items := make([]resolve.Item, len(srcs))
	for i, x := range srcs {
		items[i] = resolve.Item{ID: x.ID, Title: x.Title}
	}
	it, err := resolve.Resolve(ref, items)
	return it.ID, err
}

func (s *server) sourceRead(ctx context.Context, in SourceReadIn) (SourceReadOut, error) {
	var markdown bool
	switch in.Mode {
	case "", "fulltext":
	case "markdown":
		markdown = true
	case "guide":
	default:
		return SourceReadOut{}, badInput("mode must be fulltext, markdown or guide, got %q", in.Mode)
	}
	if in.Offset < 0 || in.MaxChars < 0 {
		return SourceReadOut{}, badInput("offset and max_chars must not be negative")
	}
	limit := in.MaxChars
	if limit == 0 {
		limit = defaultReadChars
	}
	limit = min(limit, maxReadChars)
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return SourceReadOut{}, err
	}
	sid, err := s.resolveSource(ctx, nb, in.Source)
	if err != nil {
		return SourceReadOut{}, err
	}
	var txt string
	var keywords []string
	if in.Mode == "guide" {
		txt, keywords, err = s.NLM.SourceGuide(ctx, nb, sid)
	} else {
		txt, err = s.NLM.SourceText(ctx, nb, sid, markdown)
	}
	if err != nil {
		return SourceReadOut{}, err
	}
	r := []rune(txt)
	out := SourceReadOut{SourceID: sid, Offset: in.Offset, TotalChars: len(r), Keywords: keywords}
	start := min(in.Offset, len(r))
	end := min(start+limit, len(r))
	out.Text = string(r[start:end])
	if end < len(r) {
		out.Truncated, out.NextOffset = true, end
	}
	return out, nil
}

func (s *server) sourceUpdate(ctx context.Context, in SourceUpdateIn) (SourceUpdateOut, error) {
	if in.Title == "" && !in.Refresh && !in.CheckFreshness {
		return SourceUpdateOut{}, badInput("nothing to do: set title, refresh or check_freshness")
	}
	if in.Refresh {
		return SourceUpdateOut{}, unsupported("sources.refresh", "source refresh is not yet available")
	}
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return SourceUpdateOut{}, err
	}
	sid, err := s.resolveSource(ctx, nb, in.Source)
	if err != nil {
		return SourceUpdateOut{}, err
	}
	out := SourceUpdateOut{SourceID: sid}
	if in.Title != "" {
		err := s.NLM.RenameSource(ctx, nb, sid, in.Title)
		s.invalidateNotebook(nb)
		if err != nil {
			return out, err
		}
		out.Renamed = true
	}
	if in.CheckFreshness {
		fresh, err := s.NLM.SourceFresh(ctx, nb, sid)
		if err != nil {
			return out, fmt.Errorf("freshness check: %w", err)
		}
		out.Fresh = &fresh
	}
	return out, nil
}
