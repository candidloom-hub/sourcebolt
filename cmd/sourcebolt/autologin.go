package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/app"
	"github.com/candidloom-hub/sourcebolt/internal/safety"
	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport/browser"
)

// plainBrowser is the browser process that `sourcebolt login` (no flags)
// starts for the user. *browser.Plain implements it; tests use a fake.
type plainBrowser interface {
	DevToolsURL() string
	Done() <-chan struct{}
	Close(ctx context.Context) error
}

// Outside effects of the no-flag login; tests replace them.
var (
	findBrowser = browser.FindBrowser
	freePort    = browser.FreePort
	startPlain  = func(o browser.PlainOptions) (plainBrowser, error) { return browser.StartPlain(o) }
	pollEvery   = 2 * time.Second
)

// defaultAutoTimeout is how long `sourcebolt login` waits for sign-in.
const defaultAutoTimeout = 10 * time.Minute

const fallbackHint = "If Google will not let you sign in there, use one of the other ways: sourcebolt login --attach (a browser you start yourself) or sourcebolt login --import (a cookie file). See docs/auth.md"

// autoLogin starts a normal browser window on the profile's browser dir,
// waits until the user has signed in (the cookies read over the debugging
// port are complete and list notebooks), saves the profile and closes the
// window. It types nothing into the page.
func autoLogin(ctx context.Context, cfg app.Config, h session.Host, browserPath string, timeout time.Duration, stdout, stderr io.Writer) int {
	if err := safeProfileName(cfg.Profile); err != nil {
		return fail(stderr, err)
	}
	exe, err := findBrowser(browserPath)
	if err != nil {
		msg := safety.Redact(err.Error())
		if errors.Is(err, browser.ErrNoBrowser) {
			msg = "no Chrome, Edge or Brave browser was found on this computer. Install one of them and run this again, or set " + browser.EnvBrowser + " to the browser program"
		}
		fmt.Fprintf(stderr, "error: %s.\n%s\n", msg, fallbackHint)
		return 1
	}
	port, err := freePort()
	if err != nil {
		return fail(stderr, err)
	}
	pb, err := startPlain(browser.PlainOptions{
		ExecPath:   exe,
		ProfileDir: filepath.Join(cfg.ConfigDir, "browser", cfg.Profile),
		Port:       port,
		StartURL:   h.Origin() + "/",
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: could not open the browser: %s\n%s\n", safety.Redact(err.Error()), fallbackHint)
		return 1
	}
	closeBrowser := func() {
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		_ = pb.Close(cctx)
		cancel()
	}

	fmt.Fprintf(stderr, "A browser window opened. Sign in to your Google account there.\nThis window will finish by itself once you see your notebooks (waiting up to %s).\nClose the browser window to cancel.\n", timeout)

	lctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	jar, n, err := waitForSession(lctx, pb, h)
	if err != nil {
		closeBrowser()
		switch {
		case errors.Is(err, browser.ErrCancelled):
			fmt.Fprintln(stderr, "error: sign-in cancelled: the browser window was closed; nothing saved.")
		case errors.Is(err, browser.ErrTimeout):
			fmt.Fprintf(stderr, "error: sign-in timed out after %s; nothing saved.\n%s\n%s\n", timeout, err, fallbackHint)
		default:
			fmt.Fprintf(stderr, "error: %s; nothing saved.\n", safety.Redact(err.Error()))
		}
		return 1
	}
	code := saveProfile(cfg, h, jar, n, stdout, stderr)
	closeBrowser()
	if code == 0 {
		fmt.Fprintln(stdout, "Signed in. The browser window was closed for you.")
	}
	return code
}

// waitForSession polls the browser until its cookies are complete and pass
// a real call. It returns the jar and the number of notebooks.
func waitForSession(ctx context.Context, pb plainBrowser, h session.Host) (*session.Jar, int, error) {
	t := time.NewTicker(pollEvery)
	defer t.Stop()
	var last error
	for {
		select {
		case <-pb.Done():
			return nil, 0, browser.ErrCancelled
		default:
		}
		if ctx.Err() != nil {
			return nil, 0, waitErr(ctx, last)
		}
		jar, err := attachBrowser(ctx, pb.DevToolsURL(), h)
		if err == nil && jar.Valid() {
			n, cerr := checkSession(ctx, &session.Profile{Name: "login-check", Host: h, Jar: jar})
			if cerr == nil {
				return jar, n, nil
			}
			last = fmt.Errorf("the cookies did not work yet: %s", safety.Redact(cerr.Error()))
		}
		select {
		case <-pb.Done():
			return nil, 0, browser.ErrCancelled
		case <-ctx.Done():
			return nil, 0, waitErr(ctx, last)
		case <-t.C:
		}
	}
}

func waitErr(ctx context.Context, last error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		if last != nil {
			return fmt.Errorf("%w (last problem: %w)", browser.ErrTimeout, last)
		}
		return browser.ErrTimeout
	}
	return browser.ErrCancelled
}
