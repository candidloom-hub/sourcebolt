package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/app"
	"github.com/candidloom-hub/sourcebolt/internal/session"
)

func cmdLogin(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(stderr)
	importFrom := fs.String("import", "", "cookie export to import: a file path, or - for stdin")
	format := fs.String("format", "", "netscape, json or header (auto-detected when empty)")
	profile := fs.String("profile", "", "profile name (default $NOTEBOOKLM_PROFILE or \"default\")")
	host := fs.String("host", "", "NotebookLM host (default notebook.google.com)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *importFrom == "" {
		fmt.Fprintln(stderr, "usage: sourcebolt login --import <file|-> [--format netscape|json|header]")
		fmt.Fprintln(stderr, "Browser login (--browser, --attach) arrives in a later release.")
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

	var raw []byte
	if *importFrom == "-" {
		raw, err = io.ReadAll(io.LimitReader(stdin, 4<<20))
	} else {
		raw, err = os.ReadFile(*importFrom)
	}
	if err != nil {
		return fail(stderr, err)
	}
	jar, err := parseCookies(raw, *format, h)
	if err != nil {
		return fail(stderr, err)
	}
	if !jar.Valid() {
		return fail(stderr, errors.New("missing required cookies: need SID, __Secure-1PSIDTS and OSID (or APISID + SAPISID); export all google.com cookies"))
	}

	// Validate with a real call before saving, so a bad export never
	// replaces a working profile.
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
		return fail(stderr, fmt.Errorf("cookies did not authenticate with %s: %w", p.Host, err))
	}
	if err := store.Save(p); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "Saved profile %q for %s (%d notebooks visible).\n", p.Name, p.Host, len(nbs))
	return 0
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
