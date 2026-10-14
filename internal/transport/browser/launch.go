// Package browser drives a user-installed Chromium-based browser (Brave,
// Chrome, Edge, Chromium) over the DevTools protocol: interactive login,
// attaching to a running browser, and later the fallback transport.
//
// The browser always runs on a dedicated user-data-dir, never the user's
// default profile, and without chromedp's --enable-automation flag.
package browser

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/chromedp/chromedp"
)

// Options configures one browser process.
type Options struct {
	ProfileDir string        // dedicated user-data-dir, required: <config>/browser/<profile>
	Headless   bool          // false opens a visible window (login)
	ExecPath   string        // browser executable; empty = discover
	Idle       time.Duration // close the browser after this long without a Tab; 0 = never

	startURL string // tests: overrides https://<host>/ in LoginInteractive
}

// EnvBrowser names the environment variable that points at a browser.
const EnvBrowser = "SOURCEBOLT_BROWSER"

// ErrNoBrowser means no Chromium-based browser was found.
var ErrNoBrowser = errors.New("no Chromium-based browser found")

// allocatorOptions builds the chromedp allocator options: chromedp's
// defaults minus --enable-automation, on the dedicated profile dir.
func allocatorOptions(o Options) []chromedp.ExecAllocatorOption {
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts,
		chromedp.UserDataDir(o.ProfileDir),
		chromedp.ExecPath(o.ExecPath),
		chromedp.Flag("enable-automation", false),
	)
	if !o.Headless {
		// Removes headless, hide-scrollbars, mute-audio, enable-automation
		// and disable-extensions, and maximizes the window.
		opts = append(opts, chromedp.VisibleWindow)
	}
	return opts
}

// finder discovers a browser executable. Its funcs are injectable for tests.
type finder struct {
	goos     string
	getenv   func(string) string
	exists   func(string) bool
	lookPath func(string) (string, error)
}

func osFinder() finder {
	return finder{
		goos:   runtime.GOOS,
		getenv: os.Getenv,
		exists: func(p string) bool {
			st, err := os.Stat(p)
			return err == nil && !st.IsDir()
		},
		lookPath: exec.LookPath,
	}
}

// FindBrowser returns the browser to use: explicit (the --browser-path flag),
// then $SOURCEBOLT_BROWSER, then Brave, Chrome, Edge and Chromium.
func FindBrowser(explicit string) (string, error) { return osFinder().find(explicit) }

func (f finder) find(explicit string) (string, error) {
	if explicit != "" {
		if !f.exists(explicit) {
			return "", fmt.Errorf("browser: --browser-path %q does not exist", explicit)
		}
		return explicit, nil
	}
	if p := f.getenv(EnvBrowser); p != "" {
		if !f.exists(p) {
			return "", fmt.Errorf("browser: %s=%q does not exist", EnvBrowser, p)
		}
		return p, nil
	}
	for _, p := range f.candidates() {
		if f.exists(p) {
			return p, nil
		}
	}
	for _, name := range pathNames {
		if p, err := f.lookPath(name); err == nil {
			return p, nil
		}
	}
	return "", &transport.Error{
		Kind:      transport.KindUnsupported,
		Transport: "browser",
		Message:   "no Chromium-based browser found; install Brave, Chrome or Edge, or pass --browser-path <exe> or set " + EnvBrowser,
		Cause:     ErrNoBrowser,
	}
}

// pathNames are looked up on $PATH after the well-known install locations.
var pathNames = []string{
	"brave-browser", "brave",
	"google-chrome", "google-chrome-stable", "chrome",
	"microsoft-edge", "microsoft-edge-stable", "msedge",
	"chromium", "chromium-browser",
}

// candidates lists install locations in preference order: Brave, Chrome,
// Edge, Chromium.
func (f finder) candidates() []string {
	switch f.goos {
	case "windows":
		var out []string
		roots := []string{f.getenv("ProgramFiles"), f.getenv("ProgramFiles(x86)"), f.getenv("LOCALAPPDATA")}
		add := func(rel string) {
			for _, r := range append([]string{`C:\Program Files`, `C:\Program Files (x86)`}, roots...) {
				if r == "" {
					continue
				}
				p := r + `\` + rel
				if !slices.Contains(out, p) {
					out = append(out, p)
				}
			}
		}
		add(`BraveSoftware\Brave-Browser\Application\brave.exe`)
		add(`Google\Chrome\Application\chrome.exe`)
		add(`Microsoft\Edge\Application\msedge.exe`)
		add(`Chromium\Application\chrome.exe`)
		return out
	case "darwin":
		var out []string
		for _, app := range []string{
			"Brave Browser.app/Contents/MacOS/Brave Browser",
			"Google Chrome.app/Contents/MacOS/Google Chrome",
			"Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"Chromium.app/Contents/MacOS/Chromium",
		} {
			out = append(out, "/Applications/"+app)
			if home := f.getenv("HOME"); home != "" {
				out = append(out, filepath.Join(home, "Applications", app))
			}
		}
		return out
	default:
		return []string{
			"/usr/bin/brave-browser", "/usr/bin/brave", "/opt/brave.com/brave/brave", "/snap/bin/brave",
			"/usr/bin/google-chrome", "/usr/bin/google-chrome-stable", "/opt/google/chrome/chrome",
			"/usr/bin/microsoft-edge", "/usr/bin/microsoft-edge-stable", "/opt/microsoft/msedge/msedge",
			"/usr/bin/chromium", "/usr/bin/chromium-browser", "/snap/bin/chromium",
		}
	}
}
