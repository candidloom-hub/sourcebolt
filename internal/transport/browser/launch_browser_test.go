//go:build browser

// Tests in this file start a real browser. Run them with
// go test -tags browser ./internal/transport/browser -v
package browser

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// profileDir returns a fresh user-data-dir. Browser child processes can hold
// files for a moment after exit on Windows, so cleanup errors are ignored.
func profileDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "sourcebolt-browser-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for range 20 {
			if os.RemoveAll(dir) == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
	return filepath.Join(dir, "browser", "default")
}

func TestLaunchTabAndClose(t *testing.T) {
	b, err := Launch(t.Context(), Options{ProfileDir: profileDir(t), Headless: true})
	if err != nil {
		t.Fatal(err)
	}
	tab, release, err := b.Tab(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// Note: navigator.webdriver is true for any CDP-launched Chromium (pipe
	// or port), even without --enable-automation; hiding it would need a
	// stealth flag, which the project does not use.
	sum, err := chromedp.Run(tab, chromedp.Evaluate[int](`1 + 2`))
	release()
	if err != nil || sum != 3 {
		t.Fatalf("evaluate = %d, %v", sum, err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-b.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("browser did not stop after Close")
	}
	if _, _, err := b.Tab(t.Context()); err == nil {
		t.Fatal("Tab after Close must fail")
	}
}

func TestIdleReap(t *testing.T) {
	b, err := Launch(t.Context(), Options{ProfileDir: profileDir(t), Headless: true, Idle: 300 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	select {
	case <-b.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("idle browser was not reaped")
	}
}
