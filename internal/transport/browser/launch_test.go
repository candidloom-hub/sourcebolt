package browser

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/chromedp/chromedp"
)

// captureArgs runs the allocator against a program that does not exist and
// returns the command line chromedp built. No browser starts.
func captureArgs(t *testing.T, o Options) []string {
	t.Helper()
	var args []string
	opts := append(allocatorOptions(o), chromedp.ModifyCmdFunc(func(cmd *exec.Cmd) {
		args = slices.Clone(cmd.Args[1:])
	}))
	actx, cancel := chromedp.NewExecAllocator(t.Context(), opts...)
	defer cancel()
	cctx, ccancel := chromedp.NewContext(actx)
	defer ccancel()
	if err := chromedp.Do(cctx); err == nil {
		t.Fatal("a program that does not exist must fail to start")
	}
	if args == nil {
		t.Fatal("command line was not captured")
	}
	return args
}

func hasFlag(args []string, name string) bool {
	return slices.ContainsFunc(args, func(a string) bool {
		return a == "--"+name || strings.HasPrefix(a, "--"+name+"=")
	})
}

func TestAllocatorFlags(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "browser", "default")
	fake := filepath.Join(t.TempDir(), "no-such-browser.exe")
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		// chromedp refuses a visible window on Linux without a display before
		// it builds any arguments; CI runners are headless.
		t.Setenv("DISPLAY", ":99")
	}
	if runtime.GOOS != "windows" {
		// An empty executable passes path lookup and then fails to start
		// (exec format error) after the command line exists.
		if err := os.WriteFile(fake, nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, headless := range []bool{false, true} {
		args := captureArgs(t, Options{ProfileDir: dir, ExecPath: fake, Headless: headless})
		if hasFlag(args, "enable-automation") {
			t.Errorf("headless=%v: --enable-automation must be removed: %q", headless, args)
		}
		if !slices.Contains(args, "--user-data-dir="+dir) {
			t.Errorf("headless=%v: dedicated user-data-dir missing: %q", headless, args)
		}
		if got := hasFlag(args, "headless"); got != headless {
			t.Errorf("headless=%v: --headless present=%v: %q", headless, got, args)
		}
		if !hasFlag(args, "no-first-run") {
			t.Errorf("headless=%v: --no-first-run missing", headless)
		}
	}
}

func TestFindBrowserOrder(t *testing.T) {
	brave := `C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe`
	chrome := `C:\Program Files\Google\Chrome\Application\chrome.exe`
	edge := `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`
	env := func(present []string, vars map[string]string) finder {
		return finder{
			goos:     "windows",
			getenv:   func(k string) string { return vars[k] },
			exists:   func(p string) bool { return slices.Contains(present, p) },
			lookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		}
	}
	tests := []struct {
		name     string
		explicit string
		vars     map[string]string
		present  []string
		want     string
		wantErr  bool
	}{
		{name: "flag wins", explicit: `D:\x\custom.exe`, vars: map[string]string{"SOURCEBOLT_BROWSER": `D:\env.exe`}, present: []string{`D:\x\custom.exe`, `D:\env.exe`, brave}, want: `D:\x\custom.exe`},
		{name: "env before installed", vars: map[string]string{"SOURCEBOLT_BROWSER": `D:\env.exe`}, present: []string{`D:\env.exe`, brave}, want: `D:\env.exe`},
		{name: "brave before chrome", present: []string{chrome, brave, edge}, want: brave},
		{name: "chrome before edge", present: []string{edge, chrome}, want: chrome},
		{name: "edge last resort", present: []string{edge}, want: edge},
		{name: "flag path missing is an error", explicit: `D:\missing.exe`, present: []string{brave}, wantErr: true},
		{name: "env path missing is an error", vars: map[string]string{"SOURCEBOLT_BROWSER": `D:\missing.exe`}, present: []string{brave}, wantErr: true},
		{name: "none installed", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := env(tt.present, tt.vars).find(tt.explicit)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("got %q, want error", got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestFindBrowserLinuxUsesPath(t *testing.T) {
	f := finder{
		goos:   "linux",
		getenv: func(string) string { return "" },
		exists: func(string) bool { return false },
		lookPath: func(name string) (string, error) {
			if name == "google-chrome" {
				return "/usr/bin/google-chrome", nil
			}
			return "", exec.ErrNotFound
		},
	}
	got, err := f.find("")
	if err != nil || got != "/usr/bin/google-chrome" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestNoBrowserErrorNamesFlagAndEnv(t *testing.T) {
	f := finder{goos: "windows", getenv: func(string) string { return "" }, exists: func(string) bool { return false },
		lookPath: func(string) (string, error) { return "", exec.ErrNotFound }}
	_, err := f.find("")
	if !errors.Is(err, ErrNoBrowser) {
		t.Fatalf("err = %v, want ErrNoBrowser", err)
	}
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindUnsupported {
		t.Fatalf("err = %#v, want KindUnsupported", err)
	}
	for _, s := range []string{"--browser-path", "SOURCEBOLT_BROWSER"} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("error %q does not name %s", err, s)
		}
	}
}

func TestLaunchNeedsProfileDir(t *testing.T) {
	_, err := Launch(t.Context(), Options{ExecPath: "x"})
	if err == nil || !strings.Contains(err.Error(), "ProfileDir") {
		t.Fatalf("err = %v, want a ProfileDir error", err)
	}
}
