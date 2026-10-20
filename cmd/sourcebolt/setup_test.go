package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/setup"
	"github.com/candidloom-hub/sourcebolt/internal/transport/browser"
	"github.com/zalando/go-keyring"
)

const fakeExe = "/opt/sourcebolt/sourcebolt"

type setupWorld struct {
	home, appData, config string
	onPath                []string
	calls                 [][]string
	doctorRuns            int
}

// newSetupWorld points setup at a temp home and APPDATA, a fake executable
// path, fake app CLIs and a fake doctor. Nothing real is read or written.
func newSetupWorld(t *testing.T, onPath ...string) *setupWorld {
	t.Helper()
	keyring.MockInit()
	root := t.TempDir()
	w := &setupWorld{
		home:    filepath.Join(root, "home"),
		appData: filepath.Join(root, "home", "AppData", "Roaming"),
		config:  filepath.Join(root, "sbconfig"),
		onPath:  onPath,
	}
	os.MkdirAll(w.home, 0o700)
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", w.config)
	t.Setenv("NOTEBOOKLM_HOST", "")
	t.Setenv("NOTEBOOKLM_BROWSER_FALLBACK", "0")
	oldEnv, oldSelf, oldRun, oldDoctor := setupEnv, selfPath, runTool, runDoctor
	setupEnv = func() setup.Env {
		return setup.Env{
			GOOS: runtime.GOOS, Home: w.home, AppData: w.appData,
			XDGConfig:    filepath.Join(w.home, ".config"),
			LocalAppData: filepath.Join(w.home, "AppData", "Local"),
			TempDir:      os.TempDir(),
			Exists: func(p string) bool {
				_, err := os.Stat(p)
				return err == nil
			},
			LookPath: func(name string) (string, error) {
				if slices.Contains(w.onPath, name) {
					return "fake-" + name, nil
				}
				return "", errors.New("not on PATH")
			},
		}
	}
	selfPath = func() (string, error) { return fakeExe, nil }
	runTool = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		w.calls = append(w.calls, append([]string{name}, args...))
		return nil, nil
	}
	runDoctor = func(ctx context.Context, args []string, stdout, stderr io.Writer) int {
		w.doctorRuns++
		io.WriteString(stdout, "ok    fake doctor\n")
		return 0
	}
	t.Cleanup(func() { setupEnv, selfPath, runTool, runDoctor = oldEnv, oldSelf, oldRun, oldDoctor })
	// Fail safe: no path through setup may start a real browser. Tests that
	// exercise sign-in call stubAuto afterwards.
	oldFind, oldStart := findBrowser, startPlain
	findBrowser = func(string) (string, error) { return "", errors.New("tests never launch a browser") }
	startPlain = func(browser.PlainOptions) (plainBrowser, error) {
		t.Error("a test tried to start a real browser")
		return nil, errors.New("tests never launch a browser")
	}
	t.Cleanup(func() { findBrowser, startPlain = oldFind, oldStart })
	return w
}

func (w *setupWorld) path(rel ...string) string {
	return filepath.Join(append([]string{w.home}, rel...)...)
}

func (w *setupWorld) run(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(t.Context(), append([]string{"setup"}, args...), strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func backups(t *testing.T, p string) []string {
	t.Helper()
	m, _ := filepath.Glob(p + ".sourcebolt-backup-*")
	return m
}

// snapshot records every file under dir with its content.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			b, _ := os.ReadFile(p)
			out[p] = string(b)
		}
		return nil
	})
	return out
}

const cursorBefore = `{
  "theme": "keep-me",
  "mcpServers": {
    "filesystem": { "command": "npx", "args": ["-y", "fs"], "env": { "K": "v" } }
  }
}
`

func TestSetupMergesBacksUpAndUndoes(t *testing.T) {
	w := newSetupWorld(t)
	cursor := w.path(".cursor", "mcp.json")
	gemini := w.path(".gemini", "settings.json")
	write(t, cursor, cursorBefore)
	os.MkdirAll(filepath.Dir(gemini), 0o700) // Gemini installed, no settings yet

	code, out, errOut := w.run(t, "", "--yes", "--no-login")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q stdout=%q", code, errOut, out)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(read(t, cursor)), &m); err != nil {
		t.Fatal(err)
	}
	s := m["mcpServers"].(map[string]any)
	if m["theme"] != "keep-me" || s["filesystem"].(map[string]any)["env"] == nil {
		t.Errorf("other entries changed: %v", m)
	}
	sb := s["sourcebolt"].(map[string]any)
	if sb["command"] != fakeExe || sb["args"].([]any)[0] != "serve" || sb["type"] != "stdio" {
		t.Errorf("cursor entry = %v", sb)
	}
	bs := backups(t, cursor)
	if len(bs) != 1 || read(t, bs[0]) != cursorBefore {
		t.Fatalf("cursor backups = %v", bs)
	}
	if !strings.Contains(read(t, gemini), fakeExe) {
		t.Errorf("gemini settings = %q", read(t, gemini))
	}
	for _, want := range []string{"Done. Restart Cursor", "Gemini CLI", "List my NotebookLM notebooks"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	if w.doctorRuns != 1 {
		t.Errorf("doctor ran %d times", w.doctorRuns)
	}

	code, out, errOut = w.run(t, "", "--undo")
	if code != 0 {
		t.Fatalf("undo: code=%d stderr=%q", code, errOut)
	}
	if got := read(t, cursor); got != cursorBefore {
		t.Errorf("undo did not restore cursor:\n%s", got)
	}
	if _, err := os.Stat(gemini); !os.IsNotExist(err) {
		t.Errorf("undo left the settings file setup created: %v", err)
	}
	if len(backups(t, cursor))+len(backups(t, gemini)) != 0 {
		t.Error("undo left backups behind")
	}
	if !strings.Contains(out, "Cursor") {
		t.Errorf("undo output = %q", out)
	}
}

func TestSetupDryRunWritesNothing(t *testing.T) {
	w := newSetupWorld(t, "claude", "code")
	write(t, w.path(".cursor", "mcp.json"), cursorBefore)
	os.MkdirAll(w.path(".gemini"), 0o700)
	write(t, w.path(".claude.json"), `{"numStartups": 1}`)
	before := snapshot(t, filepath.Dir(w.home))

	code, out, errOut := w.run(t, "", "--dry-run")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if after := snapshot(t, filepath.Dir(w.home)); !mapsEqual(before, after) {
		t.Fatalf("dry run changed files:\nbefore %v\nafter  %v", before, after)
	}
	if len(w.calls) != 0 || w.doctorRuns != 0 {
		t.Fatalf("dry run ran tools %v / doctor %d", w.calls, w.doctorRuns)
	}
	for _, want := range []string{"Would", "Cursor", "claude mcp add", fakeExe, "sign in"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run output lacks %q:\n%s", want, out)
		}
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestSetupAsksPerClient(t *testing.T) {
	w := newSetupWorld(t)
	cursor := w.path(".cursor", "mcp.json")
	gemini := w.path(".gemini", "settings.json")
	write(t, cursor, cursorBefore)
	write(t, gemini, `{}`)
	// Order is Claude Desktop, Claude Code, Cursor, VS Code, Windsurf, Gemini:
	// only Cursor and Gemini are installed. No to Cursor, Enter (yes) to Gemini.
	code, out, _ := w.run(t, "n\n\n", "--no-login")
	if code != 0 {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if !strings.Contains(out, "Connect sourcebolt to Cursor? [Y/n]") {
		t.Errorf("no Cursor question:\n%s", out)
	}
	if read(t, cursor) != cursorBefore || len(backups(t, cursor)) != 0 {
		t.Error("declined client was changed")
	}
	if !strings.Contains(read(t, gemini), fakeExe) || len(backups(t, gemini)) != 1 {
		t.Error("accepted client was not connected")
	}
}

func TestSetupAsksBeforeReplacing(t *testing.T) {
	existing := `{"mcpServers": {"sourcebolt": {"command": "old-path", "args": ["serve"]}}}`
	for _, tc := range []struct {
		stdin   string
		replace bool
	}{{"y\nn\n", false}, {"y\ny\n", true}, {"y\n\n", false}} {
		w := newSetupWorld(t)
		cursor := w.path(".cursor", "mcp.json")
		write(t, cursor, existing)
		_, out, _ := w.run(t, tc.stdin, "--no-login")
		if !strings.Contains(out, "Replace it? [y/N]") {
			t.Errorf("%q: no replace question:\n%s", tc.stdin, out)
		}
		got := read(t, cursor)
		if replaced := strings.Contains(got, fakeExe); replaced != tc.replace {
			t.Errorf("%q: replaced=%v, want %v: %s", tc.stdin, replaced, tc.replace, got)
		}
	}
}

func TestSetupAlreadyConnected(t *testing.T) {
	w := newSetupWorld(t)
	cursor := w.path(".cursor", "mcp.json")
	same := `{"mcpServers": {"sourcebolt": {"type": "stdio", "command": "` + fakeExe + `", "args": ["serve"]}}}`
	write(t, cursor, same)
	_, out, _ := w.run(t, "", "--yes", "--no-login")
	if !strings.Contains(out, "already connected") || len(backups(t, cursor)) != 0 || read(t, cursor) != same {
		t.Fatalf("out=%q backups=%v", out, backups(t, cursor))
	}
}

func TestSetupClaudeCodeUsesCLI(t *testing.T) {
	w := newSetupWorld(t, "claude")
	write(t, w.path(".claude.json"), `{"numStartups": 1}`)
	code, out, errOut := w.run(t, "", "--yes", "--no-login", "--client", "claude-code")
	if code != 0 {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
	want := []string{"fake-claude", "mcp", "add", "--scope", "user", "--transport", "stdio", "sourcebolt", "--", fakeExe, "serve"}
	if len(w.calls) != 1 || !slices.Equal(w.calls[0], want) {
		t.Fatalf("calls = %q", w.calls)
	}
	if read(t, w.path(".claude.json")) != `{"numStartups": 1}` {
		t.Error("the CLI path must not edit ~/.claude.json itself")
	}
	if len(backups(t, w.path(".claude.json"))) != 1 {
		t.Error("no backup before the CLI change")
	}
	if !strings.Contains(out, "Restart Claude Code") {
		t.Errorf("out = %q", out)
	}
}

// Run through npx with nothing installed: apps get the npx command, never
// the npx cache path, which npm may delete.
func TestSetupUnderNpxWritesNpxCommand(t *testing.T) {
	w := newSetupWorld(t, "claude", "npx")
	cache := filepath.Join(w.home, ".npm", "_npx", "3f2a", "node_modules", "@candidloom-hub", "sourcebolt-linux-x64", "bin", "sourcebolt")
	selfPath = func() (string, error) { return cache, nil }
	write(t, w.path(".claude.json"), `{}`)
	write(t, w.path(".cursor", "mcp.json"), `{}`)
	code, out, errOut := w.run(t, "", "--yes", "--no-login", "--client", "claude-code", "--client", "cursor")
	if code != 0 {
		t.Fatalf("code=%d err=%q out=%q", code, errOut, out)
	}
	want := []string{"fake-claude", "mcp", "add", "--scope", "user", "--transport", "stdio", "sourcebolt", "--", "npx", "-y", "sourcebolt", "serve"}
	if len(w.calls) != 1 || !slices.Equal(w.calls[0], want) {
		t.Fatalf("claude calls = %q", w.calls)
	}
	var m map[string]any
	json.Unmarshal([]byte(read(t, w.path(".cursor", "mcp.json"))), &m)
	sb := m["mcpServers"].(map[string]any)["sourcebolt"].(map[string]any)
	if sb["command"] != "npx" || !slices.Equal(toStrings(sb["args"]), []string{"-y", "sourcebolt", "serve"}) {
		t.Fatalf("cursor entry = %v", sb)
	}
}

// Run through npx with an installed copy: apps get the installed program.
func TestSetupUnderNpxPrefersInstalledBinary(t *testing.T) {
	w := newSetupWorld(t, "npx")
	installed := filepath.Join(w.home, ".local", "bin", "sourcebolt")
	if runtime.GOOS == "windows" {
		installed = filepath.Join(w.home, "AppData", "Local", "Programs", "sourcebolt", "sourcebolt.exe")
	}
	write(t, installed, "binary")
	selfPath = func() (string, error) {
		return filepath.Join(w.home, ".npm", "_npx", "3f2a", "node_modules", "@candidloom-hub", "sourcebolt-linux-x64", "bin", "sourcebolt"), nil
	}
	write(t, w.path(".cursor", "mcp.json"), `{}`)
	code, out, errOut := w.run(t, "", "--yes", "--no-login", "--client", "cursor")
	if code != 0 {
		t.Fatalf("code=%d err=%q out=%q", code, errOut, out)
	}
	var m map[string]any
	json.Unmarshal([]byte(read(t, w.path(".cursor", "mcp.json"))), &m)
	sb := m["mcpServers"].(map[string]any)["sourcebolt"].(map[string]any)
	if sb["command"] != installed || !slices.Equal(toStrings(sb["args"]), []string{"serve"}) {
		t.Fatalf("cursor entry = %v, want %q serve", sb, installed)
	}
}

// Run from a temp folder with nothing installed: refuse and change nothing.
func TestSetupFromTempFolderRefuses(t *testing.T) {
	w := newSetupWorld(t)
	selfPath = func() (string, error) {
		return filepath.Join(os.TempDir(), "go-build1", "b001", "exe", "sourcebolt"), nil
	}
	write(t, w.path(".cursor", "mcp.json"), cursorBefore)
	code, _, errOut := w.run(t, "", "--yes", "--no-login")
	if code != 1 || !strings.Contains(errOut, "Install sourcebolt first") {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
	if read(t, w.path(".cursor", "mcp.json")) != cursorBefore || len(backups(t, w.path(".cursor", "mcp.json"))) != 0 {
		t.Fatal("nothing may change")
	}
}

func toStrings(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

func TestSetupClaudeCodeReplaceRemovesFirst(t *testing.T) {
	w := newSetupWorld(t, "claude")
	write(t, w.path(".claude.json"), `{"mcpServers": {"sourcebolt": {"command": "old"}}}`)
	w.run(t, "", "--yes", "--no-login", "--client", "claude-code")
	if len(w.calls) != 2 || !slices.Equal(w.calls[0], []string{"fake-claude", "mcp", "remove", "--scope", "user", "sourcebolt"}) || w.calls[1][2] != "add" {
		t.Fatalf("calls = %q", w.calls)
	}
}

func TestSetupClaudeCodeWithoutCLIEditsFile(t *testing.T) {
	w := newSetupWorld(t)
	p := w.path(".claude.json")
	write(t, p, `{"numStartups": 1, "projects": {}}`)
	w.run(t, "", "--yes", "--no-login", "--client", "claude-code")
	if got := read(t, p); !strings.Contains(got, `"numStartups": 1`) || !strings.Contains(got, fakeExe) {
		t.Fatalf("~/.claude.json = %s", got)
	}
	// ~/.claude.json keeps changing while Claude Code runs; undo touches only
	// the sourcebolt entry.
	write(t, p, strings.Replace(read(t, p), `"numStartups": 1`, `"numStartups": 9`, 1))
	w.run(t, "", "--undo", "--client", "claude-code")
	if got := read(t, p); !strings.Contains(got, `"numStartups": 9`) || strings.Contains(got, "sourcebolt") {
		t.Fatalf("after undo = %s", got)
	}
}

// A failed app CLI changed nothing, so its backup is dropped (undo must not
// "restore" over a later change) and setup reports the failure.
func TestSetupCLIFailureKeepsNoBackup(t *testing.T) {
	w := newSetupWorld(t, "claude")
	p := w.path(".claude.json")
	write(t, p, `{"numStartups": 1}`)
	runTool = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("boom"), errors.New("exit status 1")
	}
	code, out, _ := w.run(t, "", "--yes", "--no-login", "--client", "claude-code")
	if code != 1 || !strings.Contains(out, "Could not connect Claude Code") || !strings.Contains(out, "boom") {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if bs := backups(t, p); len(bs) != 0 {
		t.Fatalf("backups left: %v", bs)
	}
}

func TestSetupVSCodeUsesCLI(t *testing.T) {
	w := newSetupWorld(t, "code")
	w.run(t, "", "--yes", "--no-login", "--client", "vscode")
	if len(w.calls) != 1 || w.calls[0][0] != "fake-code" || w.calls[0][1] != "--add-mcp" {
		t.Fatalf("calls = %q", w.calls)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(w.calls[0][2]), &got); err != nil {
		t.Fatal(err)
	}
	if got["name"] != "sourcebolt" || got["type"] != "stdio" || got["command"] != fakeExe || got["args"].([]any)[0] != "serve" {
		t.Fatalf("--add-mcp %v", got)
	}
}

func TestSetupClientFilterAndUnknownClient(t *testing.T) {
	w := newSetupWorld(t)
	write(t, w.path(".cursor", "mcp.json"), cursorBefore)
	write(t, w.path(".gemini", "settings.json"), `{}`)
	w.run(t, "", "--yes", "--no-login", "--client", "gemini")
	if read(t, w.path(".cursor", "mcp.json")) != cursorBefore {
		t.Error("--client gemini changed Cursor")
	}
	if !strings.Contains(read(t, w.path(".gemini", "settings.json")), fakeExe) {
		t.Error("--client gemini did not connect Gemini")
	}
	if code, _, errOut := w.run(t, "", "--client", "notepad"); code != 2 || !strings.Contains(errOut, "cursor") {
		t.Fatalf("unknown client: code=%d err=%q", code, errOut)
	}
}

func TestSetupSkipsNonJSONConfig(t *testing.T) {
	w := newSetupWorld(t)
	p := w.path(".cursor", "mcp.json")
	body := "{\n  // my servers\n  \"mcpServers\": {}\n}\n"
	write(t, p, body)
	_, out, _ := w.run(t, "", "--yes", "--no-login")
	if read(t, p) != body || len(backups(t, p)) != 0 {
		t.Fatal("a file with comments must be left alone")
	}
	if !strings.Contains(out, "by hand") {
		t.Errorf("out = %q", out)
	}
}

func TestSetupNoClientsFound(t *testing.T) {
	w := newSetupWorld(t)
	code, out, _ := w.run(t, "", "--yes", "--no-login")
	if code != 0 || !strings.Contains(out, "No supported AI apps") || !strings.Contains(out, "clients.md") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestSetupRunsLoginWithoutProfile(t *testing.T) {
	w := newSetupWorld(t)
	s := stubAuto(t, nil, []*session.Jar{fakeJar(validNames...)}, nil)
	code, out, errOut := w.run(t, "", "--yes")
	if code != 0 || s.started == nil {
		t.Fatalf("code=%d started=%v err=%q", code, s.started != nil, errOut)
	}
	if want := filepath.Join(w.config, "browser", "default"); s.started.ProfileDir != want {
		t.Errorf("ProfileDir = %q", s.started.ProfileDir)
	}
	if !strings.Contains(out, "7 notebooks") || w.doctorRuns != 1 {
		t.Errorf("out=%q doctor=%d", out, w.doctorRuns)
	}
}

func TestSetupSkipsLoginWhenSignedIn(t *testing.T) {
	w := newSetupWorld(t)
	s := stubAuto(t, nil, nil, nil)
	store := session.NewFileStore(filepath.Join(w.config, "profiles"), session.OSKeyring{})
	if err := store.Save(&session.Profile{Name: "default", Host: session.HostDefault, Jar: fakeJar(validNames...)}); err != nil {
		t.Fatal(err)
	}
	code, out, _ := w.run(t, "", "--yes")
	if code != 0 || s.started != nil || !strings.Contains(out, "already signed in") {
		t.Fatalf("code=%d started=%v out=%q", code, s.started != nil, out)
	}
}

func TestSetupLoginFailureExitsOne(t *testing.T) {
	w := newSetupWorld(t)
	s := stubAuto(t, nil, nil, nil)
	close(s.plain.done) // user closes the window
	code, out, errOut := w.run(t, "", "--yes")
	if code != 1 || !strings.Contains(errOut+out, "sourcebolt login") || w.doctorRuns != 0 {
		t.Fatalf("code=%d out=%q err=%q doctor=%d", code, out, errOut, w.doctorRuns)
	}
}
