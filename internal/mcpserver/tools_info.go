package mcpserver

import (
	"context"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ServerInfoIn struct {
	IncludeUsage bool `json:"include_usage,omitempty" jsonschema:"also fetch account tier and quota windows (one Google call)"`
}

type ServerInfoOut struct {
	Name          string     `json:"name"`
	Version       string     `json:"version"`
	Profile       string     `json:"profile"`
	Host          string     `json:"host"`
	ToolProfile   string     `json:"tool_profile"`
	Tools         []string   `json:"tools"`
	LastTransport string     `json:"last_transport,omitempty"`
	Usage         *nlm.Quota `json:"usage,omitempty"`
}

func (s *server) registerInfo(m *mcp.Server) {
	register(s, m, "core", &mcp.Tool{
		Name:        "server_info",
		Description: "Show the sourcebolt version, active Google profile, NotebookLM host, enabled tools and which transport served the last call. No Google call unless include_usage is set.",
		Annotations: readOnly("Server info"),
	}, func(ctx context.Context, in ServerInfoIn) (ServerInfoOut, error) {
		last, _ := s.lastTransport.Load().(string)
		out := ServerInfoOut{
			Name: version.Name, Version: version.Version, Profile: s.Profile, Host: string(s.Host),
			ToolProfile: s.toolProfile, Tools: s.tools, LastTransport: last,
		}
		if in.IncludeUsage {
			q, err := s.NLM.Quota(ctx)
			if err != nil {
				return ServerInfoOut{}, err
			}
			out.Usage = &q
		}
		return out, nil
	})
}
