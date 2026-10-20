package distcheck

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/tools/internal/schemas"
)

const root = "../.."

func read(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}

func decode(t *testing.T, rel string, v any) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(read(t, rel)))
	if err := dec.Decode(v); err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
}

func TestServerJSONMatchesRegistrySchema(t *testing.T) {
	raw := read(t, "server.json")
	if err := schemas.Validate(schemas.ServerJSON, raw); err != nil {
		t.Fatal(err)
	}
	var s struct {
		Schema   string `json:"$schema"`
		Name     string `json:"name"`
		Version  string `json:"version"`
		Packages []struct {
			RegistryType string `json:"registryType"`
			Identifier   string `json:"identifier"`
		} `json:"packages"`
	}
	decode(t, "server.json", &s)
	if s.Schema != "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json" {
		t.Fatalf("$schema changed to %s; update the pinned copy in tools/internal/schemas too", s.Schema)
	}
	if s.Name != "io.github.candidloom-hub/sourcebolt" {
		t.Fatalf("name %s", s.Name)
	}
	// Only packages that exist may be listed (registry ownership check).
	// PyPI is not published; add it back per docs/registry.md.
	for _, p := range s.Packages {
		switch p.RegistryType {
		case "npm":
			if p.Identifier != "sourcebolt" {
				t.Errorf("npm identifier %s", p.Identifier)
			}
		case "oci":
			if p.Identifier != "ghcr.io/candidloom-hub/sourcebolt:"+s.Version {
				t.Errorf("oci identifier %s does not match version %s", p.Identifier, s.Version)
			}
		default:
			t.Errorf("package type %s is listed but not published; see docs/registry.md", p.RegistryType)
		}
	}
}

// Every package block in docs/registry.md must be valid when added.
func TestRegistryDocBlocksAreValid(t *testing.T) {
	doc := string(read(t, "docs/registry.md"))
	re := regexp.MustCompile("(?s)<!-- registry-package: (\\w+) -->\n```json\n(.*?)\n```")
	blocks := re.FindAllStringSubmatch(doc, -1)
	if len(blocks) != 3 {
		t.Fatalf("found %d package blocks, want 3 (npm, pypi, mcpb)", len(blocks))
	}
	for _, b := range blocks {
		var srv map[string]any
		decode(t, "server.json", &srv)
		var pkg map[string]any
		if err := json.Unmarshal([]byte(b[2]), &pkg); err != nil {
			t.Fatalf("%s block: %v", b[1], err)
		}
		if pkg["registryType"] != b[1] {
			t.Fatalf("block %s has registryType %v", b[1], pkg["registryType"])
		}
		srv["packages"] = append(srv["packages"].([]any), pkg)
		out, _ := json.Marshal(srv)
		if err := schemas.Validate(schemas.ServerJSON, out); err != nil {
			t.Fatalf("server.json plus the %s block: %v", b[1], err)
		}
	}
}

func TestClaudePluginMarketplace(t *testing.T) {
	var mk struct {
		Name    string                `json:"name"`
		Owner   struct{ Name string } `json:"owner"`
		Plugins []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
		} `json:"plugins"`
	}
	decode(t, ".claude-plugin/marketplace.json", &mk)
	if mk.Name == "" || mk.Owner.Name == "" || len(mk.Plugins) != 1 {
		t.Fatalf("marketplace.json: %+v", mk)
	}
	p := mk.Plugins[0]
	if !strings.HasPrefix(p.Source, "./") || strings.Contains(p.Source, "..") {
		t.Fatalf("plugin source %q must be ./ relative without ..", p.Source)
	}
	var man struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	decode(t, p.Source+"/.claude-plugin/plugin.json", &man)
	if man.Name != p.Name {
		t.Fatalf("entry name %s != manifest name %s", p.Name, man.Name)
	}
	var srv struct{ Version string }
	decode(t, "server.json", &srv)
	if man.Version != srv.Version {
		t.Errorf("plugin version %s != server.json version %s", man.Version, srv.Version)
	}
	var mcp struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	decode(t, p.Source+"/.mcp.json", &mcp)
	s, ok := mcp.MCPServers["sourcebolt"]
	if !ok || s.Command != "sourcebolt" || strings.Join(s.Args, " ") != "serve" {
		t.Fatalf("plugin .mcp.json: %+v", mcp)
	}
	// The plugin ships the root SKILL.md unchanged.
	if !bytes.Equal(read(t, p.Source+"/skills/sourcebolt/SKILL.md"), read(t, "SKILL.md")) {
		t.Fatal("plugins/sourcebolt/skills/sourcebolt/SKILL.md differs from SKILL.md; copy it again")
	}
}

func TestGlamaJSON(t *testing.T) {
	// Schema: https://glama.ai/mcp/schemas/server.json (required: maintainers,
	// an array of unique GitHub usernames).
	var g map[string]any
	decode(t, "glama.json", &g)
	if g["$schema"] != "https://glama.ai/mcp/schemas/server.json" {
		t.Fatalf("$schema %v", g["$schema"])
	}
	m, ok := g["maintainers"].([]any)
	if !ok || len(m) == 0 {
		t.Fatal("maintainers missing")
	}
	seen := map[any]bool{}
	for _, x := range m {
		if s, ok := x.(string); !ok || s == "" || seen[x] {
			t.Fatalf("bad maintainer %v", x)
		}
		seen[x] = true
	}
}

func TestGeminiExtension(t *testing.T) {
	var g struct {
		Name       string `json:"name"`
		Version    string `json:"version"`
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
		ContextFileName string `json:"contextFileName"`
	}
	decode(t, "gemini-extension.json", &g)
	if !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(g.Name) {
		t.Fatalf("name %q must be lowercase, digits and dashes", g.Name)
	}
	var srv struct{ Version string }
	decode(t, "server.json", &srv)
	if g.Version != srv.Version {
		t.Errorf("gemini-extension.json version %s != server.json %s", g.Version, srv.Version)
	}
	if s := g.MCPServers["sourcebolt"]; s.Command != "sourcebolt" || strings.Join(s.Args, " ") != "serve" {
		t.Fatalf("mcpServers: %+v", g.MCPServers)
	}
	if g.ContextFileName != "" {
		read(t, g.ContextFileName)
	}
}
