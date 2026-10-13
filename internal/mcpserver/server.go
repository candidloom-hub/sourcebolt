// Package mcpserver exposes the NotebookLM domain client as MCP tools.
package mcpserver

import (
	"context"
	"sync/atomic"

	"github.com/candidloom-hub/sourcebolt/internal/cache"
	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Deps are the server's collaborators.
type Deps struct {
	NLM     *nlm.Client
	Cache   *cache.Cache
	Profile string // account profile name
	Host    session.Host
}

type server struct {
	Deps
	toolProfile   string
	lastTransport atomic.Value // string
	tools         []string
}

// New builds the MCP server with the tools enabled by toolProfile
// ("default", "minimal" or "all").
func New(d Deps, toolProfile string) *mcp.Server {
	if toolProfile == "" {
		toolProfile = "default"
	}
	s := &server{Deps: d, toolProfile: toolProfile}
	prev := d.NLM.OnTransport
	d.NLM.OnTransport = func(name string) {
		s.lastTransport.Store(name)
		if prev != nil {
			prev(name)
		}
	}
	m := mcp.NewServer(&mcp.Implementation{Name: version.Name, Version: version.Version}, nil)
	s.registerInfo(m)
	s.registerNotebooks(m)
	return m
}

// register adds a typed tool when the active profile enables it. Handler
// errors become the JSON error envelope; the request ctx flows unchanged
// into every outbound call.
func register[In, Out any](s *server, m *mcp.Server, group string, t *mcp.Tool, h func(ctx context.Context, in In) (Out, error)) {
	if !enabled(s.toolProfile, t.Name, group) {
		return
	}
	s.tools = append(s.tools, t.Name)
	mcp.AddTool(m, t, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		out, err := h(ctx, in)
		if err != nil {
			var zero Out
			return nil, zero, toEnvelope(err)
		}
		return nil, out, nil
	})
}

func readOnly(title string) *mcp.ToolAnnotations {
	f := false
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &f}
}
