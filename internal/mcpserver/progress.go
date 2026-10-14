package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type reqKey struct{}

// withRequest stores the incoming tool request so handlers can send
// progress notifications without seeing SDK types in their signatures.
func withRequest(ctx context.Context, req *mcp.CallToolRequest) context.Context {
	return context.WithValue(ctx, reqKey{}, req)
}

// Progress reports work done so far. It is a no-op when the client sent no
// progress token. progress must increase on every call (MCP spec).
type Progress func(message string, progress, total float64)

// progressFrom returns a Progress for the request in ctx.
func progressFrom(ctx context.Context) Progress {
	req, _ := ctx.Value(reqKey{}).(*mcp.CallToolRequest)
	if req == nil || req.Params == nil || req.Session == nil {
		return func(string, float64, float64) {}
	}
	token := req.Params.GetProgressToken()
	if token == nil {
		return func(string, float64, float64) {}
	}
	return func(message string, progress, total float64) {
		_ = req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
			ProgressToken: token, Message: message, Progress: progress, Total: total,
		})
	}
}
