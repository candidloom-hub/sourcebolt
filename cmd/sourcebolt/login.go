package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
  sourcebolt login --browser [--browser-path exe] [--timeout 300s] [--profile p] [--host h]
  sourcebolt login --attach <cdp ws:// or http:// url> [--profile p] [--host h]
  sourcebolt login --import <file|-> [--format netscape|json|header] [--profile p] [--host h]`

func cmdLogin(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(stderr)
	useBrowser := fs.Bool("browser", false, "open a browser window on a dedicated profile and wait for you to sign in")
	attach := fs.String("attach", "", "read cookies from a running browser started with --remote-debugging-port (ws:// or http:// DevTools url)")
	browserPath := fs.String("browser-path", "", "browser executable (default $"+browser.EnvBrowser+", then Brave, Chrome, Edge, Chromium)")
	timeout := fs.Duration("timeout", 300*time.Second, "how long --browser waits for sign-in")
	importFrom := fs.String("import", "", "cookie export to import: a file path, or - for stdin")
	format := fs.String("format", "", "netscape, json or header (auto-detected when empty)")
	profile := fs.String("profile", "", "profile name (default $NOTEBOOKLM_PROFILE or \"default\")")
	host := fs.String("host", "", "NotebookLM host (default notebook.google.com)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	methods := 0
	for _, set := range []bool{*useBrowser, *attach != "", *importFrom != ""} {
		if set {
			methods++
		}
	}
	if methods != 1 {
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
			return fail(stderr, fmt.Errorf("missing required cookies: %s. Export every google.com cookie, including the plain SID, HSID and APISID ones, not only the __Secure- ones", strings.Join(jar.Missing(), ", ")))
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
	store := cfg.Store()
	p := &session.Profile{Name: cfg.Profile, Host: h, Jar: jar}
	if old, err := store.Load(cfg.Profile); err == nil {
		p.Rev = old.Rev
	}
	rt := app.ForProfile(p, nil)
	vctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	nbs, err := rt.Deps.NLM.ListNotebooks(vctx)
	if err != nil {
		return fail(stderr, fmt.Errorf("cookies did not authenticate with %s: %s", p.Host, safety.Redact(err.Error())))
	}
	if err := store.Save(p); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "Saved profile %q for %s (%d notebooks visible).\n", p.Name, p.Host, len(nbs))
	return 0
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
