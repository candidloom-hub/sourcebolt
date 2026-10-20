package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/app"
	"github.com/candidloom-hub/sourcebolt/internal/safety"
	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/setup"
)

// Outside effects of setup; tests replace them.
var (
	setupEnv = setup.OSEnv
	selfPath = func() (string, error) {
		p, err := os.Executable()
		if err != nil {
			return "", err
		}
		if r, err := filepath.EvalSymlinks(p); err == nil {
			p = r
		}
		return p, nil
	}
	runTool = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, name, args...).CombinedOutput()
	}
	runDoctor = cmdDoctor
	setupNow  = time.Now
)

const serverName = "sourcebolt"

const setupUsage = `usage:
  sourcebolt setup [--yes] [--client name]... [--dry-run] [--no-login]
  sourcebolt setup --undo [--client name]...

clients: ` + "claude-desktop, claude-code, cursor, vscode, windsurf, gemini"

// stringList is a repeatable string flag.
type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

// asker asks yes/no questions on stdin; with --yes every answer is yes.
type asker struct {
	in  *bufio.Reader
	out io.Writer
	yes bool
}

func (a asker) ask(q string, def bool) bool {
	if a.yes {
		return true
	}
	hint := "[y/N]"
	if def {
		hint = "[Y/n]"
	}
	fmt.Fprintf(a.out, "%s %s ", q, hint)
	line, err := a.in.ReadString('\n')
	fmt.Fprintln(a.out)
	ans := strings.ToLower(strings.TrimSpace(line))
	if ans == "" && err != nil && line == "" {
		return def // no more input
	}
	switch ans {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	}
	return def
}

func cmdSetup(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	yes := fs.Bool("yes", false, "answer yes to every question (connect every app found, replace old entries)")
	var only stringList
	fs.Var(&only, "client", "only this app (repeatable): "+strings.Join(setup.IDs, ", "))
	dry := fs.Bool("dry-run", false, "print what would change and change nothing")
	noLogin := fs.Bool("no-login", false, "do not open the browser to sign in")
	undo := fs.Bool("undo", false, "put back each app's config from its newest sourcebolt backup")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	for _, id := range only {
		if !slices.Contains(setup.IDs, id) {
			fmt.Fprintf(stderr, "unknown --client %q\n%s\n", id, setupUsage)
			return 2
		}
	}
	clients := setup.Clients(setupEnv())
	if len(only) > 0 {
		clients = slices.DeleteFunc(clients, func(c setup.Client) bool { return !slices.Contains(only, c.ID) })
	}
	if *undo {
		return undoSetup(clients, len(only) > 0, stdout, stderr)
	}

	exe, err := selfPath()
	if err != nil {
		return fail(stderr, fmt.Errorf("cannot find the sourcebolt program's own path: %w", err))
	}
	a := asker{in: bufio.NewReader(stdin), out: stdout, yes: *yes}
	if *dry {
		fmt.Fprintln(stdout, "Dry run: nothing will be changed.")
	}
	fmt.Fprintln(stdout, "Looking for AI apps on this computer...")
	var found []setup.Client
	for _, c := range clients {
		if c.Installed {
			found = append(found, c)
			fmt.Fprintf(stdout, "  found %s\n", c.Name)
		} else if len(only) > 0 {
			fmt.Fprintf(stdout, "  %s was not found on this computer; skipping it.\n", c.Name)
		}
	}
	if len(found) == 0 {
		fmt.Fprintln(stdout, "No supported AI apps were found. You can connect another app by hand: see docs/clients.md.")
	}
	fmt.Fprintln(stdout)

	var connected []string
	failed := false
	for _, c := range found {
		if !a.ask(fmt.Sprintf("Connect sourcebolt to %s?", c.Name), true) {
			fmt.Fprintf(stdout, "Skipped %s.\n", c.Name)
			continue
		}
		ok, err := connectClient(ctx, c, exe, a, *dry, stdout)
		if err != nil {
			failed = true
			fmt.Fprintf(stdout, "Could not connect %s: %s\nYou can add it by hand: see docs/clients.md.\n", c.Name, safety.Redact(err.Error()))
			continue
		}
		if ok {
			connected = append(connected, c.Name)
		}
	}
	fmt.Fprintln(stdout)

	if *dry {
		if !*noLogin {
			fmt.Fprintln(stdout, "Would open a browser window so you can sign in to Google, unless you are already signed in.")
		}
		fmt.Fprintln(stdout, "Would run the doctor checks. Run again without --dry-run to make these changes.")
		return 0
	}

	if !*noLogin {
		if code := setupLogin(ctx, stdout, stderr); code != 0 {
			fmt.Fprintln(stderr, "Your apps are connected, but sign-in did not finish. Run: sourcebolt login")
			return code
		}
	}
	fmt.Fprintln(stdout, "\nChecking the setup:")
	doctorFailed := runDoctor(ctx, nil, stdout, stderr) != 0

	switch {
	case len(connected) > 0:
		fmt.Fprintf(stdout, "\nDone. Restart %s and ask: 'List my NotebookLM notebooks'\n", strings.Join(connected, ", "))
	default:
		fmt.Fprintln(stdout, "\nDone. Connect your AI app (docs/clients.md), then ask: 'List my NotebookLM notebooks'")
	}
	if doctorFailed {
		fmt.Fprintln(stdout, "Some checks failed. See docs/troubleshooting.md.")
	}
	if failed || doctorFailed {
		return 1
	}
	return 0
}

// connectClient adds the sourcebolt entry to one client. ok is false when
// nothing was (or would be) changed.
func connectClient(ctx context.Context, c setup.Client, exe string, a asker, dry bool, stdout io.Writer) (ok bool, err error) {
	entry := c.Entry(exe)
	data, err := os.ReadFile(c.Path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	old, has, err := setup.Lookup(data, c.Key, serverName)
	if err != nil {
		return false, fmt.Errorf("%s: %w", c.Path, err)
	}
	if has && old.Command == entry.Command && slices.Equal(old.Args, entry.Args) {
		fmt.Fprintf(stdout, "%s is already connected.\n", c.Name)
		return true, nil
	}
	if has && !a.ask(fmt.Sprintf("%s already has a sourcebolt entry (command %q). Replace it?", c.Name, old.Command), false) {
		fmt.Fprintf(stdout, "Kept the existing entry in %s.\n", c.Name)
		return false, nil
	}

	cmds := cliCommands(c, entry, has)
	if dry {
		fmt.Fprintf(stdout, "Would back up %s to %s.sourcebolt-backup-<time>.\n", c.Path, c.Path)
		if cmds != nil {
			for _, cmd := range cmds {
				fmt.Fprintf(stdout, "Would run: %s\n", strings.Join(cmd, " "))
			}
		} else {
			b, _ := json.MarshalIndent(entry, "  ", "  ")
			fmt.Fprintf(stdout, "Would add to %s under %q:\n  %q: %s\n", c.Path, c.Key, serverName, b)
		}
		return true, nil
	}

	backup, err := setup.Backup(c.Path, setupNow())
	if err != nil {
		return false, err
	}
	if err := apply(ctx, c, entry, data, cmds); err != nil {
		os.Remove(backup) // nothing changed, so nothing to undo
		return false, err
	}
	fmt.Fprintf(stdout, "Connected %s (%s).\n  Backup: %s\n", c.Name, c.Path, backup)
	return true, nil
}

// cliCommands returns the app CLI commands to run instead of editing the
// file, or nil to edit the file.
func cliCommands(c setup.Client, e setup.Entry, replace bool) [][]string {
	switch {
	case c.ID == "claude-code" && c.CLI != "":
		add := append([]string{c.CLI, "mcp", "add", "--scope", "user", "--transport", "stdio", serverName, "--", e.Command}, e.Args...)
		if replace {
			return [][]string{{c.CLI, "mcp", "remove", "--scope", "user", serverName}, add}
		}
		return [][]string{add}
	case c.ID == "vscode" && c.CLI != "" && !replace:
		// `code --add-mcp` adds a new server; replacing edits the file we
		// found the old entry in.
		b, _ := json.Marshal(struct {
			Name string `json:"name"`
			setup.Entry
		}{serverName, e})
		return [][]string{{c.CLI, "--add-mcp", string(b)}}
	}
	return nil
}

func apply(ctx context.Context, c setup.Client, e setup.Entry, data []byte, cmds [][]string) error {
	if cmds != nil {
		for _, cmd := range cmds {
			cctx, cancel := context.WithTimeout(ctx, time.Minute)
			out, err := runTool(cctx, cmd[0], cmd[1:]...)
			cancel()
			if err != nil {
				return fmt.Errorf("%s failed: %w: %s", strings.Join(cmd[:min(3, len(cmd))], " "), err, strings.TrimSpace(string(out)))
			}
		}
		return nil
	}
	out, _, err := setup.Merge(data, c.Key, serverName, e)
	if err != nil {
		return err
	}
	return setup.WriteFile(c.Path, out)
}

// setupLogin signs in unless the profile already holds a valid session.
func setupLogin(ctx context.Context, stdout, stderr io.Writer) int {
	cfg, err := app.ConfigFromEnv()
	if err != nil {
		return fail(stderr, err)
	}
	if p, err := cfg.Store().Load(cfg.Profile); err == nil && p.Jar.Valid() {
		fmt.Fprintf(stdout, "You are already signed in to NotebookLM (profile %q).\n", cfg.Profile)
		return 0
	}
	h := session.HostDefault
	if cfg.Host != "" {
		h = cfg.Host
	}
	fmt.Fprintln(stdout, "Next: sign in to NotebookLM.")
	return autoLogin(ctx, cfg, h, "", defaultAutoTimeout, stdout, stderr)
}

// undoSetup puts back each client's newest backup. Claude Code keeps
// rewriting ~/.claude.json, so there only the sourcebolt entry is undone.
func undoSetup(clients []setup.Client, named bool, stdout, stderr io.Writer) int {
	code := 0
	undone := 0
	for _, c := range clients {
		var from string
		var err error
		if c.ID == "claude-code" {
			from, err = setup.RestoreEntry(c.Path, c.Key, serverName)
		} else {
			from, err = setup.Restore(c.Path)
		}
		switch {
		case errors.Is(err, setup.ErrNoBackup):
			if named {
				fmt.Fprintf(stdout, "Nothing to undo for %s.\n", c.Name)
			}
		case err != nil:
			code = 1
			fmt.Fprintf(stderr, "Could not undo %s: %s\n", c.Name, safety.Redact(err.Error()))
		default:
			undone++
			fmt.Fprintf(stdout, "Restored %s from %s. Restart %s.\n", c.Name, filepath.Base(from), c.Name)
		}
	}
	if undone == 0 && code == 0 {
		fmt.Fprintln(stdout, "Nothing to undo: setup has not changed any app.")
	}
	return code
}
