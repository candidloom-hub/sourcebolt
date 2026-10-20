package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/cache"
	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// B1/B2: New runs before initialize can be answered, and what it allocates
// sets the idle heap. Schemas are resolved for validation on a tool's first
// call, not for all tools at startup (that was over half of New's time and
// allocations).
func TestNewAllocBudget(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector inflates allocation counts")
	}
	allocs := testing.AllocsPerRun(5, func() {
		New(Deps{NLM: nlm.New(benchTransport{}), Cache: cache.New(nil), Profile: "default", Host: "notebook.google.com"}, "all")
	})
	if allocs > 15000 {
		t.Fatalf("New(all) = %.0f allocs, budget 15000", allocs)
	}
	t.Logf("New(all) = %.0f allocs", allocs)
}

// Every advertised schema must still resolve as the lazy validator will
// resolve it, so a schema bug fails here rather than on a first call.
func TestAdvertisedSchemasResolve(t *testing.T) {
	srv := New(Deps{NLM: nlm.New(benchTransport{}), Cache: cache.New(nil), Profile: "default", Host: "notebook.google.com"}, "all")
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	res, err := cs.ListTools(ctx, nil)
	if err != nil || len(res.Tools) == 0 {
		t.Fatalf("tools = %v, err = %v", res, err)
	}
	for _, tl := range res.Tools {
		for name, raw := range map[string]any{"input": tl.InputSchema, "output": tl.OutputSchema} {
			if raw == nil {
				continue
			}
			var s jsonschema.Schema
			b, _ := json.Marshal(raw)
			if err := s.UnmarshalJSON(b); err != nil {
				t.Fatalf("%s %s schema: %v", tl.Name, name, err)
			}
			if _, err := s.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true}); err != nil {
				t.Fatalf("%s %s schema does not resolve: %v", tl.Name, name, err)
			}
		}
	}
}
