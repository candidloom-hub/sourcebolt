package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/candidloom-hub/sourcebolt/internal/jobs"
	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/resolve"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ChatAskIn struct {
	Notebook        string   `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Question        string   `json:"question" jsonschema:"the question to ask"`
	Sources         []string `json:"sources,omitempty" jsonschema:"restrict the answer to these sources (id, id prefix or title); default all sources"`
	ConversationID  string   `json:"conversation_id,omitempty" jsonschema:"continue this conversation; default is the notebook's newest thread"`
	NewConversation bool     `json:"new_conversation,omitempty" jsonschema:"start a fresh conversation instead of the default thread"`
	Wait            *bool    `json:"wait,omitempty" jsonschema:"wait for the answer (default true); false returns a job_id to poll with chat_status"`
}

// CitationOut is one cited passage. Start and End are UTF-16 code unit
// offsets into the answer text.
type CitationOut struct {
	N           int    `json:"n"`
	SourceID    string `json:"source_id"`
	SourceTitle string `json:"source_title,omitempty"`
	Text        string `json:"text,omitempty"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
}

type ChatAskOut struct {
	Answer         string        `json:"answer,omitempty"`
	Citations      []CitationOut `json:"citations,omitempty"`
	ConversationID string        `json:"conversation_id,omitempty"`
	ResponseID     string        `json:"response_id,omitempty"`
	JobID          string        `json:"job_id,omitempty"`
	Status         string        `json:"status,omitempty"`
}

type ChatHistoryIn struct {
	Notebook       string `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	ConversationID string `json:"conversation_id,omitempty" jsonschema:"conversation to read; default is the newest thread"`
	Limit          int    `json:"limit,omitempty" jsonschema:"max turns per page (default 20)"`
	Cursor         string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous page (older turns)"`
	Format         string `json:"format,omitempty" jsonschema:"json (default) or markdown"`
}

type ChatHistoryOut struct {
	ConversationID string     `json:"conversation_id,omitempty"`
	Turns          []nlm.Turn `json:"turns,omitempty"`
	NextCursor     string     `json:"next_cursor,omitempty"`
	Markdown       string     `json:"markdown,omitempty"`
}

type ChatConfigureIn struct {
	Notebook       string `json:"notebook" jsonschema:"notebook id, id prefix (6+ chars) or title"`
	Mode           string `json:"mode,omitempty" jsonschema:"conversational goal; custom needs custom_prompt"`
	CustomPrompt   string `json:"custom_prompt,omitempty" jsonschema:"instructions for mode custom, up to 10000 characters"`
	ResponseLength string `json:"response_length,omitempty" jsonschema:"response length"`
}

type ChatConfigureOut struct {
	Notebook string `json:"notebook"`
	Mode     string `json:"mode,omitempty"`
	Length   string `json:"response_length,omitempty"`
	Updated  bool   `json:"updated"`
}

type SuggestIn struct {
	Notebook string   `json:"notebook,omitempty" jsonschema:"notebook id, id prefix or title (not needed for studio_options)"`
	Surface  string   `json:"surface" jsonschema:"chat, followups, report or studio_options"`
	Sources  []string `json:"sources,omitempty" jsonschema:"limit to these sources (id, id prefix or title)"`
	Query    string   `json:"query,omitempty" jsonschema:"steer chat suggestions toward this topic"`
}

type SuggestOut struct {
	Suggestions []nlm.Suggestion `json:"suggestions"`
}

func (s *server) registerChat(m *mcp.Server) {
	f := false
	register(s, m, "chat", &mcp.Tool{
		Name:        "chat_ask",
		Description: "Ask a question about a notebook's sources. Streams partial text as progress notifications. Returns the answer, citations (source, cited text, UTF-16 offsets) and the conversation id. wait=false returns a job_id; poll with chat_status. Not retried after sending.",
		Annotations: &mcp.ToolAnnotations{Title: "Ask a notebook", ReadOnlyHint: false, DestructiveHint: &f, IdempotentHint: false, OpenWorldHint: &f},
	}, s.chatAsk)

	register(s, m, "chat", &mcp.Tool{
		Name:        "chat_history",
		Description: "Read a notebook conversation (oldest turn first within a page), with cursor paging. format=markdown adds a rendered transcript. Turn layout is not yet live-verified.",
		Annotations: readOnly("Chat history"),
	}, s.chatHistory)

	register(s, m, "chat", &mcp.Tool{
		Name:        "chat_configure",
		Description: "Change a notebook's chat settings: mode (learning_guide, or custom with custom_prompt) and response_length (longer or shorter). Unset fields keep their value; both are always sent, so on a notebook whose current mode or length is the default, pass both. The web app's default mode and default length are not live-verified and return UNSUPPORTED_BY_TRANSPORT.",
		Annotations: &mcp.ToolAnnotations{Title: "Configure chat", ReadOnlyHint: false, DestructiveHint: &f, IdempotentHint: true, OpenWorldHint: &f},
		InputSchema: chatConfigureSchema(),
	}, s.chatConfigure)

	register(s, m, "chat", &mcp.Tool{
		Name:        "suggest",
		Description: "Suggested prompts for a surface: chat, report or studio_options. The followups surface is not yet available (UNSUPPORTED_BY_TRANSPORT until a live capture pins it).",
		Annotations: readOnly("Suggestions"),
	}, s.suggest)
}

// resolveSources turns source refs (id, prefix or title) into ids. Empty
// refs give nil.
func (s *server) resolveSources(ctx context.Context, nb string, refs []string) ([]string, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	srcs, err := s.NLM.ListSources(ctx, nb)
	if err != nil {
		return nil, err
	}
	items := make([]resolve.Item, len(srcs))
	for i, x := range srcs {
		items[i] = resolve.Item{ID: x.ID, Title: x.Title}
	}
	ids := make([]string, 0, len(refs))
	for _, r := range refs {
		it, err := resolve.Resolve(r, items)
		if err != nil {
			return nil, err
		}
		ids = append(ids, it.ID)
	}
	return ids, nil
}

// maxDetachedAsks caps concurrently running chat_ask wait=false jobs.
const maxDetachedAsks = 16

// metaConversation is the job Meta key holding a detached ask's explicit
// conversation id, so job_cancel can stop the server-side generation.
const metaConversation = "conversation_id"

func (s *server) chatAsk(ctx context.Context, in ChatAskIn) (ChatAskOut, error) {
	if strings.TrimSpace(in.Question) == "" {
		return ChatAskOut{}, badInput("question is empty")
	}
	if in.NewConversation && in.ConversationID != "" {
		return ChatAskOut{}, badInput("new_conversation and conversation_id are mutually exclusive")
	}
	wait := in.Wait == nil || *in.Wait
	if !wait && s.Jobs == nil {
		return ChatAskOut{}, &transport.Error{Kind: transport.KindUnsupported, RPC: "chat.ask", Message: "wait=false needs the job table, which is disabled"}
	}
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return ChatAskOut{}, err
	}
	srcIDs, err := s.resolveSources(ctx, nb, in.Sources)
	if err != nil {
		return ChatAskOut{}, err
	}
	opts := nlm.AskOptions{SourceIDs: srcIDs, ConversationID: in.ConversationID, NewConversation: in.NewConversation}

	if !wait {
		jopts := []jobs.StartOption{jobs.Limit(maxDetachedAsks)}
		if in.ConversationID != "" {
			jopts = append(jopts, jobs.Meta(metaConversation, in.ConversationID))
		}
		// The job context derives from the table's lifetime, not from this
		// request: it ends on job_cancel or when the server closes the table.
		id, err := s.Jobs.Submit("chat_ask", func(jctx context.Context) (any, error) {
			return s.runAsk(jctx, nb, in.Question, opts, nil)
		}, jopts...)
		switch {
		case errors.Is(err, jobs.ErrBusy):
			return ChatAskOut{}, &transport.Error{Kind: transport.KindRateLimited, RPC: "chat.ask", Message: fmt.Sprintf(
				"%d detached chat_ask jobs are already running; wait for one to finish (chat_status), cancel one (job_cancel), or ask with wait=true", maxDetachedAsks)}
		case err != nil:
			return ChatAskOut{}, &transport.Error{Kind: transport.KindUnsupported, RPC: "chat.ask", Message: "the job table is closed: " + err.Error()}
		}
		return ChatAskOut{JobID: id, Status: "running"}, nil
	}

	return s.runAsk(ctx, nb, in.Question, opts, askProgress(progressFrom(ctx)))
}

// progressTail is how many trailing characters of the partial answer a
// progress notification carries.
const progressTail = 200

// askProgress turns streamed answer snapshots into progress notifications.
// The message is the answer's last progressTail characters ("…" marks a
// cut), not the whole answer, so a long answer is not resent every second.
// progress is the answer length in characters, bumped when needed so it
// increases strictly (MCP spec); total stays unknown.
func askProgress(prog Progress) func(string) {
	last := 0.0
	return func(text string) {
		n := utf8.RuneCountInString(text)
		msg := text
		if n > progressTail {
			cut := len(text)
			for range progressTail {
				_, size := utf8.DecodeLastRuneInString(text[:cut])
				cut -= size
			}
			msg = "…" + text[cut:]
		}
		v := float64(n)
		if v <= last {
			v = last + 1
		}
		last = v
		prog(msg, v, 0)
	}
}

func (s *server) runAsk(ctx context.Context, nb, q string, o nlm.AskOptions, progress func(string)) (ChatAskOut, error) {
	a, err := s.NLM.Ask(ctx, nb, q, o, progress)
	if err != nil {
		return ChatAskOut{}, err
	}
	out := ChatAskOut{Answer: a.Text, ConversationID: a.ConversationID, ResponseID: a.ResponseID}
	titles := map[string]string{}
	if len(a.Citations) > 0 {
		// Best effort: a failed lookup leaves titles empty.
		if srcs, err := s.NLM.ListSources(ctx, nb); err == nil {
			for _, x := range srcs {
				titles[x.ID] = x.Title
			}
		}
	}
	for _, c := range a.Citations {
		out.Citations = append(out.Citations, CitationOut{N: c.N, SourceID: c.SourceID, SourceTitle: titles[c.SourceID], Text: c.Text, Start: c.Start, End: c.End})
	}
	return out, nil
}

func (s *server) chatHistory(ctx context.Context, in ChatHistoryIn) (ChatHistoryOut, error) {
	switch in.Format {
	case "", "json", "markdown":
	default:
		return ChatHistoryOut{}, badInput("format must be json or markdown, got %q", in.Format)
	}
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return ChatHistoryOut{}, err
	}
	conv := in.ConversationID
	if conv == "" {
		threads, err := s.NLM.ChatSessions(ctx, nb)
		if err != nil {
			return ChatHistoryOut{}, err
		}
		if len(threads) == 0 {
			return ChatHistoryOut{}, nil
		}
		conv = threads[0].ID
	}
	turns, next, err := s.NLM.History(ctx, nb, conv, in.Limit, in.Cursor)
	if err != nil {
		return ChatHistoryOut{}, err
	}
	out := ChatHistoryOut{ConversationID: conv, Turns: turns, NextCursor: next}
	if in.Format == "markdown" {
		var b strings.Builder
		for _, t := range turns {
			who := "Assistant"
			if t.Role == "user" {
				who = "You"
			}
			fmt.Fprintf(&b, "**%s**\n\n%s\n\n", who, t.Text)
		}
		out.Markdown = strings.TrimRight(b.String(), "\n")
	}
	return out, nil
}

// chatConfigureSchema is ChatConfigureIn's schema with enums of the proven
// settings only. Input is validated against the plain type schema, so
// "default" reaches the handler and gets UNSUPPORTED_BY_TRANSPORT.
func chatConfigureSchema() *jsonschema.Schema {
	s, _ := schemaFor[ChatConfigureIn]()
	for _, v := range nlm.ChatGoals {
		s.Properties["mode"].Enum = append(s.Properties["mode"].Enum, v)
	}
	for _, v := range nlm.ChatLengths {
		s.Properties["response_length"].Enum = append(s.Properties["response_length"].Enum, v)
	}
	return s
}

func (s *server) chatConfigure(ctx context.Context, in ChatConfigureIn) (ChatConfigureOut, error) {
	cfg := nlm.ChatConfig{Prompt: in.CustomPrompt, Length: in.ResponseLength}
	switch in.Mode {
	case "":
		if in.CustomPrompt != "" {
			cfg.Goal = "custom"
		}
	case "default", "learning_guide", "custom": // default: refused by the domain layer as unproven
		cfg.Goal = in.Mode
	default:
		return ChatConfigureOut{}, badInput("mode must be learning_guide or custom, got %q", in.Mode)
	}
	if cfg.Goal == "" && cfg.Length == "" {
		return ChatConfigureOut{}, badInput("give mode, custom_prompt or response_length")
	}
	nb, err := s.resolveNotebook(ctx, in.Notebook)
	if err != nil {
		return ChatConfigureOut{}, err
	}
	if err := s.NLM.Configure(ctx, nb, cfg); err != nil {
		return ChatConfigureOut{}, err
	}
	s.Cache.Invalidate(s.Profile + "/notebooks")
	s.Cache.Invalidate(s.Profile + "/notebook/" + nb)
	return ChatConfigureOut{Notebook: nb, Mode: cfg.Goal, Length: cfg.Length, Updated: true}, nil
}

func (s *server) suggest(ctx context.Context, in SuggestIn) (SuggestOut, error) {
	switch in.Surface {
	case "chat", "followups", "report", "studio_options":
	default:
		return SuggestOut{}, badInput("surface must be chat, followups, report or studio_options, got %q", in.Surface)
	}
	var nb string
	var srcIDs []string
	if in.Surface != "studio_options" {
		if in.Notebook == "" {
			return SuggestOut{}, badInput("notebook is required for surface %s", in.Surface)
		}
		var err error
		if nb, err = s.resolveNotebook(ctx, in.Notebook); err != nil {
			return SuggestOut{}, err
		}
		if srcIDs, err = s.resolveSources(ctx, nb, in.Sources); err != nil {
			return SuggestOut{}, err
		}
	}
	res, err := s.NLM.Suggest(ctx, nb, in.Surface, srcIDs, in.Query)
	if err != nil {
		return SuggestOut{}, err
	}
	if res == nil {
		res = []nlm.Suggestion{}
	}
	return SuggestOut{Suggestions: res}, nil
}
