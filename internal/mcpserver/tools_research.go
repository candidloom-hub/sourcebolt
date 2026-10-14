package mcpserver

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	minQueryWords   = 4
	maxResearchWait = 120
)

// researchPollEvery is the poll interval of research_status; tests shorten it.
var researchPollEvery = 3 * time.Second

type ResearchStartIn struct {
	Notebook string `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Query    string `json:"query" jsonschema:"what to research; at least 4 words unless force is set"`
	Mode     string `json:"mode,omitempty" jsonschema:"fast (default) or deep; discover is not yet available"`
	Corpus   string `json:"corpus,omitempty" jsonschema:"web (default) or drive; deep supports web only"`
	Force    bool   `json:"force,omitempty" jsonschema:"allow a query shorter than 4 words"`
}

type ResearchStartOut struct {
	Notebook string          `json:"notebook"`
	Job      nlm.ResearchJob `json:"job"`
}

type ResearchStatusIn struct {
	Notebook      string `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Job           string `json:"job,omitempty" jsonschema:"research job id; omit for all jobs of the notebook"`
	WaitSeconds   int    `json:"wait_seconds,omitempty" jsonschema:"0-120: poll every 3 s until the job is done or time runs out"`
	IncludeReport bool   `json:"include_report,omitempty" jsonschema:"include the deep-research report text"`
}

type CandidateView struct {
	Index int `json:"index"`
	nlm.Candidate
}

type JobView struct {
	ID         string          `json:"id"`
	AltID      string          `json:"alt_id,omitempty"`
	Mode       string          `json:"mode"`
	Query      string          `json:"query,omitempty"`
	Status     int             `json:"status"`
	Done       bool            `json:"done"`
	Candidates []CandidateView `json:"candidates,omitempty"`
	Report     string          `json:"report,omitempty"`
}

type ResearchStatusOut struct {
	Notebook string    `json:"notebook"`
	Jobs     []JobView `json:"jobs"`
	Done     bool      `json:"done"`
}

type ResearchImportIn struct {
	Notebook string `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Job      string `json:"job" jsonschema:"research job id from research_start or research_status"`
	Select   []int  `json:"select,omitempty" jsonschema:"candidate indices from research_status to import"`
	All      bool   `json:"all,omitempty" jsonschema:"import every candidate"`
}

type ResearchImportOut struct {
	Notebook string          `json:"notebook"`
	Added    int             `json:"added"`
	Skipped  int             `json:"skipped"`
	Failed   int             `json:"failed"`
	Results  []ImportOutcome `json:"results"`
}

type ImportOutcome struct {
	Index    int    `json:"index"`
	Title    string `json:"title,omitempty"`
	SourceID string `json:"source_id,omitempty"`
	Status   string `json:"status"` // added|skipped|failed
	Error    string `json:"error,omitempty"`
}

func (s *server) registerResearch(m *mcp.Server) {
	f := false
	register(s, m, "research", &mcp.Tool{
		Name:        "research_start",
		Description: "Start a web or Drive research job in a notebook (fast or deep). Returns a job handle; poll with research_status, then import with research_import. Queries under 4 words are rejected unless force is true. Mode discover is not yet available (UNSUPPORTED_BY_TRANSPORT).",
		Annotations: &mcp.ToolAnnotations{Title: "Start research", ReadOnlyHint: false, DestructiveHint: &f, IdempotentHint: false, OpenWorldHint: boolPtr(true)},
	}, s.researchStart)
	register(s, m, "research", &mcp.Tool{
		Name:        "research_status",
		Description: "Check research jobs of a notebook and list their candidates with indices. With wait_seconds (0-120) polls every 3 s, reporting progress, until the job is done.",
		Annotations: readOnly("Research status"),
	}, s.researchStatus)
	register(s, m, "research", &mcp.Tool{
		Name:        "research_import",
		Description: "Import research candidates as notebook sources, by select indices or all. Candidates already in the notebook are skipped, so a retry is safe. Reports added, skipped and failed per candidate.",
		Annotations: &mcp.ToolAnnotations{Title: "Import research", ReadOnlyHint: false, DestructiveHint: &f, IdempotentHint: false, OpenWorldHint: &f},
	}, s.researchImport)
}

func (s *server) researchStart(ctx context.Context, in ResearchStartIn) (ResearchStartOut, error) {
	mode := in.Mode
	if mode == "" {
		mode = nlm.ModeFast
	}
	switch mode {
	case nlm.ModeFast, nlm.ModeDeep:
	case nlm.ModeDiscover:
		return ResearchStartOut{}, unsupported("research.discover", "discover mode is not yet available")
	default:
		return ResearchStartOut{}, badInput("mode must be fast, deep or discover, got %q", in.Mode)
	}
	switch in.Corpus {
	case "", nlm.CorpusWeb, nlm.CorpusDrive:
	default:
		return ResearchStartOut{}, badInput("corpus must be web or drive, got %q", in.Corpus)
	}
	if strings.TrimSpace(in.Query) == "" {
		return ResearchStartOut{}, badInput("query must not be empty")
	}
	if !in.Force && len(strings.Fields(in.Query)) < minQueryWords {
		return ResearchStartOut{}, badInput("query has fewer than %d words; vague queries give poor results. Add detail or set force:true", minQueryWords)
	}
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return ResearchStartOut{}, err
	}
	job, err := s.NLM.StartResearch(ctx, nb, in.Query, mode, in.Corpus)
	if err != nil {
		return ResearchStartOut{}, err
	}
	s.invalidateNotebook(nb)
	return ResearchStartOut{Notebook: nb, Job: job}, nil
}

func matchJob(j nlm.ResearchJob, ref string) bool { return ref == "" || j.ID == ref || j.AltID == ref }

func (s *server) researchStatus(ctx context.Context, in ResearchStatusIn) (ResearchStatusOut, error) {
	if in.WaitSeconds < 0 || in.WaitSeconds > maxResearchWait {
		return ResearchStatusOut{}, badInput("wait_seconds must be between 0 and %d", maxResearchWait)
	}
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return ResearchStatusOut{}, err
	}
	progress := progressFrom(ctx)
	deadline := time.Now().Add(time.Duration(in.WaitSeconds) * time.Second)
	for step := 1.0; ; step++ {
		jobs, err := s.NLM.PollResearch(ctx, nb)
		if err != nil {
			return ResearchStatusOut{}, err
		}
		var sel []nlm.ResearchJob
		for _, j := range jobs {
			if matchJob(j, in.Job) {
				sel = append(sel, j)
			}
		}
		done := len(sel) > 0 && !slices.ContainsFunc(sel, func(j nlm.ResearchJob) bool { return !j.Status.Done() })
		remaining := time.Until(deadline)
		if done || remaining <= 0 {
			return researchStatusOut(nb, sel, done, in.IncludeReport), nil
		}
		progress("research running", step, 0)
		select {
		case <-ctx.Done():
			return ResearchStatusOut{}, ctx.Err()
		case <-time.After(min(researchPollEvery, remaining)):
		}
	}
}

func researchStatusOut(nb string, jobs []nlm.ResearchJob, done, report bool) ResearchStatusOut {
	out := ResearchStatusOut{Notebook: nb, Done: done, Jobs: []JobView{}}
	for _, j := range jobs {
		v := JobView{ID: j.ID, AltID: j.AltID, Mode: j.Mode, Query: j.Query, Status: int(j.Status), Done: j.Status.Done()}
		if report {
			v.Report = j.Report
		}
		for i, c := range j.Candidates {
			if !report {
				c.Report = ""
			}
			v.Candidates = append(v.Candidates, CandidateView{Index: i, Candidate: c})
		}
		out.Jobs = append(out.Jobs, v)
	}
	return out
}

func (s *server) researchImport(ctx context.Context, in ResearchImportIn) (ResearchImportOut, error) {
	if strings.TrimSpace(in.Job) == "" {
		return ResearchImportOut{}, badInput("job must not be empty")
	}
	if !in.All && len(in.Select) == 0 {
		return ResearchImportOut{}, badInput("choose candidates: set select (indices from research_status) or all:true")
	}
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return ResearchImportOut{}, err
	}
	jobs, err := s.NLM.PollResearch(ctx, nb)
	if err != nil {
		return ResearchImportOut{}, err
	}
	idx := slices.IndexFunc(jobs, func(j nlm.ResearchJob) bool { return j.ID == in.Job || j.AltID == in.Job })
	if idx < 0 {
		return ResearchImportOut{}, badInput("research job %q not found in this notebook; call research_status", in.Job)
	}
	job := jobs[idx]
	var chosen []int
	if in.All {
		for i := range job.Candidates {
			chosen = append(chosen, i)
		}
	} else {
		for _, i := range in.Select {
			if i < 0 || i >= len(job.Candidates) {
				return ResearchImportOut{}, badInput("select index %d out of range (candidates: %d)", i, len(job.Candidates))
			}
			if !slices.Contains(chosen, i) {
				chosen = append(chosen, i)
			}
		}
	}
	if len(chosen) == 0 {
		return ResearchImportOut{}, badInput("the job has no candidates yet; wait with research_status")
	}
	picks := make([]nlm.Candidate, len(chosen))
	for k, i := range chosen {
		picks[k] = job.Candidates[i]
		if picks[k].Kind == nlm.CandReport && picks[k].Report == "" {
			picks[k].Report = job.Report
		}
	}
	outcomes, err := s.NLM.ImportResearch(ctx, nb, job.ID, picks)
	s.invalidateNotebook(nb)
	if err != nil {
		return ResearchImportOut{}, err
	}
	out := ResearchImportOut{Notebook: nb, Results: make([]ImportOutcome, len(outcomes))}
	for k, o := range outcomes {
		out.Results[k] = ImportOutcome{Index: chosen[k], Title: picks[k].Title, SourceID: o.SourceID, Status: o.Status, Error: o.Error}
		switch o.Status {
		case nlm.AddAdded:
			out.Added++
		case nlm.AddSkipped:
			out.Skipped++
		default:
			out.Failed++
		}
	}
	return out, nil
}
