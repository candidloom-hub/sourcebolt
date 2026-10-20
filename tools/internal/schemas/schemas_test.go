package schemas

import "testing"

// Guard against a validator that accepts everything: known-bad documents
// must fail, so a passing check on our files means something.
func TestValidatorRejectsBadDocuments(t *testing.T) {
	bad := map[string]string{
		MCPBManifest: `{"manifest_version":"0.3","name":"x","version":"1","description":"d","author":{"name":"a"},
			"server":{"type":"go","entry_point":"x","mcp_config":{"command":"x"}}}`,
		ServerJSON: `{"name":"no-slash","description":"d","version":"1.0.0"}`,
	}
	for schema, doc := range bad {
		if err := Validate(schema, []byte(doc)); err == nil {
			t.Errorf("%s accepted a bad document", schema)
		}
	}
	good := map[string]string{
		MCPBManifest: `{"manifest_version":"0.3","name":"x","version":"1","description":"d","author":{"name":"a"},
			"server":{"type":"binary","entry_point":"x","mcp_config":{"command":"x"}}}`,
		ServerJSON: `{"name":"io.github.a/b","description":"d","version":"1.0.0"}`,
	}
	for schema, doc := range good {
		if err := Validate(schema, []byte(doc)); err != nil {
			t.Errorf("%s rejected a good document: %v", schema, err)
		}
	}
}
