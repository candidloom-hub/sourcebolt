package nlm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Streamer is implemented by transports that can stream a chat answer
// (rpc.Transport). Ask finds it with a type assertion on the Client's
// transport; a transport without it gets KindUnsupported, so a Router that
// wraps several transports must implement Stream itself to offer chat.
type Streamer interface {
	Stream(ctx context.Context, nb string, params any, onSnap func(wire.Snapshot)) (wire.Snapshot, error)
}

// AskOptions narrows a question. Empty SourceIDs means every source of the
// notebook (sent as explicit ids, never an empty list). Without
// ConversationID the question goes to the notebook's default thread;
// NewConversation starts a fresh one instead.
type AskOptions struct {
	SourceIDs       []string
	ConversationID  string
	NewConversation bool
}

// Answer is the final state of a streamed answer.
type Answer struct {
	Text           string          `json:"text"`
	Citations      []wire.Citation `json:"citations,omitempty"`
	ConversationID string          `json:"conversation_id,omitempty"`
	ResponseID     string          `json:"response_id,omitempty"`
}

// progressEvery is the minimum gap between progress callbacks.
const progressEvery = time.Second

// askNow is the clock for progress throttling (tests replace it).
var askNow = time.Now

// askParams is the 9-element chat request pinned by live capture
// (2026-10-13): [sources, question, null, HDR, conversation_id, null, null,
// notebook_id, seq], sources as a list of [[sid]]. A nil conversation id
// (new thread) is the catalog's 9-element form, not live-verified.
func askParams(sources []string, question, conv, nb string, seq int) []any {
	srcs := make([]any, len(sources))
	for i, s := range sources {
		srcs[i] = []any{[]any{s}}
	}
	var cv any
	if conv != "" {
		cv = conv
	}
	return []any{srcs, question, nil, requestOptions(), cv, nil, nil, nb, seq}
}

// Ask streams one question and returns the final answer. progress (may be
// nil) receives the latest answer text at most once per second; thinking
// chunks are never reported.
//
// The conversation id is never taken from the stream: live captures show
// the stream carries a per-stream session id, and replaying it creates a
// ghost turn. Without an explicit id the notebook's default thread is read
// from hPTbtc (newest first); when there is none yet, or NewConversation is
// set, the question is sent with a null id and the thread is read back
// from hPTbtc afterwards. Null-id asks hold a per-notebook lock from send
// through read-back so concurrent ones cannot read each other's thread.
//
// seq is always 1. Ruling: the live capture (first ask) sent 1; counting
// turns would cost a khqZz round trip per question and its turn layout is
// not live-verified. Revisit if a multi-turn capture shows the server
// rejects or mis-threads seq 1.
func (c *Client) Ask(ctx context.Context, nb, question string, o AskOptions, progress func(text string)) (Answer, error) {
	if strings.TrimSpace(question) == "" {
		return Answer{}, invalidArg("chat.ask", "question is empty")
	}
	st, ok := c.t.(Streamer)
	if !ok {
		return Answer{}, &transport.Error{Kind: transport.KindUnsupported, RPC: "chat.ask", Transport: c.t.Name(), Message: "transport cannot stream chat answers"}
	}
	sources := o.SourceIDs
	if len(sources) == 0 {
		ids, err := c.notebookSourceIDs(ctx, nb)
		if err != nil {
			return Answer{}, err
		}
		if len(ids) == 0 {
			return Answer{}, invalidArg("chat.ask", "notebook has no sources to ask about")
		}
		sources = ids
	}
	conv := o.ConversationID
	if conv == "" && !o.NewConversation {
		threads, err := c.ChatSessions(ctx, nb)
		if err != nil {
			return Answer{}, err
		}
		if len(threads) > 0 {
			conv = threads[0].ID
		}
	}
	if conv == "" {
		// A null id makes the server start a thread whose id is then read
		// back as the newest in hPTbtc. Two such asks on one notebook could
		// read each other's thread, so they run one at a time per notebook,
		// from send through read-back. Asks on other notebooks and asks
		// with an id stay concurrent.
		unlock, err := c.newThread.Lock(ctx, nb)
		if err != nil {
			return Answer{}, err
		}
		defer unlock()
		if !o.NewConversation {
			// The default thread may have been started while this ask waited.
			threads, err := c.ChatSessions(ctx, nb)
			if err != nil {
				return Answer{}, err
			}
			if len(threads) > 0 {
				conv = threads[0].ID
			}
		}
	}

	var last time.Time
	onSnap := func(s wire.Snapshot) {
		if progress == nil || s.Thinking || s.Text == "" {
			return
		}
		now := askNow()
		if !last.IsZero() && now.Sub(last) < progressEvery {
			return
		}
		last = now
		progress(s.Text)
	}
	snap, err := st.Stream(ctx, nb, askParams(sources, question, conv, nb, 1), onSnap)
	if err != nil {
		return Answer{}, err
	}
	if c.OnTransport != nil {
		c.OnTransport(c.t.Name())
	}
	if conv == "" {
		// Best effort: the answer is already in hand, so a failed lookup
		// leaves the id empty instead of failing the question.
		if threads, err := c.ChatSessions(ctx, nb); err == nil && len(threads) > 0 {
			conv = threads[0].ID
		}
	}
	return Answer{Text: snap.Text, Citations: snap.Citations, ConversationID: conv, ResponseID: snap.ResponseID}, nil
}

// notebookSourceIDs reads the notebook's source ids with the rLM1Ne read
// behind GetNotebook (without its summary call, which chat does not need).
func (c *Client) notebookSourceIDs(ctx context.Context, nb string) ([]string, error) {
	p, err := c.do(ctx, "notebooks.get", getNotebookParams(nb), callOpts{sourcePath: notebookPath(nb), idempotent: true, null: wire.NullAllowed})
	if err != nil {
		return nil, err
	}
	root, perr := wire.Parse(p)
	if p != nil && perr != nil {
		return nil, fmt.Errorf("nlm: decode notebook: %w", perr)
	}
	var ids []string
	for _, s := range root.At(0).At(1).Elems() {
		if id := parseSource(s).ID; id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
