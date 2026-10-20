package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/app"
	"github.com/candidloom-hub/sourcebolt/internal/safety"
	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport/browser"
)

// Browser entry points; tests replace them.
var (
	loginBrowser  = browser.LoginInteractive
	attachBrowser = browser.AttachCDP
)

const loginUsage = `usage:
  sourcebolt login [--timeout 10m] [--browser-path exe] [--profile p] [--host h]
  sourcebolt login --browser [--browser-path exe] [--timeout 300s] [--profile p] [--host h]
  sourcebolt login --attach <cdp ws:// or http:// url> [--profile p] [--host h]
  sourcebolt login --import <file|-> [--format netscape|json|header] [--profile p] [--host h]`

func cmdLogin(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(stderr)
	useBrowser := fs.Bool("browser", false, "open a browser window on a dedicated profile and wait for you to sign in")
	attach := fs.String("attach", "", "read cookies from a running browser started with --remote-debugging-port (ws:// or http:// DevTools url)")
	browserPath := fs.String("browser-path", "", "browser executable (default $"+browser.EnvBrowser+", then Brave, Chrome, Edge, Chromium)")
	timeout := fs.Duration("timeout", 0, "how long to wait for sign-in (default 10m; 300s with --browser)")
	importFrom := fs.String("import", "", "cookie export to import: a file path, or - for stdin")
	format := fs.String("format", "", "netscape, json or header (auto-detected when empty)")
	profile := fs.String("profile", "", "profile name (default $NOTEBOOKLM_PROFILE or \"default\")")
	host := fs.String("host", "", "NotebookLM host (default notebook.google.com)")
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}
	methods := 0
	for _, set := range []bool{*useBrowser, *attach != "", *importFrom != ""} {
		if set {
			methods++
		}
	}
	if methods > 1 {
		fmt.Fprintln(stderr, loginUsage)
		return 2
	}
	cfg, err := app.ConfigFromEnv()
	if err != nil {
		return fail(stderr, err)
	}
	if *profile != "" {
		cfg.Profile = *profile
	}
	h := session.HostDefault
	if *host != "" {
		h = session.Host(*host)
	} else if cfg.Host != "" {
		h = cfg.Host
	}

	if methods == 0 {
		t := *timeout
		if t <= 0 {
			t = defaultAutoTimeout
		}
		return autoLogin(ctx, cfg, h, *browserPath, t, stdout, stderr)
	}
	if *timeout <= 0 {
		*timeout = 300 * time.Second
	}

	var jar *session.Jar
	switch {
	case *useBrowser:
		if err := safeProfileName(cfg.Profile); err != nil {
			return fail(stderr, err)
		}
		o := browser.Options{
			ProfileDir: filepath.Join(cfg.ConfigDir, "browser", cfg.Profile),
			ExecPath:   *browserPath,
		}
		fmt.Fprintf(stderr, "Opening a browser window (profile dir %s).\nSign in to Google there; waiting up to %s. Close the window to cancel.\n", o.ProfileDir, *timeout)
		lctx, cancel := context.WithTimeout(ctx, *timeout)
		var landed session.Host
		jar, landed, err = loginBrowser(lctx, o, h)
		cancel()
		if err != nil {
			return failBrowser(stderr, err)
		}
		if landed != "" {
			h = landed // record where the account actually landed
		}
	case *attach != "":
		jar, err = attachBrowser(ctx, *attach, h)
		if err != nil {
			return failBrowser(stderr, err)
		}
	default:
		var raw []byte
		if *importFrom == "-" {
			raw, err = io.ReadAll(io.LimitReader(stdin, 4<<20))
		} else {
			raw, err = os.ReadFile(*importFrom)
		}
		if err != nil {
			return fail(stderr, err)
		}
		jar, err = parseCookies(raw, *format, h)
		if err != nil {
			return fail(stderr, err)
		}
		if !jar.Valid() {
			return fail(stderr, fmt.Errorf("missing required cookies: %s. Export every google.com cookie (sourcebolt needs SID, __Secure-1PSIDTS, and OSID or APISID+SAPISID), not only the __Secure- ones", strings.Join(jar.Missing(), ", ")))
		}
	}
	if !jar.Valid() {
		return fail(stderr, fmt.Errorf("signed in, but missing required cookies: %s. Finish signing in to NotebookLM in the browser and try again; nothing was saved", strings.Join(jar.Missing(), ", ")))
	}
	return validateAndSave(ctx, cfg, h, jar, stdout, stderr)
}

// validateAndSave checks the cookies with a real call before saving, so a
// bad login never replaces a working profile.
func validateAndSave(ctx context.Context, cfg app.Config, h session.Host, jar *session.Jar, stdout, stderr io.Writer) int {
	n, err := checkSession(ctx, &session.Profile{Name: cfg.Profile, Host: h, Jar: jar})
	if err != nil {
		return fail(stderr, fmt.Errorf("cookies did not authenticate with %s: %s", h, safety.Redact(err.Error())))
	}
	return saveProfile(cfg, h, jar, n, stdout, stderr)
}

// checkSession proves a jar works with one real call (list notebooks) and
// returns the number of notebooks. Tests replace it.
var checkSession = func(ctx context.Context, p *session.Profile) (int, error) {
	rt := app.ForProfile(p, nil)
	vctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	nbs, err := rt.Deps.NLM.ListNotebooks(vctx)
	return len(nbs), err
}

// saveProfile stores a jar that already passed checkSession.
func saveProfile(cfg app.Config, h session.Host, jar *session.Jar, notebooks int, stdout, stderr io.Writer) int {
	store := cfg.Store()
	p := &session.Profile{Name: cfg.Profile, Host: h, Jar: jar}
	rev, replaced, err := prepareSave(store, cfg.Profile)
	if err != nil {
		return fail(stderr, err)
	}
	p.Rev = rev
	if err := store.Save(p); err != nil {
		return fail(stderr, err)
	}
	if replaced {
		fmt.Fprintf(stdout, "Replaced profile %q: its encryption key was missing from the OS keyring, so the old session could not be read.\n", p.Name)
	}
	fmt.Fprintf(stdout, "Saved profile %q for %s (%d notebooks visible).\n", p.Name, p.Host, notebooks)
	return 0
}

// prepareSave returns the revision a new login must save over. A profile
// that exists but whose encryption key is gone (keyring entry and key file
// both missing) can never be read again; it is deleted so the new login
// can be saved, and replaced is true. Other load errors are returned.
func prepareSave(store session.Store, name string) (rev int64, replaced bool, err error) {
	old, err := store.Load(name)
	if err == nil {
		return old.Rev, false, nil
	}
	names, lerr := store.List()
	if lerr != nil || !slices.Contains(names, name) {
		return 0, false, nil // no profile yet
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return 0, false, fmt.Errorf("profile %q exists but cannot be read (%w); run: sourcebolt logout --profile %s, then log in again", name, err, name)
	}
	if derr := store.Delete(name); derr != nil {
		return 0, false, derr
	}
	return 0, true, nil
}

func failBrowser(stderr io.Writer, err error) int {
	msg := safety.Redact(err.Error())
	if errors.Is(err, browser.ErrCancelled) || errors.Is(err, browser.ErrTimeout) {
		msg += "; nothing was saved"
	}
	fmt.Fprintln(stderr, "error:", msg)
	return 1
}

// safeProfileName keeps the profile name usable as a directory name under
// <config>/browser (the store applies the same rule to its files).
func safeProfileName(name string) error {
	if name == "" || strings.ContainsAny(name, `/\:*?"<>|`) || strings.HasPrefix(name, ".") {
		return fmt.Errorf("invalid profile name %q", name)
	}
	return nil
}

func parseCookies(raw []byte, format string, h session.Host) (*session.Jar, error) {
	if format == "" {
		t := bytes.TrimSpace(raw)
		switch {
		case bytes.HasPrefix(t, []byte("[")):
			format = "json"
		case bytes.Contains(t, []byte("\t")):
			format = "netscape"
		default:
			format = "header"
		}
	}
	switch format {
	case "json":
		return session.ImportJSON(bytes.NewReader(raw))
	case "netscape":
		return session.ImportNetscape(bytes.NewReader(raw))
	case "header":
		return session.ImportHeader(strings.TrimSpace(string(raw)), h)
	}
	return nil, fmt.Errorf("unknown --format %q", format)
}
