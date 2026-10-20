package setup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeEnv builds an Env whose filesystem is a set of existing paths and whose
// PATH holds the named programs. Nothing on the real machine is read.
func fakeEnv(goos string, existing []string, onPath ...string) Env {
	home, appdata := "/home/u", ""
	switch goos {
	case "windows":
		home, appdata = `C:\Users\u`, `C:\Users\u\AppData\Roaming`
	case "darwin":
		home = "/Users/u"
	}
	set := map[string]bool{}
	for _, p := range existing {
		set[p] = true
	}
	return Env{
		GOOS: goos, Home: home, AppData: appdata,
		Exists: func(p string) bool { return set[p] },
		LookPath: func(name string) (string, error) {
			for _, n := range onPath {
				if n == name {
					return filepath.Join("/bin", name), nil
				}
			}
			return "", errors.New("not found")
		},
	}
}

func byID(t *testing.T, cs []Client, id string) Client {
	t.Helper()
	for _, c := range cs {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no client %q", id)
	return Client{}
}

func TestClientPathsPerOS(t *testing.T) {
	j := filepath.Join
	tests := []struct {
		goos, id, want, key string
	}{
		{"windows", "claude-desktop", j(`C:\Users\u\AppData\Roaming`, "Claude", "claude_desktop_config.json"), "mcpServers"},
		{"darwin", "claude-desktop", j("/Users/u", "Library", "Application Support", "Claude", "claude_desktop_config.json"), "mcpServers"},
		{"linux", "claude-desktop", j("/home/u", ".config", "Claude", "claude_desktop_config.json"), "mcpServers"},
		{"windows", "claude-code", j(`C:\Users\u`, ".claude.json"), "mcpServers"},
		{"darwin", "claude-code", j("/Users/u", ".claude.json"), "mcpServers"},
		{"windows", "cursor", j(`C:\Users\u`, ".cursor", "mcp.json"), "mcpServers"},
		{"linux", "cursor", j("/home/u", ".cursor", "mcp.json"), "mcpServers"},
		{"windows", "vscode", j(`C:\Users\u\AppData\Roaming`, "Code", "User", "mcp.json"), "servers"},
		{"darwin", "vscode", j("/Users/u", "Library", "Application Support", "Code", "User", "mcp.json"), "servers"},
		{"linux", "vscode", j("/home/u", ".config", "Code", "User", "mcp.json"), "servers"},
		{"windows", "windsurf", j(`C:\Users\u`, ".codeium", "windsurf", "mcp_config.json"), "mcpServers"},
		{"darwin", "windsurf", j("/Users/u", ".codeium", "windsurf", "mcp_config.json"), "mcpServers"},
		{"windows", "gemini", j(`C:\Users\u`, ".gemini", "settings.json"), "mcpServers"},
		{"darwin", "gemini", j("/Users/u", ".gemini", "settings.json"), "mcpServers"},
	}
	for _, tc := range tests {
		c := byID(t, Clients(fakeEnv(tc.goos, nil)), tc.id)
		if c.Path != tc.want || c.Key != tc.key {
			t.Errorf("%s %s: path=%q key=%q, want %q %q", tc.goos, tc.id, c.Path, c.Key, tc.want, tc.key)
		}
		if c.Installed {
			t.Errorf("%s %s: installed on an empty machine", tc.goos, tc.id)
		}
		if !strings.HasPrefix(c.Docs, "https://") {
			t.Errorf("%s %s: no docs url", tc.goos, tc.id)
		}
	}
}

// Windsurf moved under Devin: the new config dir wins when it exists.
func TestWindsurfDevinPath(t *testing.T) {
	j := filepath.Join
	win := fakeEnv("windows", []string{j(`C:\Users\u\AppData\Roaming`, "devin")})
	if c := byID(t, Clients(win), "windsurf"); c.Path != j(`C:\Users\u\AppData\Roaming`, "devin", "mcp_config.json") || !c.Installed {
		t.Errorf("windows: %+v", c)
	}
	mac := fakeEnv("darwin", []string{j("/Users/u", ".config", "devin")})
	if c := byID(t, Clients(mac), "windsurf"); c.Path != j("/Users/u", ".config", "devin", "mcp_config.json") || !c.Installed {
		t.Errorf("darwin: %+v", c)
	}
	xdg := fakeEnv("linux", []string{j("/x", "devin")})
	xdg.XDGConfig = "/x"
	if c := byID(t, Clients(xdg), "windsurf"); c.Path != j("/x", "devin", "mcp_config.json") {
		t.Errorf("xdg: %+v", c)
	}
}

func TestClientDetection(t *testing.T) {
	j := filepath.Join
	env := fakeEnv("windows", []string{
		j(`C:\Users\u\AppData\Roaming`, "Claude"),
		j(`C:\Users\u`, ".cursor"),
		j(`C:\Users\u`, ".gemini"),
	}, "claude", "code")
	want := map[string]bool{"claude-desktop": true, "claude-code": true, "cursor": true, "vscode": true, "windsurf": false, "gemini": true}
	for _, c := range Clients(env) {
		if c.Installed != want[c.ID] {
			t.Errorf("%s installed=%v, want %v", c.ID, c.Installed, want[c.ID])
		}
	}
	if c := byID(t, Clients(env), "claude-code"); c.CLI != j("/bin", "claude") {
		t.Errorf("claude CLI = %q", c.CLI)
	}
	if c := byID(t, Clients(env), "vscode"); c.CLI != j("/bin", "code") {
		t.Errorf("code CLI = %q", c.CLI)
	}
	// Without the CLI, an existing config file or app dir still counts.
	env2 := fakeEnv("darwin", []string{j("/Users/u", ".claude.json"), j("/Users/u", "Library", "Application Support", "Code", "User")})
	if c := byID(t, Clients(env2), "claude-code"); !c.Installed || c.CLI != "" {
		t.Errorf("claude-code: %+v", c)
	}
	if c := byID(t, Clients(env2), "vscode"); !c.Installed || c.CLI != "" {
		t.Errorf("vscode: %+v", c)
	}
}

func TestEntryShape(t *testing.T) {
	cs := Clients(fakeEnv("linux", nil))
	for id, typed := range map[string]bool{"claude-desktop": false, "claude-code": true, "cursor": true, "vscode": true, "windsurf": false, "gemini": false} {
		e := byID(t, cs, id).Entry("/opt/sourcebolt")
		if e.Command != "/opt/sourcebolt" || len(e.Args) != 1 || e.Args[0] != "serve" {
			t.Errorf("%s: %+v", id, e)
		}
		if (e.Type == "stdio") != typed {
			t.Errorf("%s: type=%q, want typed=%v", id, e.Type, typed)
		}
	}
}

func TestBackupAndRestore(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "mcp.json")
	os.WriteFile(p, []byte(`{"a":1}`), 0o600)
	now := time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC)
	b1, err := Backup(p, now)
	if err != nil || !strings.HasPrefix(filepath.Base(b1), "mcp.json.sourcebolt-backup-20261021-093000") {
		t.Fatalf("backup = %q, err = %v", b1, err)
	}
	if got, _ := os.ReadFile(b1); string(got) != `{"a":1}` {
		t.Fatalf("backup content = %q", got)
	}
	os.WriteFile(p, []byte(`{"a":2}`), 0o600)
	b2, _ := Backup(p, now) // same second: still a distinct, later backup
	if b2 == b1 {
		t.Fatal("second backup overwrote the first")
	}
	os.WriteFile(p, []byte(`{"a":3}`), 0o600)

	from, err := Restore(p)
	if err != nil || from != b2 {
		t.Fatalf("restored from %q (want %q), err = %v", from, b2, err)
	}
	if got, _ := os.ReadFile(p); string(got) != `{"a":2}` {
		t.Fatalf("after first undo = %q", got)
	}
	if _, err := Restore(p); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); string(got) != `{"a":1}` {
		t.Fatalf("after second undo = %q", got)
	}
	if _, err := Restore(p); !errors.Is(err, ErrNoBackup) {
		t.Fatalf("third undo: err = %v, want ErrNoBackup", err)
	}
}

// RestoreEntry undoes only the sourcebolt entry, for files the app itself
// keeps rewriting (~/.claude.json): later changes to other keys survive.
func TestRestoreEntry(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".claude.json")
	os.WriteFile(p, []byte(`{"n": 1, "mcpServers": {"sourcebolt": {"command": "old", "args": ["serve"], "env": {"K": "v"}}}}`), 0o600)
	Backup(p, time.Now())
	os.WriteFile(p, []byte(`{"n": 2, "mcpServers": {"sourcebolt": {"command": "new", "args": ["serve"]}, "x": {}}}`), 0o600)
	if _, err := RestoreEntry(p, "mcpServers", "sourcebolt"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	m := decode(t, got)
	s := m["mcpServers"].(map[string]any)
	old := s["sourcebolt"].(map[string]any)
	if m["n"] != 2.0 || s["x"] == nil || old["command"] != "old" || old["env"] == nil {
		t.Fatalf("after undo: %s", got)
	}
	if _, err := LatestBackup(p); !errors.Is(err, ErrNoBackup) {
		t.Fatalf("backup not used up: %v", err)
	}

	// The entry was new: undo removes it and keeps the file.
	Backup(p, time.Now())
	os.WriteFile(p, []byte(`{"n": 3, "mcpServers": {"x": {}, "sourcebolt": {"command": "c"}}}`), 0o600)
	os.WriteFile(p+".sourcebolt-backup-29990101-000000", []byte(`{"n": 0}`), 0o600)
	if _, err := RestoreEntry(p, "mcpServers", "sourcebolt"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); string(got) != `{"n": 3, "mcpServers": {"x": {}}}` {
		t.Fatalf("after undo: %s", got)
	}
}

// A config file that did not exist before setup is removed again by undo.
func TestBackupOfMissingFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	if _, err := Backup(p, time.Now()); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(p, []byte(`{"mcpServers":{}}`), 0o600)
	if _, err := Restore(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("file still exists: %v", err)
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Fatalf("left behind: %v", left)
	}
}
