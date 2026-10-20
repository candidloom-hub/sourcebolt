// Package schemas holds pinned copies of third-party JSON Schemas that our
// distribution files must satisfy, and validates documents against them
// offline. Sources (Model Context Protocol project, Apache-2.0 / MIT; see
// NOTICE):
//
//   - mcpb-manifest-v0.3.schema.json:
//     https://github.com/modelcontextprotocol/mcpb/blob/main/schemas/mcpb-manifest-v0.3.schema.json
//     (identical to mcpb-manifest-latest.schema.json on 2026-10-20)
//   - server.schema.2025-12-11.json:
//     https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json
//
// Update a copy only together with the documents that use it.
package schemas

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

//go:embed *.json
var files embed.FS

const (
	MCPBManifest = "mcpb-manifest-v0.3.schema.json"
	ServerJSON   = "server.schema.2025-12-11.json"
)

// Validate checks doc (raw JSON) against the named embedded schema.
func Validate(schemaFile string, doc []byte) error {
	raw, err := files.ReadFile(schemaFile)
	if err != nil {
		return err
	}
	var s jsonschema.Schema
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("schema %s: %w", schemaFile, err)
	}
	rs, err := s.Resolve(nil)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", schemaFile, err)
	}
	var v any
	if err := json.Unmarshal(doc, &v); err != nil {
		return fmt.Errorf("document is not JSON: %w", err)
	}
	return rs.Validate(v)
}
