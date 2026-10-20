package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/bench/benchdata"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Every tool result in this package's tests is checked against its output
// schema; production skips the check (see validateOutput).
func init() { validateOutput = true }

// B3: the go-sdk typed path re-parsed and re-validated every result against
// its own output schema (about 6200 allocs for 100 notebooks). Results are
// marshalled once now; outside tests they are not re-validated.
func TestToolCallAllocBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("alloc budget measured in full runs")
	}
	if raceEnabled {
		t.Skip("the race detector inflates allocation counts")
	}
	prev := validateOutput
	validateOutput = false
	t.Cleanup(func() { validateOutput = prev })
	cs := connectWith(t, benchTransport{payload: benchdata.ListPayload(100)})
	ctx := context.Background()
	p := &mcp.CallToolParams{Name: "notebook_list", Arguments: map[string]any{"refresh": true}}
	allocs := testing.AllocsPerRun(50, func() {
		if res, err := cs.CallTool(ctx, p); err != nil || res.IsError {
			t.Fatalf("call: %v %v", err, res)
		}
	})
	if allocs > 3000 {
		t.Fatalf("notebook_list(100) = %.0f allocs/call, budget 3000", allocs)
	}
}

// The output schema is still advertised, and structured content and the
// text fallback carry the same JSON.
func TestToolResultShapeKept(t *testing.T) {
	cs := connectWith(t, benchTransport{payload: benchdata.ListPayload(3)})
	ctx := context.Background()
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tl := range tools.Tools {
		if tl.Name == "notebook_list" && tl.OutputSchema == nil {
			t.Fatal("notebook_list lost its output schema")
		}
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "notebook_list"})
	if err != nil || res.IsError || res.StructuredContent == nil || len(res.Content) != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if txt := res.Content[0].(*mcp.TextContent).Text; !strings.HasPrefix(txt, "{") || !strings.Contains(txt, "notebooks") {
		t.Fatalf("text fallback = %.80s", txt)
	}
}

// Bad arguments fail the same way the go-sdk typed path failed them.
func TestBadArgumentsRejected(t *testing.T) {
	cs := connectWith(t, benchTransport{payload: benchdata.ListPayload(1)})
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "notebook_list", Arguments: map[string]any{"refresh": "yes"}})
	if err != nil || !res.IsError {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if txt := res.Content[0].(*mcp.TextContent).Text; !strings.Contains(txt, `validating "arguments"`) {
		t.Fatalf("error text = %q", txt)
	}
}

// badOut marshals a string where its schema says integer.
type badOut struct{ N int }

func (badOut) MarshalJSON() ([]byte, error) { return []byte(`{"N":"x"}`), nil }

// Schemas are resolved on a tool's first call; one that cannot be resolved
// must fail that call as a tool error, not panic and end the server.
func TestUnresolvableSchemaIsToolError(t *testing.T) {
	// A schema whose default does not match its type fails to resolve.
	type brokenIn struct {
		N int `json:"n"`
	}
	prev := resolveSchema
	t.Cleanup(func() { resolveSchema = prev })
	resolveSchema = func(s *jsonschema.Schema) (*jsonschema.Resolved, error) {
		if _, ok := s.Properties["n"]; ok {
			s = s.CloneSchemas()
			s.Properties["n"].Default = json.RawMessage(`"not a number"`)
		}
		return prev(s)
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "v0"}, nil)
	srv.AddTool(typedHandler(&mcp.Tool{Name: "broken"}, func(context.Context, *mcp.CallToolRequest, brokenIn) (struct{}, error) {
		return struct{}{}, nil
	}))
	srv.AddTool(typedHandler(&mcp.Tool{Name: "ok"}, func(context.Context, *mcp.CallToolRequest, struct{}) (struct{}, error) {
		return struct{}{}, nil
	}))
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "v0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	for range 2 {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "broken"})
		if err != nil || !res.IsError {
			t.Fatalf("broken schema: res = %+v, err = %v", res, err)
		}
		if txt := res.Content[0].(*mcp.TextContent).Text; !strings.Contains(txt, "schema") {
			t.Fatalf("error text = %q", txt)
		}
	}
	if res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "ok"}); err != nil || res.IsError {
		t.Fatalf("server stopped serving: res = %+v, err = %v", res, err)
	}
}

// Under test, a result that does not match its schema is an error.
func TestOutputValidatedUnderTest(t *testing.T) {
	_, h := typedHandler(&mcp.Tool{Name: "bad"}, func(context.Context, *mcp.CallToolRequest, struct{}) (badOut, error) {
		return badOut{}, nil
	})
	_, err := h(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{}})
	if err == nil || !strings.Contains(err.Error(), "validating tool output") {
		t.Fatalf("err = %v", err)
	}
}
