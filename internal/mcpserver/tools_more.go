package mcpserver

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/safety"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sync/errgroup"
)

// Notebook create/update, auth and cross-notebook ask.

const (
	defaultAskParallel = 3
	maxAskParallel     = 10
	maxAskNotebooks    = 50
)

type NotebookCreateIn struct {
	Title   string       `json:"title" jsonschema:"title of the new notebook"`
	Emoji   string       `json:"emoji,omitempty" jsonschema:"notebook emoji (ignored: no known wire slot yet)"`
	Sources []SourceItem `json:"sources,omitempty" jsonschema:"sources to add right after creation; same items as source_add"`
}

type NotebookCreateOut struct {
	NotebookID string        `json:"notebook_id"`
	Title      string        `json:"title,omitempty"`
	Sources    *SourceAddOut `json:"sources,omitempty"`
	Warnings   []string      `json:"warnings,omitempty"`
}

type NotebookUpdateIn struct {
	Notebook         string   `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Title            string   `json:"title,omitempty" jsonschema:"new title"`
	RemoveFromRecent bool     `json:"remove_from_recent,omitempty" jsonschema:"drop the notebook from the recently viewed list"`
	Emoji            string   `json:"emoji,omitempty" jsonschema:"not available yet (ignored)"`
	Pinned           *bool    `json:"pinned,omitempty" jsonschema:"not available yet (ignored)"`
	Alias            string   `json:"alias,omitempty" jsonschema:"not available yet (ignored)"`
	Tags             []string `json:"tags,omitempty" jsonschema:"not available yet (ignored)"`
}

type NotebookUpdateOut struct {
	NotebookID        string   `json:"notebook_id"`
	Renamed           bool     `json:"renamed,omitempty"`
	RemovedFromRecent bool     `json:"removed_from_recent,omitempty"`
	Notes             []string `json:"notes,omitempty"`
}

type AuthLoginIn struct {
	Method  string `json:"method" jsonschema:"import (cookies or file), browser or attach"`
	Profile string `json:"profile,omitempty" jsonschema:"account profile name"`
	Host    string `json:"host,omitempty" jsonschema:"NotebookLM host, for example notebook.google.com"`
	File    string `json:"file,omitempty" jsonschema:"path of a cookie export file (method import)"`
}

type AuthLogoutIn struct {
	Profile string `json:"profile,omitempty" jsonschema:"account profile name"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"must be true to delete the stored session"`
}

type AuthOut struct {
	Status string `json:"status"`
}

type ChatAskManyIn struct {
	Question    string   `json:"question" jsonschema:"the question to ask every notebook"`
	Notebooks   []string `json:"notebooks,omitempty" jsonschema:"notebook ids, id prefixes or titles"`
	Tag         string   `json:"tag,omitempty" jsonschema:"not available yet: local tags are not implemented"`
	MaxParallel int      `json:"max_parallel,omitempty" jsonschema:"notebooks asked at once (default 3, max 10)"`
}

type AskManyResult struct {
	Notebook       string        `json:"notebook"`
	NotebookID     string        `json:"notebook_id,omitempty"`
	Status         string        `json:"status"` // ok|failed
	Answer         string        `json:"answer,omitempty"`
	Citations      []CitationOut `json:"citations,omitempty"`
	ConversationID string        `json:"conversation_id,omitempty"`
	Error          string        `json:"error,omitempty"`
	Code           string        `json:"code,omitempty"`
}

type ChatAskManyOut struct {
	Question  string          `json:"question"`
	Asked     int             `json:"asked"`
	Succeeded int             `json:"succeeded"`
	Failed    int             `json:"failed"`
	Results   []AskManyResult `json:"results"`
}

func (s *server) registerMore(m *mcp.Server) {
	register(s, m, "core", &mcp.Tool{
		Name:        "notebook_create",
		Description: "Create a notebook with a title and optionally add sources in the same call (same items as source_add). The create call is sent once and never retried. emoji is accepted but ignored (no known wire slot). If the rename or the source add fails, the notebook still exists and its id is returned with warnings.",
		Annotations: writeAnn("Create notebook", false, false, true),
	}, s.notebookCreate)
	register(s, m, "core", &mcp.Tool{
		Name:        "notebook_update",
		Description: "Rename a notebook (title, verified by read-back) and/or drop it from the recently viewed list (remove_from_recent). emoji, pinned, alias and tags are not available yet and are ignored with a note.",
		Annotations: writeAnn("Update notebook", false, true, false),
	}, s.notebookUpdate)
	register(s, m, "account", &mcp.Tool{
		Name:        "auth_login",
		Description: "Not available through MCP: sign-in needs the local credential store, and cookies should not pass through a chat. Returns UNSUPPORTED_BY_TRANSPORT with the CLI command to run instead (sourcebolt login --import, --browser or --attach).",
		Annotations: writeAnn("Sign in", false, false, false),
	}, s.authLogin)
	register(s, m, "account", &mcp.Tool{
		Name:        "auth_logout",
		Description: "Not available through MCP: the credential store is only reachable from the CLI. Returns UNSUPPORTED_BY_TRANSPORT with the command to run instead (sourcebolt logout).",
		Annotations: writeAnn("Sign out", true, true, false),
	}, s.authLogout)
	register(s, m, groupBatch, &mcp.Tool{
		Name:        "chat_ask_many",
		Description: "Ask the same question in several notebooks (their default conversation thread), at most max_parallel at a time (default 3). Returns one answer with citations per notebook; a notebook that fails does not fail the others. Opt-in tool (batch group). tag is not available yet.",
		Annotations: writeAnn("Ask many notebooks", false, false, false),
	}, s.chatAskMany)
}

func (s *server) notebookCreate(ctx context.Context, in NotebookCreateIn) (NotebookCreateOut, error) {
	if strings.TrimSpace(in.Title) == "" {
		return NotebookCreateOut{}, badInput("title must not be empty")
	}
	if _, err := validateSourceItems(ctx, in.Sources); err != nil {
		return NotebookCreateOut{}, err
	}
	nb, err := s.NLM.CreateNotebook(ctx, in.Title, in.Emoji)
	s.invalidateNotebook(nb.ID)
	out := NotebookCreateOut{NotebookID: nb.ID, Title: nb.Title}
	if err != nil {
		if nb.ID == "" {
			return NotebookCreateOut{}, err
		}
		out.Warnings = append(out.Warnings, "notebook created but title not applied: "+safety.Redact(err.Error()))
	}
	if in.Emoji != "" {
		out.Warnings = append(out.Warnings, "emoji ignored: no known wire slot yet")
	}
	if len(in.Sources) > 0 {
		res, serr := s.sourceAdd(ctx, SourceAddIn{Notebook: nb.ID, Items: in.Sources})
		if serr != nil {
			out.Warnings = append(out.Warnings, "notebook created but sources not added: "+safety.Redact(serr.Error()))
		} else {
			out.Sources = &res
		}
	}
	return out, nil
}

func (s *server) notebookUpdate(ctx context.Context, in NotebookUpdateIn) (NotebookUpdateOut, error) {
	var notes []string
	if in.Emoji != "" {
		notes = append(notes, "emoji ignored: no known wire slot yet")
	}
	if in.Pinned != nil {
		notes = append(notes, "pinned ignored: pin/unpin is not available yet")
	}
	if in.Alias != "" {
		notes = append(notes, "alias ignored: local aliases are not available yet")
	}
	if len(in.Tags) > 0 {
		notes = append(notes, "tags ignored: local tags are not available yet")
	}
	if in.Title == "" && !in.RemoveFromRecent {
		if len(notes) > 0 {
			return NotebookUpdateOut{}, unsupported("notebooks.mutate", "%s", strings.Join(notes, "; "))
		}
		return NotebookUpdateOut{}, badInput("nothing to update: set title and/or remove_from_recent")
	}
	if in.Title != "" && strings.TrimSpace(in.Title) == "" {
		return NotebookUpdateOut{}, badInput("title must not be blank")
	}
	// A mutation: strict match against a fresh list, no title substrings.
	it, err := s.resolveNotebookStrict(ctx, in.Notebook)
	if err != nil {
		return NotebookUpdateOut{}, err
	}
	nb := it.ID
	out := NotebookUpdateOut{NotebookID: nb, Notes: notes}
	if in.Title != "" {
		err := s.NLM.RenameNotebook(ctx, nb, in.Title)
		s.invalidateNotebook(nb)
		if err != nil {
			return out, err
		}
		out.Renamed = true
	}
	if in.RemoveFromRecent {
		err := s.NLM.RemoveFromRecent(ctx, nb)
		s.invalidateNotebook(nb)
		if err != nil {
			return out, err
		}
		out.RemovedFromRecent = true
	}
	return out, nil
}

func (s *server) authLogin(_ context.Context, in AuthLoginIn) (AuthOut, error) {
	hint := "run `sourcebolt login --import <file|->` (cookie export), `sourcebolt login --browser` or `sourcebolt login --attach` in a terminal"
	return AuthOut{}, unsupported("auth.login", "auth_login is not available over MCP (method %q): %s", in.Method, hint)
}

func (s *server) authLogout(_ context.Context, _ AuthLogoutIn) (AuthOut, error) {
	return AuthOut{}, unsupported("auth.logout", "auth_logout is not available over MCP: run `sourcebolt logout` in a terminal")
}

func (s *server) chatAskMany(ctx context.Context, in ChatAskManyIn) (ChatAskManyOut, error) {
	if strings.TrimSpace(in.Question) == "" {
		return ChatAskManyOut{}, badInput("question is empty")
	}
	if in.Tag != "" {
		return ChatAskManyOut{}, unsupported("chat.ask", "tag is not available yet: local tags are not implemented, pass notebooks")
	}
	if len(in.Notebooks) == 0 {
		return ChatAskManyOut{}, badInput("notebooks must list at least one notebook")
	}
	if len(in.Notebooks) > maxAskNotebooks {
		return ChatAskManyOut{}, badInput("at most %d notebooks per call, got %d", maxAskNotebooks, len(in.Notebooks))
	}
	par := in.MaxParallel
	switch {
	case par < 0:
		return ChatAskManyOut{}, badInput("max_parallel must not be negative")
	case par == 0:
		par = defaultAskParallel
	case par > maxAskParallel:
		par = maxAskParallel
	}

	results := make([]AskManyResult, len(in.Notebooks))
	var mu sync.Mutex
	seen := map[string]int{}
	var g errgroup.Group
	g.SetLimit(par)
	for i, ref := range in.Notebooks {
		results[i].Notebook = ref
		g.Go(func() error {
			id, err := s.resolveNotebook(ctx, ref)
			if err == nil {
				mu.Lock()
				first, dup := seen[id]
				if !dup {
					seen[id] = i
				}
				mu.Unlock()
				if dup {
					err = badInput("duplicate of notebooks[%d]", first)
				}
			}
			if err != nil {
				fillAskErr(&results[i], err)
				return nil
			}
			results[i].NotebookID = id
			a, err := s.runAsk(ctx, id, in.Question, nlm.AskOptions{}, nil, false)
			if err != nil {
				fillAskErr(&results[i], err)
				return nil
			}
			results[i].Status = "ok"
			results[i].Answer, results[i].Citations, results[i].ConversationID = a.Answer, a.Citations, a.ConversationID
			return nil
		})
	}
	_ = g.Wait()
	if err := ctx.Err(); err != nil {
		return ChatAskManyOut{}, err
	}
	out := ChatAskManyOut{Question: in.Question, Asked: len(results), Results: results}
	for _, r := range results {
		if r.Status == "ok" {
			out.Succeeded++
		} else {
			out.Failed++
		}
	}
	return out, nil
}

func fillAskErr(r *AskManyResult, err error) {
	r.Status = "failed"
	r.Error, r.Code = safety.Redact(err.Error()), "INTERNAL"
	var ee *envelopeError
	if e := toEnvelope(err); errors.As(e, &ee) {
		r.Error, r.Code = ee.env.Error, ee.env.Code
	}
}
