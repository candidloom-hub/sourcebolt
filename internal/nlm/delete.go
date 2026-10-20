package nlm

import (
	"context"
	"errors"
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

// Delete payloads pinned by live capture #2 (2026-10-20), one id per call as
// the web app sent them:
//
//	tGMBJ  source:       [[[sid]], HDR]
//	AH0mwd note:         [nb, null, [note_id], HDR]
//	J7Gthc chat history: [HDR, conversation_id, null, 1]
//
// Each answered [].
func deleteSourceParams(sid string) []any { return []any{[]any{[]any{sid}}, requestOptions()} }

func deleteNoteParams(nb, id string) []any {
	return []any{nb, nil, []any{id}, requestOptions()}
}

func deleteChatHistoryParams(conv string) []any { return []any{requestOptions(), conv, nil, 1} }

// DeleteResult is the outcome of deleting one id.
type DeleteResult struct {
	ID     string
	Status string // deleted | failed | unsupported
	Err    error  // set unless Status is "deleted"
}

// Delete removes items of one kind. It is a plain domain call: preview and
// confirmation belong to the MCP layer. Kinds:
//
//	notebook      ids are notebook ids; nb is ignored (WWINqb)
//	source        source ids of notebook nb (tGMBJ)
//	note          note ids of notebook nb (AH0mwd)
//	chat_history  conversation ids of notebook nb; deletes the whole thread history (J7Gthc)
//
// "artifact" (V5N4be) has conflicting catalog shapes and returns
// KindUnsupported until a live capture pins it.
//
// Duplicate ids are dropped. Every remaining id is tried even after a
// failure, so the result lists one outcome per id; the returned error is only
// for invalid input or a blocked kind (then nothing is sent). Calls are never
// retried.
func (c *Client) Delete(ctx context.Context, kind string, nb string, ids []string) ([]DeleteResult, error) {
	var rpc string
	var params func(id string) []any
	o := callOpts{null: wire.NullAllowed, sourcePath: notebookPath(nb)}
	switch kind {
	case "notebook":
		rpc, params, o.sourcePath = "notebooks.delete", deleteNotebookParams, ""
	case "source":
		rpc, params = "sources.delete", deleteSourceParams
	case "note":
		rpc, params = "notes.delete", func(id string) []any { return deleteNoteParams(nb, id) }
	case "chat_history":
		rpc, params = "chat.delete_turns", deleteChatHistoryParams
	case "artifact":
		return nil, blocked("studio.delete", "V5N4be (shape conflict)")
	default:
		return nil, &transport.Error{Kind: transport.KindInvalidArgument, RPC: "delete", Message: fmt.Sprintf("unknown kind %q", kind)}
	}
	if kind != "notebook" && nb == "" {
		return nil, &transport.Error{Kind: transport.KindInvalidArgument, RPC: rpc, Message: "no notebook id given"}
	}
	if len(ids) == 0 {
		return nil, &transport.Error{Kind: transport.KindInvalidArgument, RPC: rpc, Message: "no ids given"}
	}
	seen := make(map[string]bool, len(ids))
	uniq := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			return nil, &transport.Error{Kind: transport.KindInvalidArgument, RPC: rpc, Message: "empty id"}
		}
		if !seen[id] {
			seen[id] = true
			uniq = append(uniq, id)
		}
	}
	out := make([]DeleteResult, 0, len(uniq))
	for _, id := range uniq {
		if err := ctx.Err(); err != nil {
			out = append(out, DeleteResult{ID: id, Status: "failed", Err: err})
			continue
		}
		r := DeleteResult{ID: id, Status: "deleted"}
		if _, err := c.do(ctx, rpc, params(id), o); err != nil {
			r.Status, r.Err = "failed", err
			var te *transport.Error
			if errors.As(err, &te) && te.Kind == transport.KindUnsupported {
				r.Status = "unsupported"
			}
		}
		out = append(out, r)
	}
	return out, nil
}

// CancelChat stops a running chat generation (XgrPMd).
func (c *Client) CancelChat(ctx context.Context, conv string) error {
	if conv == "" {
		return &transport.Error{Kind: transport.KindInvalidArgument, RPC: "chat.cancel", Message: "no conversation id given"}
	}
	_, err := c.do(ctx, "chat.cancel", cancelChatParams(conv), callOpts{idempotent: true, null: wire.NullAllowed})
	return err
}
