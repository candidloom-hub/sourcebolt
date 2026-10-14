package nlm

import (
	"context"
	"fmt"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// blocked reports a kind whose payload is still "conflicting" in the RPC
// catalog and has no live fixture.
func blocked(rpc, what string) error {
	return &transport.Error{Kind: transport.KindUnsupported, RPC: rpc,
		Message: "not implemented until live capture pins " + what}
}

// shape: catalog consensus, not live-verified (WWINqb): [[nb],[2]].
// Batch shapes are rejected by the server, so one call per notebook.
func deleteNotebookParams(id string) []any { return []any{[]any{id}, []any{2}} }

// shape: catalog consensus, not live-verified (XgrPMd): [null, conversation_id].
func cancelChatParams(conv string) []any { return []any{nil, conv} }

// Delete removes items of one kind. It is a plain domain call: preview and
// confirmation belong to the MCP layer. Only "notebook" is implemented (ids
// are notebook ids; nb is ignored). "source" (tGMBJ), "artifact" (V5N4be),
// "chat_turn" (J7Gthc) and "note" (AH0mwd) have conflicting catalog shapes
// and return KindUnsupported until a live capture pins them. Calls are never
// retried.
func (c *Client) Delete(ctx context.Context, kind string, nb string, ids []string) error {
	switch kind {
	case "notebook":
	case "source":
		return blocked("sources.delete", "tGMBJ (four conflicting shapes)")
	case "artifact":
		return blocked("studio.delete", "V5N4be (shape conflict)")
	case "chat_turn":
		return blocked("chat.delete_turns", "J7Gthc (shape conflict)")
	case "note":
		return blocked("notes.delete", "AH0mwd (trailing [2] conflict)")
	default:
		return &transport.Error{Kind: transport.KindInvalidArgument, RPC: "delete", Message: fmt.Sprintf("unknown kind %q", kind)}
	}
	if len(ids) == 0 {
		return &transport.Error{Kind: transport.KindInvalidArgument, RPC: "notebooks.delete", Message: "no ids given"}
	}
	for _, id := range ids {
		if id == "" {
			return &transport.Error{Kind: transport.KindInvalidArgument, RPC: "notebooks.delete", Message: "empty id"}
		}
	}
	for _, id := range ids {
		if _, err := c.do(ctx, "notebooks.delete", deleteNotebookParams(id), callOpts{null: wire.NullAllowed}); err != nil {
			return err
		}
	}
	return nil
}

// CancelChat stops a running chat generation (XgrPMd).
func (c *Client) CancelChat(ctx context.Context, conv string) error {
	if conv == "" {
		return &transport.Error{Kind: transport.KindInvalidArgument, RPC: "chat.cancel", Message: "no conversation id given"}
	}
	_, err := c.do(ctx, "chat.cancel", cancelChatParams(conv), callOpts{idempotent: true, null: wire.NullAllowed})
	return err
}
