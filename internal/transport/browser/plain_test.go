package browser

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"
)

// The plain login browser must look like a browser a person started: only a
// debugging port and the dedicated profile, never chromedp's automation
// flags. Google refuses sign-in in automation-launched browsers.
func TestPlainArgs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "browser", "default")
	args := PlainArgs(PlainOptions{ProfileDir: dir, Port: 9333, StartURL: "https://notebook.google.com/"})
	for _, want := range []string{"--remote-debugging-port=9333", "--user-data-dir=" + dir} {
		if !slices.Contains(args, want) {
			t.Errorf("args lack %q: %q", want, args)
		}
	}
	if args[len(args)-1] != "https://notebook.google.com/" {
		t.Errorf("start url must be the last argument: %q", args)
	}
	for _, bad := range []string{"enable-automation", "headless", "disable-extensions", "remote-debugging-pipe",
		"disable-blink-features", "mute-audio", "hide-scrollbars", "disable-background-networking"} {
		if hasFlag(args, bad) {
			t.Errorf("plain browser must not get --%s: %q", bad, args)
		}
	}
	if len(args) != 3 {
		t.Errorf("want exactly port, profile and url, got %q", args)
	}
}

func TestFreePortIsLoopbackAndFree(t *testing.T) {
	port, err := FreePort()
	if err != nil || port <= 0 {
		t.Fatalf("port = %d, err = %v", port, err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		t.Fatalf("port %d is not free: %v", port, err)
	}
	l.Close()
}

func TestPlainDevToolsURL(t *testing.T) {
	p := &Plain{port: 9444}
	if got := p.DevToolsURL(); got != "http://127.0.0.1:9444" {
		t.Fatalf("DevToolsURL = %q", got)
	}
}

func TestStartPlainNeedsProfileDir(t *testing.T) {
	if _, err := StartPlain(PlainOptions{ExecPath: os.Args[0], Port: 1}); err == nil {
		t.Fatal("an empty profile dir must be refused (never the default profile)")
	}
}

// The test binary stands in for the browser: it rejects the browser flags
// and exits at once, so Done must close. No browser starts.
func TestStartPlainDoneWhenProcessExits(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "browser", "default")
	p, err := StartPlain(PlainOptions{ExecPath: os.Args[0], ProfileDir: dir, Port: 1, StartURL: "about:blank"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Done():
	case <-time.After(30 * time.Second):
		p.Close(context.Background())
		t.Fatal("Done did not close after the process exited")
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("profile dir not created: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Close(ctx); err != nil {
		t.Fatalf("Close after exit: %v", err)
	}
}
