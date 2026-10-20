package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	beginMarker = "<!-- BEGIN GENERATED: go run ./tools/installlinks -->\n"
	endMarker   = "<!-- END GENERATED -->"
)

func TestDocIsCurrent(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "install-links.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.ReplaceAll(string(raw), "\r\n", "\n")
	i, j := strings.Index(doc, beginMarker), strings.Index(doc, endMarker)
	if i < 0 || j < i {
		t.Fatal("generated markers missing in docs/install-links.md")
	}
	var b bytes.Buffer
	generate(&b)
	if got := doc[i+len(beginMarker) : j]; got != "\n"+b.String() {
		t.Fatal("docs/install-links.md is stale; paste the output of go run ./tools/installlinks between the markers")
	}
}

// JavaScript reference values: encodeURIComponent('{"a":"b c/é!~*\'()"}').
func TestEncodeURIComponentMatchesJavaScript(t *testing.T) {
	in := `{"a":"b c/é!~*'()"}`
	want := `%7B%22a%22%3A%22b%20c%2F%C3%A9!~*'()%22%7D`
	if got := encodeURIComponent(in); got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

// Cursor's documented example decodes to the server config without a name;
// our encoder must produce the same bytes for the same config.
func TestCursorMatchesDocumentedExample(t *testing.T) {
	ex := "eyJjb21tYW5kIjoibnB4IiwiYXJncyI6WyIteSIsIkBtb2RlbGNvbnRleHRwcm90b2NvbC9zZXJ2ZXItcG9zdGdyZXMiLCJwb3N0Z3Jlc3FsOi8vbG9jYWxob3N0L215ZGIiXX0="
	got := base64.StdEncoding.EncodeToString([]byte(jsonStringify(Server{"npx", []string{"-y", "@modelcontextprotocol/server-postgres", "postgresql://localhost/mydb"}})))
	if got != ex {
		t.Fatalf("got %s\nwant %s", got, ex)
	}
}

func TestLinksDecodeToConfig(t *testing.T) {
	for _, v := range Variants {
		for _, scheme := range []string{"vscode", "vscode-insiders"} {
			u := VSCodeURL(v, scheme)
			prefix := scheme + ":mcp/install?"
			if !strings.HasPrefix(u, prefix) {
				t.Fatalf("%s: bad prefix %s", v.Key, u)
			}
			js, err := url.QueryUnescape(strings.TrimPrefix(u, prefix))
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				Name, Type, Command string
				Args                []string
			}
			if err := json.Unmarshal([]byte(js), &got); err != nil {
				t.Fatalf("%s: %v", v.Key, err)
			}
			if got.Name != "sourcebolt" || got.Type != "stdio" || got.Command != v.Server.Command || !reflect.DeepEqual(got.Args, v.Server.Args) {
				t.Fatalf("%s: decoded %+v", v.Key, got)
			}
		}

		cu, err := url.Parse(CursorURL(v))
		if err != nil {
			t.Fatal(err)
		}
		if cu.Scheme != "cursor" || cu.Host != "anysphere.cursor-deeplink" || cu.Path != "/mcp/install" || cu.Query().Get("name") != "sourcebolt" {
			t.Fatalf("%s: bad cursor URL %s", v.Key, cu)
		}
		// Read config from the raw query: base64 '+' must not turn into a space.
		raw := cu.RawQuery[strings.Index(cu.RawQuery, "config=")+len("config="):]
		dec, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			t.Fatalf("%s: %v", v.Key, err)
		}
		var s Server
		if err := json.Unmarshal(dec, &s); err != nil || !reflect.DeepEqual(s, v.Server) {
			t.Fatalf("%s: cursor config %s (%v)", v.Key, dec, err)
		}

		bash := AddMCPBash(v)
		js := strings.TrimSuffix(strings.TrimPrefix(bash, "code --add-mcp '"), "'")
		if strings.Contains(js, "'") || js != vscodeConfig(v) {
			t.Fatalf("%s: bash line %s", v.Key, bash)
		}
		ps := AddMCPPowerShell(v)
		inner := strings.TrimSuffix(strings.TrimPrefix(ps, `code --% --add-mcp "`), `"`)
		if strings.ReplaceAll(inner, `\"`, `"`) != vscodeConfig(v) {
			t.Fatalf("%s: powershell line %s", v.Key, ps)
		}
	}
}
