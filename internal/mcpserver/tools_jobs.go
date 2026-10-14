package mcpserver

import (
	"context"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/jobs"
	"github.com/candidloom-hub/sourcebolt/internal/safety"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ChatStatusIn struct {
	JobID string `json:"job_id" jsonschema:"job id returned by chat_ask with wait=false"`
}

type ChatStatusOut struct {
	JobID   string    `json:"job_id"`
	Kind    string    `json:"kind"`
	Status  string    `json:"status"`
	Result  any       `json:"result,omitempty"`
	Error   string    `json:"error,omitempty"`
	Started time.Time `json:"started"`
}

type JobCancelIn struct {
	JobID string `json:"job_id" jsonschema:"job id to cancel"`
}

type JobCancelOut struct {
	JobID     string `json:"job_id"`
	Cancelled bool   `json:"cancelled"`
	Status    string `json:"status"`
	Note      string `json:"note,omitempty"`
}

func (s *server) registerJobs(m *mcp.Server) {
	f, t := false, true
	register(s, m, "chat", &mcp.Tool{
		Name:        "chat_status",
		Description: "Status and result of a detached chat_ask (wait=false): running, done, failed or cancelled. Jobs live in memory and are dropped an hour after they finish or when the server restarts.",
		Annotations: readOnly("Chat job status"),
	}, s.chatStatus)

	register(s, m, "core", &mcp.Tool{
		Name:        "job_cancel",
		Description: "Cancel a running detached job (chat_ask with wait=false, deep research). For an ask with a known conversation the server-side generation is also stopped, best effort.",
		Annotations: &mcp.ToolAnnotations{Title: "Cancel job", ReadOnlyHint: false, DestructiveHint: &t, IdempotentHint: true, OpenWorldHint: &f},
	}, s.jobCancel)
}

func (s *server) lookupJob(id string) (jobs.Job, error) {
	if s.Jobs == nil {
		return jobs.Job{}, &transport.Error{Kind: transport.KindUnsupported, RPC: "jobs", Message: "the job table is disabled"}
	}
	if id == "" {
		return jobs.Job{}, badInput("job_id is empty")
	}
	j, ok := s.Jobs.Get(id)
	if !ok {
		return jobs.Job{}, &transport.Error{Kind: transport.KindNotFound, RPC: "jobs", Message: "unknown job " + id + " (finished jobs are dropped after an hour)"}
	}
	return j, nil
}

func (s *server) chatStatus(_ context.Context, in ChatStatusIn) (ChatStatusOut, error) {
	j, err := s.lookupJob(in.JobID)
	if err != nil {
		return ChatStatusOut{}, err
	}
	out := ChatStatusOut{JobID: j.ID, Kind: j.Kind, Status: j.Status, Result: j.Result, Started: j.Started}
	if j.Err != nil {
		out.Error = safety.Redact(j.Err.Error())
	}
	return out, nil
}

func (s *server) jobCancel(ctx context.Context, in JobCancelIn) (JobCancelOut, error) {
	j, err := s.lookupJob(in.JobID)
	if err != nil {
		return JobCancelOut{}, err
	}
	if !s.Jobs.Cancel(in.JobID) {
		cur, _ := s.Jobs.Get(in.JobID)
		return JobCancelOut{JobID: in.JobID, Status: cur.Status, Note: "job was not running"}, nil
	}
	out := JobCancelOut{JobID: in.JobID, Cancelled: true, Status: jobs.StatusCancelled}
	if v, ok := jobConvs.LoadAndDelete(in.JobID); ok && j.Kind == "chat_ask" {
		if err := s.NLM.CancelChat(ctx, v.(string)); err != nil {
			out.Note = "job cancelled locally; server-side stop failed: " + safety.Redact(err.Error())
		}
	}
	return out, nil
}
