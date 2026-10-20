package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/bench/benchdata"
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
