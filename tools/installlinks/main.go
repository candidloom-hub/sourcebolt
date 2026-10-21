// Command installlinks prints the generated part of docs/install-links.md:
// one-click install URLs and CLI lines for VS Code and Cursor, for the
// binary on PATH and for the Docker image.
//
//	go run ./tools/installlinks
//
// A test fails when docs/install-links.md is stale, and decodes every URL
// back to its config.
//
// Formats (primary sources, checked 2026-10-20):
//   - VS Code: vscode:mcp/install?{encodeURIComponent(JSON.stringify(obj))}
//     where obj is {name, ...server config}; Insiders uses vscode-insiders:.
//     https://code.visualstudio.com/docs/copilot/guides/mcp-developer-guide
//   - code --add-mcp '{"name":...,"type":"stdio","command":...,"args":[...]}'
//     https://code.visualstudio.com/docs/copilot/reference/mcp-configuration
//   - Cursor: cursor://anysphere.cursor-deeplink/mcp/install?name=$NAME&config=$BASE64
//     where BASE64 is standard base64 of JSON.stringify(server config)
//     without the name. https://cursor.com/docs/context/mcp/install-links
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Image is the pinned Docker image; there is no "latest" tag.
const Image = "ghcr.io/candidloom-hub/sourcebolt:0.3.3"

// Server is an MCP stdio server config in the shape VS Code and Cursor use.
type Server struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// Variant is one way to run sourcebolt.
type Variant struct {
	Key, Title string
	Server     Server
}

var Variants = []Variant{
	{"binary", "Binary on PATH", Server{"sourcebolt", []string{"serve"}}},
	{"docker", "Docker", Server{"docker", []string{"run", "-i", "--rm", "-v", "sourcebolt-config:/config", Image, "serve"}}},
}

// jsonStringify matches JavaScript's JSON.stringify for these values
// (no HTML escaping, no spaces).
func jsonStringify(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		panic(err)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// encodeURIComponent matches JavaScript's: everything except
// A-Z a-z 0-9 - _ . ! ~ * ' ( ) is percent-encoded as UTF-8.
func encodeURIComponent(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			strings.IndexByte("-_.!~*'()", c) >= 0:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// vscodeConfig is the object VS Code takes: name plus the server config.
func vscodeConfig(v Variant) string {
	return jsonStringify(struct {
		Name    string   `json:"name"`
		Type    string   `json:"type"`
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}{"sourcebolt", "stdio", v.Server.Command, v.Server.Args})
}

func VSCodeURL(v Variant, scheme string) string {
	return scheme + ":mcp/install?" + encodeURIComponent(vscodeConfig(v))
}

func CursorURL(v Variant) string {
	cfg := base64.StdEncoding.EncodeToString([]byte(jsonStringify(v.Server)))
	return "cursor://anysphere.cursor-deeplink/mcp/install?name=sourcebolt&config=" + cfg
}

// AddMCPBash is the code --add-mcp line for bash/zsh (single quotes).
func AddMCPBash(v Variant) string { return "code --add-mcp '" + vscodeConfig(v) + "'" }

// AddMCPPowerShell uses the stop-parsing token so the JSON reaches code.cmd
// unchanged in Windows PowerShell 5.1 and PowerShell 7.
func AddMCPPowerShell(v Variant) string {
	return `code --% --add-mcp "` + strings.ReplaceAll(vscodeConfig(v), `"`, `\"`) + `"`
}

func generate(w io.Writer) {
	for _, v := range Variants {
		fmt.Fprintf(w, "### %s\n\n", v.Title)
		fmt.Fprintf(w, "Server config: `%s`\n\n", jsonStringify(v.Server))
		fmt.Fprintf(w, "VS Code install link:\n\n```text\n%s\n```\n\n", VSCodeURL(v, "vscode"))
		fmt.Fprintf(w, "VS Code Insiders install link:\n\n```text\n%s\n```\n\n", VSCodeURL(v, "vscode-insiders"))
		fmt.Fprintf(w, "VS Code from a terminal (bash, zsh):\n\n```bash\n%s\n```\n\n", AddMCPBash(v))
		fmt.Fprintf(w, "VS Code from PowerShell:\n\n```powershell\n%s\n```\n\n", AddMCPPowerShell(v))
		fmt.Fprintf(w, "Cursor install link:\n\n```text\n%s\n```\n\n", CursorURL(v))
	}
}

func main() {
	generate(os.Stdout)
}
