package main

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"io"

	"github.com/candidloom-hub/sourcebolt/internal/app"
	"github.com/candidloom-hub/sourcebolt/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// cmdCall runs one tool in-process and prints its result: every MCP tool
// is scriptable without a second command tree.
func cmdCall(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintln(stderr, "usage: sourcebolt call <tool> '<json args>'")
		return 2
	}
	name, rawArgs := args[0], "{}"
	if len(args) == 2 {
		rawArgs = args[1]
	}
	var argv map[string]any
	if err := json.Unmarshal([]byte(rawArgs), &argv); err != nil {
		fmt.Fprintln(stderr, "arguments must be a JSON object:", err)
		return 2
	}
	cfg, err := app.ConfigFromEnv()
	if err != nil {
		return fail(stderr, err)
	}
	rt, err := app.Build(cfg)
	if err != nil {
		return fail(stderr, err)
	}
	srv := mcpserver.New(rt.Deps, "all")
	ct, st := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		return fail(stderr, err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "sourcebolt-call", Version: "v1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		return fail(stderr, err)
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		return fail(stderr, err)
	}
	known := false
	for _, t := range tools.Tools {
		known = known || t.Name == name
	}
	if !known {
		fmt.Fprintf(stderr, "unknown tool %q\n", name)
		return 2
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: argv})
	if err != nil {
		return fail(stderr, err)
	}
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			fmt.Fprintln(stdout, tc.Text)
		}
	}
	if res.IsError {
		return 1
	}
	return 0
}
