//go:build browser

package browser

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// fakeNotebook mimics the sign-in flow with fake values only: "/" redirects
// to a sign-in page, which moves on to "/app" after a moment. "/app" sets
// session cookies and serves WIZ_global_data with SNlM0e and oPEP7c.
func fakeNotebook(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/signin", http.StatusFound)
	})
	mux.HandleFunc("/signin", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>sign in<script>setTimeout(() => location.href = "/app", 300)</script></body></html>`)
	})
	mux.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		for _, c := range []*http.Cookie{
			{Name: "SID", Value: "fake-sid", Path: "/", MaxAge: 3600},
			{Name: "HSID", Value: "fake-hsid", Path: "/", MaxAge: 3600, HttpOnly: true},
			{Name: "APISID", Value: "fake-apisid", Path: "/", MaxAge: 3600},
			{Name: "NID", Value: "fake-nid", Path: "/"},
		} {
			http.SetCookie(w, c)
		}
		fmt.Fprint(w, `<html lang="en"><body>app<script>window.WIZ_global_data = {"SNlM0e":"fake-at","oPEP7c":"user@example.test"};</script></body></html>`)
	})
	mux.HandleFunc("/never", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>signed out shell<script>window.WIZ_global_data = {};</script></body></html>`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func hostOf(srv *httptest.Server) session.Host {
	return session.Host(strings.TrimPrefix(srv.URL, "http://"))
}

func TestLoginInteractiveLocal(t *testing.T) {
	srv := fakeNotebook(t)
	h := hostOf(srv)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	// Visible window: the real login path.
	o := Options{ProfileDir: profileDir(t), Headless: false, startURL: srv.URL + "/"}
	jar, landed, err := LoginInteractive(ctx, o, h)
	if err != nil {
		t.Fatal(err)
	}
	if landed != h {
		t.Errorf("landed = %q, want %q", landed, h)
	}
	got := map[string]session.Cookie{}
	for _, c := range jar.All() {
		got[c.Name] = c
	}
	for _, n := range []string{"SID", "HSID", "APISID", "NID"} {
		if _, ok := got[n]; !ok {
			t.Errorf("cookie %s missing", n)
		}
	}
	if !got["HSID"].HTTPOnly {
		t.Error("HSID must keep its HttpOnly flag (read through CDP)")
	}
	if got["SID"].Expires.IsZero() {
		t.Error("SID must keep its expiry")
	}
	if !got["NID"].Expires.IsZero() {
		t.Error("NID is a session cookie and must have no expiry")
	}
	if got["SID"].Domain != "127.0.0.1" {
		t.Errorf("SID domain = %q, want host-only 127.0.0.1", got["SID"].Domain)
	}
}

func TestLoginInteractiveWindowClosed(t *testing.T) {
	srv := fakeNotebook(t)
	afterNavigate = func(b *Browser, tab context.Context) {
		// What the user does: close the only tab, which closes the window
		// and the browser.
		id := chromedp.FromContext(tab).Target.TargetID
		go chromedp.CallBrowser(b.tab, target.CloseTarget, target.CloseTargetParams{TargetID: id})
	}
	t.Cleanup(func() { afterNavigate = nil })
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	o := Options{ProfileDir: profileDir(t), Headless: true, startURL: srv.URL + "/never"}
	jar, _, err := LoginInteractive(ctx, o, hostOf(srv))
	if !errors.Is(err, ErrCancelled) || !strings.Contains(err.Error(), "CANCELLED") {
		t.Fatalf("err = %v, want CANCELLED", err)
	}
	if jar != nil {
		t.Fatal("no jar may be returned after cancel")
	}
}

func TestLoginInteractiveBrowserExited(t *testing.T) {
	srv := fakeNotebook(t)
	afterNavigate = func(b *Browser, tab context.Context) {
		// Closing the last window of a visible browser makes the process
		// exit and the DevTools pipe drop; killing it has the same effect.
		if err := chromedp.FromContext(b.tab).Browser.Process().Kill(); err != nil {
			t.Errorf("kill: %v", err)
		}
	}
	t.Cleanup(func() { afterNavigate = nil })
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	start := time.Now()
	o := Options{ProfileDir: profileDir(t), Headless: true, startURL: srv.URL + "/never"}
	_, _, err := LoginInteractive(ctx, o, hostOf(srv))
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("err = %v, want CANCELLED", err)
	}
	if d := time.Since(start); d > 15*time.Second {
		t.Errorf("cancel took %v", d)
	}
}

func TestLoginInteractiveTimeout(t *testing.T) {
	srv := fakeNotebook(t)
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()
	o := Options{ProfileDir: profileDir(t), Headless: true, startURL: srv.URL + "/never"}
	_, _, err := LoginInteractive(ctx, o, hostOf(srv))
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want TIMEOUT", err)
	}
}

// startDebuggable starts a browser with a DevTools port, as a user would
// with --remote-debugging-port, and returns its http address.
func startDebuggable(t *testing.T, pageURL string) (string, *exec.Cmd) {
	t.Helper()
	exe, err := FindBrowser("")
	if err != nil {
		t.Skip(err)
	}
	dir := profileDir(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "--headless", "--no-first-run", "--no-default-browser-check",
		"--remote-debugging-port=0", "--user-data-dir="+dir, pageURL)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
	portFile := filepath.Join(dir, "DevToolsActivePort")
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if f, err := os.Open(portFile); err == nil {
			sc := bufio.NewScanner(f)
			ok := sc.Scan()
			f.Close()
			if ok && sc.Text() != "" {
				return "http://127.0.0.1:" + sc.Text(), cmd
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser did not write DevToolsActivePort")
	return "", nil
}

func TestAttachCDP(t *testing.T) {
	srv := fakeNotebook(t)
	addr, cmd := startDebuggable(t, srv.URL+"/app")
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	var jar *session.Jar
	var err error
	for ctx.Err() == nil {
		jar, err = AttachCDP(ctx, addr, hostOf(srv))
		if err == nil && jar.Has("SID", "HSID") {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil || !jar.Has("SID", "HSID", "APISID") {
		t.Fatalf("attach: err=%v", err)
	}
	// Attaching must leave the user's browser running.
	time.Sleep(500 * time.Millisecond)
	if cmd.ProcessState != nil {
		t.Fatal("AttachCDP closed the browser it attached to")
	}
	if _, err := AttachCDP(ctx, addr, hostOf(srv)); err != nil {
		t.Fatalf("second attach failed, browser likely closed: %v", err)
	}
}
