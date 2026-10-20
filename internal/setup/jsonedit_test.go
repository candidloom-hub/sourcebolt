package setup

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

var testEntry = Entry{Command: `C:\Programs\sourcebolt.exe`, Args: []string{"serve"}}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("result is not JSON: %v\n%s", err, b)
	}
	return m
}

func wantEntry(t *testing.T, m map[string]any, key string) {
	t.Helper()
	servers, _ := m[key].(map[string]any)
	got, _ := servers["sourcebolt"].(map[string]any)
	if got["command"] != testEntry.Command || !reflect.DeepEqual(got["args"], []any{"serve"}) {
		t.Fatalf("sourcebolt entry = %v", got)
	}
}

// Other servers, unknown keys and their order and spacing survive the merge.
func TestMergeKeepsOtherServersAndUnknownKeys(t *testing.T) {
	in := `{
    "theme": "dark",
    "mcpServers": {
        "filesystem": {"command": "npx", "args": ["-y", "fs"], "env": {"K": "v"}},
        "other": {"url": "https://x.example/mcp"}
    },
    "zz_unknown": [1, 2, {"deep": true}]
}
`
	out, existed, err := Merge([]byte(in), "mcpServers", "sourcebolt", testEntry)
	if err != nil || existed {
		t.Fatalf("existed=%v err=%v", existed, err)
	}
	m := decode(t, out)
	wantEntry(t, m, "mcpServers")
	if m["theme"] != "dark" || !reflect.DeepEqual(m["zz_unknown"], []any{1.0, 2.0, map[string]any{"deep": true}}) {
		t.Errorf("unknown keys changed: %v", m)
	}
	servers := m["mcpServers"].(map[string]any)
	if !reflect.DeepEqual(servers["filesystem"], map[string]any{"command": "npx", "args": []any{"-y", "fs"}, "env": map[string]any{"K": "v"}}) {
		t.Errorf("filesystem server changed: %v", servers["filesystem"])
	}
	if !reflect.DeepEqual(servers["other"], map[string]any{"url": "https://x.example/mcp"}) {
		t.Errorf("other server changed: %v", servers["other"])
	}
	// Untouched text is kept byte for byte, including its 4-space indent.
	head := in[:strings.Index(in, `"other": {"url": "https://x.example/mcp"}`)+len(`"other": {"url": "https://x.example/mcp"}`)]
	if !strings.HasPrefix(string(out), head) {
		t.Errorf("text before the insertion changed:\n%s", out)
	}
	if !strings.HasSuffix(string(out), "\n    },\n    \"zz_unknown\": [1, 2, {\"deep\": true}]\n}\n") {
		t.Errorf("text after the insertion changed:\n%s", out)
	}
	if !strings.Contains(string(out), "\n        \"sourcebolt\": {\n            \"command\"") {
		t.Errorf("new entry does not follow the file's indent:\n%s", out)
	}
}

func TestMergeEmptyOrMissingFile(t *testing.T) {
	for _, in := range []string{"", "  \n", "{}", "{ }"} {
		out, existed, err := Merge([]byte(in), "mcpServers", "sourcebolt", testEntry)
		if err != nil || existed {
			t.Fatalf("%q: existed=%v err=%v", in, existed, err)
		}
		wantEntry(t, decode(t, out), "mcpServers")
	}
}

// A one-line file gets the common two-space indent.
func TestMergeDefaultIndent(t *testing.T) {
	out, _, err := Merge([]byte(`{}`), "mcpServers", "sourcebolt", testEntry)
	if err != nil || !strings.Contains(string(out), "{\n  \"mcpServers\": {\n    \"sourcebolt\": {\n      \"command\"") {
		t.Fatalf("err=%v out=\n%s", err, out)
	}
}

func TestMergeAddsMissingServersKey(t *testing.T) {
	in := `{"numStartups": 3, "projects": {"/p": {"mcpServers": {"x": {}}}}}`
	out, _, err := Merge([]byte(in), "mcpServers", "sourcebolt", testEntry)
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, out)
	wantEntry(t, m, "mcpServers")
	if m["numStartups"] != 3.0 {
		t.Errorf("numStartups lost: %v", m)
	}
	// The nested project-scoped mcpServers is not the target.
	p := m["projects"].(map[string]any)["/p"].(map[string]any)["mcpServers"].(map[string]any)
	if _, ok := p["sourcebolt"]; ok || len(p) != 1 {
		t.Errorf("project servers changed: %v", p)
	}
}

func TestMergeReplacesExistingEntry(t *testing.T) {
	in := `{"servers": {"a": {"command": "a"}, "sourcebolt": {"command": "old", "args": ["x"]}, "b": {"command": "b"}}}`
	ventry := testEntry
	ventry.Type = "stdio"
	out, existed, err := Merge([]byte(in), "servers", "sourcebolt", ventry)
	if err != nil || !existed {
		t.Fatalf("existed=%v err=%v", existed, err)
	}
	m := decode(t, out)
	wantEntry(t, m, "servers")
	s := m["servers"].(map[string]any)
	if len(s) != 3 || s["sourcebolt"].(map[string]any)["type"] != "stdio" {
		t.Errorf("servers = %v", s)
	}
}

func TestHasEntry(t *testing.T) {
	in := []byte(`{"mcpServers": {"sourcebolt": {"command": "c", "args": ["serve"]}}}`)
	got, ok, err := Lookup(in, "mcpServers", "sourcebolt")
	if err != nil || !ok || got.Command != "c" {
		t.Fatalf("got=%+v ok=%v err=%v", got, ok, err)
	}
	if _, ok, _ := Lookup([]byte(`{"mcpServers": {}}`), "mcpServers", "sourcebolt"); ok {
		t.Fatal("found an entry that is not there")
	}
	if _, _, err := Lookup([]byte(`{"mcpServers": {}} trailing`), "mcpServers", "sourcebolt"); err == nil {
		t.Fatal("text after the object must be an error")
	}
	if _, ok, _ := Lookup(nil, "mcpServers", "sourcebolt"); ok {
		t.Fatal("found an entry in an empty file")
	}
}

func TestMergeRefusesNonJSON(t *testing.T) {
	for _, in := range []string{
		"{\n  // a comment\n  \"servers\": {}\n}",
		`[1, 2]`,
		`{"mcpServers": [1]}`,
		`{"a": 1,}`,
		`{"a": 1} trailing`,
	} {
		if _, _, err := Merge([]byte(in), "mcpServers", "sourcebolt", testEntry); err == nil {
			t.Errorf("%q: want an error, the file must not be rewritten", in)
		}
	}
}

func TestRemove(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`{"s": {"a": 1, "sourcebolt": {"command": "x"}, "b": 2}}`, `{"s": {"a": 1, "b": 2}}`},
		{`{"s": {"sourcebolt": {"command": "x"}, "b": 2}}`, `{"s": {"b": 2}}`},
		{`{"s": {"a": 1, "sourcebolt": {"command": "x"}}}`, `{"s": {"a": 1}}`},
		{`{"s": {"sourcebolt": {"command": "x"}}}`, `{"s": {}}`},
		{`{"s": {"a": 1}}`, `{"s": {"a": 1}}`},
	} {
		out, err := Remove([]byte(tc.in), "s", "sourcebolt")
		if err != nil {
			t.Fatalf("%s: %v", tc.in, err)
		}
		if string(out) != tc.want {
			t.Errorf("Remove(%s) = %s, want %s", tc.in, out, tc.want)
		}
	}
}

func TestMergeKeepsBOM(t *testing.T) {
	in := "\ufeff{\"mcpServers\": {}}"
	out, _, err := Merge([]byte(in), "mcpServers", "sourcebolt", testEntry)
	if err != nil || !strings.HasPrefix(string(out), "\ufeff{") {
		t.Fatalf("err=%v out=%q", err, out)
	}
	wantEntry(t, decode(t, out[3:]), "mcpServers")
}
