package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// validateOutput re-checks every tool result against its output schema.
// The go-sdk typed path does this on every call by re-parsing the marshalled
// result into a generic value, which was most of B3's cost (about 6200
// allocs for 100 notebooks). Results are typed Go values whose schema is
// derived from the same type, so the check only guards against a schema
// bug: this package's tests turn it on (register_test.go); production
// skips it.
var validateOutput bool

// typedHandler adapts h to the raw go-sdk handler. It does what the go-sdk
// typed path does for input (schema defaults, validation, error text) and
// marshals the output once into StructuredContent plus the JSON text
// fallback.
func typedHandler[In, Out any](t *mcp.Tool, h func(ctx context.Context, req *mcp.CallToolRequest, in In) (Out, error)) (*mcp.Tool, mcp.ToolHandler) {
	tt := *t
	inSchema, inResolved := schemaFor[In]()
	outSchema, outResolved := schemaFor[Out]()
	if tt.InputSchema == nil {
		tt.InputSchema = inSchema
	}
	if tt.OutputSchema == nil {
		tt.OutputSchema = outSchema
	}
	return &tt, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		in, err := decodeArgs[In](req.Params.Arguments, inResolved)
		if err != nil {
			return errorResult(err), nil
		}
		out, err := h(ctx, req, in)
		if err != nil {
			return errorResult(err), nil
		}
		b, err := json.Marshal(out)
		if err != nil {
			return nil, fmt.Errorf("marshaling output: %w", err)
		}
		if validateOutput {
			var v any
			if err := json.Unmarshal(b, &v); err != nil {
				return nil, fmt.Errorf("validating tool output: %w", err)
			}
			if err := outResolved.Validate(&v); err != nil {
				return nil, fmt.Errorf("validating tool output: %w", err)
			}
		}
		return &mcp.CallToolResult{
			StructuredContent: json.RawMessage(b),
			Content:           []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil
	}
}

// schemaFor derives a type's schema exactly as the go-sdk typed path does.
// Tool types are fixed at build time, so a failure is a programming error.
func schemaFor[T any]() (*jsonschema.Schema, *jsonschema.Resolved) {
	rt := reflect.TypeFor[T]()
	if rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	s, err := jsonschema.ForType(rt, &jsonschema.ForOptions{})
	if err != nil {
		panic(fmt.Sprintf("schema for %v: %v", rt, err))
	}
	r, err := s.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		panic(fmt.Sprintf("resolve schema for %v: %v", rt, err))
	}
	return s, r
}

// decodeArgs validates raw arguments (absent means {}), applies schema
// defaults and decodes them into In, with the go-sdk's error wording.
func decodeArgs[In any](raw json.RawMessage, r *jsonschema.Resolved) (In, error) {
	var in In
	m := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &m); err != nil {
			return in, fmt.Errorf("validating \"arguments\": unmarshaling arguments: %w", err)
		}
	}
	var v any = m
	if err := r.ApplyDefaults(&v); err != nil {
		return in, fmt.Errorf("validating \"arguments\": applying schema defaults:\n%w", err)
	}
	if err := r.Validate(&v); err != nil {
		return in, fmt.Errorf("validating \"arguments\": %v", err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return in, err
	}
	if err := json.Unmarshal(b, &in); err != nil {
		return in, err
	}
	return in, nil
}

func errorResult(err error) *mcp.CallToolResult {
	var res mcp.CallToolResult
	res.SetError(err)
	return &res
}
