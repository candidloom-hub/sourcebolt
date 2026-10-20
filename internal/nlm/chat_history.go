package nlm

import (
	"context"
	"slices"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Conversation is one server-side chat thread of a notebook (hPTbtc row).
type Conversation struct {
	ID      string    `json:"id"`
	Created time.Time `json:"created,omitzero"`
	Updated time.Time `json:"updated,omitzero"`
}

// Turn is one message of a conversation. Role is "user" or "assistant".
type Turn struct {
	Role    string    `json:"role"`
	Text    string    `json:"text"`
	Created time.Time `json:"created,omitzero"`
}

// ChatConfig is a partial chat-settings change. Empty fields keep the
// current server value.
//
// Goal: "default", "custom" (needs Prompt) or "learning_guide".
// Length: "default", "longer" or "shorter".
type ChatConfig struct {
	Goal   string `json:"goal,omitempty"`
	Prompt string `json:"prompt,omitempty"`
	Length string `json:"length,omitempty"`
}

const maxChatPrompt = 10000

var (
	chatGoals   = map[string]int{"default": 1, "custom": 2, "learning_guide": 3}
	chatLengths = map[string]int{"default": 1, "longer": 4, "shorter": 5}
)

// Payload shapes pinned by live capture (2026-10-13): hPTbtc [HDR,null,nb,limit],
// khqZz [HDR,null,null,conv,limit].
func chatSessionsParams(nb string, limit int) []any {
	return []any{requestOptions(), nil, nb, limit}
}

// shape: cursor as sixth element is catalog consensus, not live-verified.
func chatTurnsParams(conv string, limit int, cursor string) []any {
	p := []any{requestOptions(), nil, nil, conv, limit}
	if cursor != "" {
		p = append(p, cursor)
	}
	return p
}

// ChatSessionsPage is how many conversations ChatSessions asks for, as the
// web app did (live 2026-10-13). hPTbtc paging is not pinned by a capture,
// so older conversations are not listed.
const ChatSessionsPage = 20

// ChatSessions lists the notebook's newest ChatSessionsPage conversations
// (hPTbtc), newest first.
func (c *Client) ChatSessions(ctx context.Context, nb string) ([]Conversation, error) {
	p, err := c.do(ctx, "chat.sessions", chatSessionsParams(nb, ChatSessionsPage), callOpts{sourcePath: notebookPath(nb), idempotent: true, null: wire.NullAllowed})
	if err != nil {
		return nil, err
	}
	rows := wire.Elems(p, 0)
	out := make([]Conversation, 0, len(rows))
	for _, r := range rows {
		out = append(out, Conversation{ID: wire.Str(r, 0), Created: stamp(r, 1), Updated: stamp(r, 2)})
	}
	return out, nil
}

// History reads one page of a conversation (khqZz). The server sends turns
// newest first; they are returned oldest first. The returned cursor is "" on
// the last page; pass it back to read the next (older) page.
//
// shape: turn layout is catalog consensus, not live-verified (the live
// capture was an empty notebook): t[1] [s,ns], t[2] 1 user / 2 assistant,
// user text t[3], assistant text t[4][0][0]; payload[1] is the next cursor.
func (c *Client) History(ctx context.Context, nb, conv string, limit int, cursor string) ([]Turn, string, error) {
	if limit <= 0 {
		limit = 20
	}
	p, err := c.do(ctx, "chat.turns", chatTurnsParams(conv, limit, cursor), callOpts{sourcePath: notebookPath(nb), idempotent: true, null: wire.NullAllowed})
	if err != nil {
		return nil, "", err
	}
	rows := wire.Elems(p, 0)
	turns := make([]Turn, 0, len(rows))
	for _, t := range rows {
		kind, _ := wire.Int(t, 2)
		switch kind {
		case 1:
			turns = append(turns, Turn{Role: "user", Text: wire.Str(t, 3), Created: stamp(t, 1)})
		case 2:
			turns = append(turns, Turn{Role: "assistant", Text: wire.Str(t, 4, 0, 0), Created: stamp(t, 1)})
		}
	}
	slices.Reverse(turns)
	return turns, wire.Str(p, 1), nil
}

func invalidArg(rpc, msg string) error {
	return &transport.Error{Kind: transport.KindInvalidArgument, RPC: rpc, Message: msg}
}

// Configure changes chat settings (s0tc2d slot 7). It reads the current
// settings from rLM1Ne row [7] first and merges the partial change.
//
// shape: catalog consensus, not live-verified. Slot 7 is
// [[goal(,prompt)],[length]]; custom goal is [2,prompt].
func (c *Client) Configure(ctx context.Context, nb string, cfg ChatConfig) error {
	if cfg.Goal == "" && cfg.Prompt == "" && cfg.Length == "" {
		return invalidArg("notebooks.mutate", "no chat setting to change")
	}
	goal, length := 0, 0
	if cfg.Goal != "" {
		g, ok := chatGoals[cfg.Goal]
		if !ok {
			return invalidArg("notebooks.mutate", "goal must be default, custom or learning_guide")
		}
		goal = g
	}
	if cfg.Length != "" {
		l, ok := chatLengths[cfg.Length]
		if !ok {
			return invalidArg("notebooks.mutate", "length must be default, longer or shorter")
		}
		length = l
	}
	if len([]rune(cfg.Prompt)) > maxChatPrompt {
		return invalidArg("notebooks.mutate", "prompt exceeds 10000 characters")
	}
	if cfg.Prompt != "" && goal != 0 && goal != 2 {
		return invalidArg("notebooks.mutate", "prompt requires goal custom")
	}

	cur, err := c.do(ctx, "notebooks.get", getNotebookParams(nb), callOpts{sourcePath: notebookPath(nb), idempotent: true, null: wire.NullAllowed})
	if err != nil {
		return err
	}
	curGoal, _ := wire.Int(cur, 0, 7, 0, 0)
	curLen, _ := wire.Int(cur, 0, 7, 1, 0)
	curPrompt := wire.Str(cur, 0, 7, 0, 1)
	if curGoal == 0 {
		curGoal = 1
	}
	if curLen == 0 {
		curLen = 1
	}

	g, prompt, l := int(curGoal), curPrompt, int(curLen)
	if goal != 0 {
		g = goal
		if g != 2 {
			prompt = ""
		}
	}
	if cfg.Prompt != "" {
		if g != 2 {
			return invalidArg("notebooks.mutate", "prompt requires goal custom")
		}
		prompt = cfg.Prompt
	}
	if length != 0 {
		l = length
	}
	if g == 2 && prompt == "" {
		return invalidArg("notebooks.mutate", "goal custom needs a prompt")
	}

	goalSlot := []any{g}
	if g == 2 {
		goalSlot = []any{2, prompt}
	}
	params := []any{nb, []any{[]any{nil, nil, nil, nil, nil, nil, nil, []any{goalSlot, []any{l}}}}}
	_, err = c.do(ctx, "notebooks.mutate", params, callOpts{sourcePath: notebookPath(nb), null: wire.NullAllowed})
	return err
}
