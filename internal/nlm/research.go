package nlm

import (
	"context"
	"slices"
	"strings"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Research modes accepted by StartResearch.
const (
	ModeDiscover = "discover"
	ModeFast     = "fast"
	ModeDeep     = "deep"
)

// Research corpora accepted by StartResearch.
const (
	CorpusWeb   = "web"
	CorpusDrive = "drive"
)

// Candidate kinds accepted by ImportResearch.
const (
	CandWeb    = "web"
	CandDrive  = "drive"
	CandReport = "report"
)

// AddSkipped is the ImportResearch outcome for a pick that was not sent
// because the notebook already holds it.
const AddSkipped = "skipped"

// ResearchStatus is the raw status int at info[4] of a poll row.
//
// RULING: the catalog is conflicting (teng-lin: 1 running / 2 done /
// 3 no results / 4 cancelled; JB, ARL, MOOD: 2 or 6 = done). The raw value
// is kept and Done() treats both 2 and 6 as done. The remaining values need
// a live capture of a non-empty e3bVqc answer (catalog open item 16).
type ResearchStatus int

// Done reports whether the run finished (2 or 6).
func (s ResearchStatus) Done() bool { return s == 2 || s == 6 }

// Candidate is one research result, or one pick to import.
type Candidate struct {
	Kind        string `json:"kind"` // web|drive|report
	URL         string `json:"url,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	DriveID     string `json:"drive_id,omitempty"`
	MIME        string `json:"mime,omitempty"`
	Report      string `json:"report,omitempty"` // markdown, kind report only
}

// ResearchJob is one research run.
//
// ID is the task id: for fast research the Ljjv0c answer; for deep research
// the first element of the QA9ei answer. AltID is the second QA9ei element.
// The catalog is conflicting on which of the two is the task id and which
// is the report/conversation id (item 15), so both are returned raw and the
// job is found in the poll list (PollResearch) by query.
type ResearchJob struct {
	ID         string         `json:"id"`
	AltID      string         `json:"alt_id,omitempty"`
	Mode       string         `json:"mode"`
	ModeCode   int            `json:"mode_code,omitempty"`
	Query      string         `json:"query,omitempty"`
	Status     ResearchStatus `json:"status"`
	Candidates []Candidate    `json:"candidates,omitempty"`
	Report     string         `json:"report,omitempty"`
}

// CandidatesByURL picks the job's web candidates by URL, in the order of
// urls (trailing slash ignored, duplicates dropped). Poll rows carry no
// result id and their order can change between polls, so the URL is the
// only stable handle. missing lists the urls with no candidate.
func (j ResearchJob) CandidatesByURL(urls []string) (picks []Candidate, missing []string) {
	seen := map[string]bool{}
	for _, u := range urls {
		k := normURL(strings.TrimSpace(u))
		if seen[k] {
			continue
		}
		seen[k] = true
		i := slices.IndexFunc(j.Candidates, func(c Candidate) bool { return c.URL != "" && normURL(c.URL) == k })
		if k == "" || i < 0 {
			missing = append(missing, u)
			continue
		}
		picks = append(picks, j.Candidates[i])
	}
	return picks, missing
}

// Payload builders. All shapes are catalog consensus, not live-verified,
// except pollResearchParams (live capture 2026-10-13).

// shape: catalog consensus, not live-verified (Ljjv0c): [[query, 1 web|2 drive], null, 1, nb]
func fastResearchParams(nb, query string, drive bool) []any {
	corpus := 1
	if drive {
		corpus = 2
	}
	return []any{[]any{query, corpus}, nil, 1, nb}
}

// shape: catalog consensus, not live-verified (QA9ei): [null, [1], [query, 1], 5, nb]
func deepResearchParams(nb, query string) []any {
	return []any{nil, []any{1}, []any{query, 1}, 5, nb}
}

// Live capture 2026-10-13 (e3bVqc): params [HDR, null, notebook_id]. The
// catalog's [null,null,nb] is outdated.
func pollResearchParams(nb string) []any { return []any{requestOptions(), nil, nb} }

// shape: catalog consensus, not live-verified (Zbrupe): [null, null, run_id]
func cancelResearchParams(run string) []any { return []any{nil, nil, run} }

// shape: catalog consensus, not live-verified (LBwxtb): [null, [1], task, nb, [entries]]
func importResearchParams(nb, task string, entries []any) []any {
	return []any{nil, []any{1}, task, nb, entries}
}

// shape: catalog consensus, not live-verified: [null,null,[url,title],null×7,2]
func webEntry(url, title string) []any {
	return []any{nil, nil, []any{url, title}, nil, nil, nil, nil, nil, nil, nil, 2}
}

// shape: catalog consensus, not live-verified: [[id,mime,1,title],null×9,2]
func driveEntry(id, mime, title string) []any {
	return []any{[]any{id, mime, 1, title}, nil, nil, nil, nil, nil, nil, nil, nil, nil, 2}
}

// shape: catalog consensus, not live-verified: [null,[title,md],null,3,null×6,3]
func reportEntry(title, md string) []any {
	return []any{nil, []any{title, md}, nil, 3, nil, nil, nil, nil, nil, nil, 3}
}

func invalid(rpc, msg string) error {
	return &transport.Error{Kind: transport.KindInvalidArgument, RPC: rpc, Message: msg}
}

// StartResearch starts a research run. mode is "fast" or "deep"; "discover"
// (Es3dTe) returns KindUnsupported because its request shape is conflicting
// in the catalog and has no live capture (item 14). corpus is "web"
// (default) or "drive"; deep research is web only. The call is never retried.
// The returned job carries the raw ids; its candidates arrive via
// PollResearch.
func (c *Client) StartResearch(ctx context.Context, nb, query, mode, corpus string) (ResearchJob, error) {
	if mode == ModeDiscover {
		return ResearchJob{}, &transport.Error{Kind: transport.KindUnsupported, RPC: "research.discover", Message: "discover mode is not supported yet: the Es3dTe request shape needs a live capture"}
	}
	if strings.TrimSpace(query) == "" {
		return ResearchJob{}, invalid("research.fast", "research needs a query")
	}
	if corpus == "" {
		corpus = CorpusWeb
	}
	if corpus != CorpusWeb && corpus != CorpusDrive {
		return ResearchJob{}, invalid("research.fast", "corpus must be web or drive")
	}
	opts := callOpts{sourcePath: notebookPath(nb), idempotent: false, null: wire.NullAllowed}
	switch mode {
	case ModeFast, "":
		p, err := c.do(ctx, "research.fast", fastResearchParams(nb, query, corpus == CorpusDrive), opts)
		if err != nil {
			return ResearchJob{}, err
		}
		id := wire.Str(p, 0)
		if id == "" {
			return ResearchJob{}, &transport.Error{Kind: transport.KindInternal, RPC: "research.fast", Message: "research start answered without a task id"}
		}
		return ResearchJob{ID: id, Mode: ModeFast, Query: query, Status: 1}, nil
	case ModeDeep:
		if corpus != CorpusWeb {
			return ResearchJob{}, invalid("research.deep", "deep research searches the web only")
		}
		p, err := c.do(ctx, "research.deep", deepResearchParams(nb, query), opts)
		if err != nil {
			return ResearchJob{}, err
		}
		a, b := wire.Str(p, 0), wire.Str(p, 1)
		if a == "" && b == "" {
			return ResearchJob{}, &transport.Error{Kind: transport.KindInternal, RPC: "research.deep", Message: "research start answered without ids"}
		}
		return ResearchJob{ID: a, AltID: b, Mode: ModeDeep, Query: query, Status: 1}, nil
	}
	return ResearchJob{}, invalid("research.fast", "mode must be fast, deep or discover")
}

// PollResearch lists the notebook's research runs (e3bVqc). An idle
// notebook answers `[]` (live). Row layout (catalog, not live-verified for a
// non-empty answer): [task_id, info, updated, created, account] with info
// [null,[query,type],mode,[[sources],summary],status,deep?]. Mode code 5 is
// deep, else fast. Source rows are [url,title,description]; rows with a null
// url are deep-report rows. The summary becomes Report.
func (c *Client) PollResearch(ctx context.Context, nb string) ([]ResearchJob, error) {
	p, err := c.do(ctx, "research.poll", pollResearchParams(nb), callOpts{sourcePath: notebookPath(nb), idempotent: true, null: wire.NullAllowed})
	if err != nil {
		return nil, err
	}
	rows := wire.Elems(p)
	// The catalog shows both [row..] and [[row..]]; a row starts with a
	// string task id, a wrapper starts with a row (an array).
	if len(rows) > 0 && wire.Str(rows[0], 0) == "" && len(wire.Elems(rows[0], 0)) > 0 {
		rows = wire.Elems(p, 0)
	}
	out := []ResearchJob{}
	for _, r := range rows {
		id := wire.Str(r, 0)
		if id == "" {
			continue
		}
		j := ResearchJob{ID: id, Mode: ModeFast, Query: wire.Str(r, 1, 1, 0)}
		if m, ok := wire.Int(r, 1, 2); ok {
			j.ModeCode = int(m)
			if m == 5 {
				j.Mode = ModeDeep
			}
		}
		if s, ok := wire.Int(r, 1, 4); ok {
			j.Status = ResearchStatus(s)
		}
		j.Report = wire.Str(r, 1, 3, 1)
		for _, s := range wire.Elems(r, 1, 3, 0) {
			cand := Candidate{Kind: CandWeb, URL: wire.Str(s, 0), Title: wire.Str(s, 1), Description: wire.Str(s, 2)}
			if cand.URL == "" && cand.Title == "" {
				continue
			}
			if cand.URL == "" {
				cand.Kind = CandReport
			}
			j.Candidates = append(j.Candidates, cand)
		}
		out = append(out, j)
	}
	return out, nil
}

// CancelResearch cancels a run (Zbrupe; answers `[]`). Cancelling twice is
// harmless, so the call may be retried.
func (c *Client) CancelResearch(ctx context.Context, runID string) error {
	if runID == "" {
		return invalid("research.cancel", "cancel needs a run id")
	}
	_, err := c.do(ctx, "research.cancel", cancelResearchParams(runID), callOpts{idempotent: true, null: wire.NullAllowed})
	return err
}

// ImportResearch imports picks into the notebook (LBwxtb). Picks already in
// the notebook are skipped and reported as AddSkipped, so repeating a
// partly-applied import is safe: web picks match source URLs (trailing slash
// ignored), Drive and report picks match source titles. The LBwxtb call
// itself is sent at most once and never retried. Outcome Index is the
// position in picks.
func (c *Client) ImportResearch(ctx context.Context, nb, jobID string, picks []Candidate) ([]AddOutcome, error) {
	if jobID == "" {
		return nil, invalid("research.import", "import needs a research task id")
	}
	if len(picks) == 0 {
		return nil, invalid("research.import", "no candidates given")
	}
	type item struct {
		entry []any
		key   string // dedupe key: URL for web, title otherwise
		match string // key used to pair the response row (rows carry titles)
	}
	items := make([]item, len(picks))
	for i, p := range picks {
		switch p.Kind {
		case CandWeb:
			if p.URL == "" {
				return nil, invalid("research.import", "web candidate needs a url")
			}
			title := p.Title
			if title == "" {
				title = p.URL
			}
			items[i] = item{webEntry(p.URL, title), p.URL, title}
		case CandDrive:
			if p.DriveID == "" || p.MIME == "" {
				return nil, invalid("research.import", "drive candidate needs drive_id and mime")
			}
			items[i] = item{driveEntry(p.DriveID, p.MIME, p.Title), p.Title, p.Title}
		case CandReport:
			if p.Report == "" {
				return nil, invalid("research.import", "report candidate needs report text")
			}
			items[i] = item{reportEntry(p.Title, p.Report), p.Title, p.Title}
		default:
			return nil, invalid("research.import", "candidate kind must be web, drive or report")
		}
	}
	rows, err := c.listRows(ctx, nb)
	if err != nil {
		return nil, err
	}
	haveURL, haveTitle := map[string]bool{}, map[string]bool{}
	for _, r := range rows {
		if r.URL != "" {
			haveURL[normURL(r.URL)] = true
		}
		if r.Title != "" {
			haveTitle[r.Title] = true
		}
	}
	out := make([]AddOutcome, len(picks))
	var entries []any
	var sent []int
	for i, it := range items {
		dup := it.key != "" && haveTitle[it.key]
		if picks[i].Kind == CandWeb {
			dup = haveURL[normURL(it.key)]
		}
		if dup {
			out[i] = AddOutcome{Index: i, Status: AddSkipped, Error: "already in the notebook"}
			continue
		}
		entries = append(entries, it.entry)
		sent = append(sent, i)
	}
	if len(entries) == 0 {
		return out, nil
	}
	p, err := c.do(ctx, "research.import", importResearchParams(nb, jobID, entries), callOpts{sourcePath: notebookPath(nb), idempotent: false, null: wire.NullAllowed})
	if err != nil {
		return nil, err
	}
	var got []srcRow
	for _, r := range wire.Elems(p, 0) {
		got = append(got, srcRow{ID: wire.Str(r, 0, 0), Title: wire.Str(r, 1)})
	}
	keys := make([]string, len(sent))
	for k, i := range sent {
		keys[k] = items[i].match
	}
	for k, m := range matchRows(keys, got, true) {
		i := sent[k]
		out[i] = AddOutcome{Index: i, Status: AddFailed, Error: "omitted from the server response"}
		if m != nil {
			out[i] = AddOutcome{Index: i, SourceID: m.ID, Status: AddAdded}
		}
	}
	return out, nil
}
