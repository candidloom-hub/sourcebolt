package setup

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestServerCommand(t *testing.T) {
	j := filepath.Join
	winHome := `C:\Users\u`
	winLocal := j(winHome, "AppData", "Local")
	winTemp := j(winLocal, "Temp")
	winStd := j(winLocal, "Programs", "sourcebolt", "sourcebolt.exe")
	unixStd := j("/home/u", ".local", "bin", "sourcebolt")
	winNpx := j(winLocal, "npm-cache", "_npx", "3f2a", "node_modules", "@candidloom-hub", "sourcebolt-win32-x64", "bin", "sourcebolt.exe")
	unixNpx := j("/home/u", ".npm", "_npx", "9c1d", "node_modules", "@candidloom-hub", "sourcebolt-linux-x64", "bin", "sourcebolt")
	goBuild := j("/tmp", "go-build123", "b001", "exe", "sourcebolt")
	npx := []string{"-y", "sourcebolt", "serve"}
	serve := []string{"serve"}

	tests := []struct {
		name     string
		goos     string
		exe      string
		existing []string
		onPath   map[string]string // program name -> path LookPath returns
		brew     string            // $HOMEBREW_PREFIX
		wantCmd  string
		wantArgs []string
		wantErr  bool
	}{
		{name: "installed binary is used as is", goos: "linux", exe: unixStd, wantCmd: unixStd, wantArgs: serve},
		{name: "any stable path is used as is", goos: "linux", exe: "/usr/bin/sourcebolt",
			onPath: map[string]string{"npx": "/usr/bin/npx"}, wantCmd: "/usr/bin/sourcebolt", wantArgs: serve},
		{name: "windows installed binary", goos: "windows", exe: winStd, wantCmd: winStd, wantArgs: serve},
		{name: "windows stable path elsewhere", goos: "windows", exe: j(`D:\tools`, "sourcebolt.exe"),
			wantCmd: j(`D:\tools`, "sourcebolt.exe"), wantArgs: serve},

		{name: "npx cache prefers the installed binary", goos: "windows", exe: winNpx,
			existing: []string{winStd}, onPath: map[string]string{"npx": `C:\node\npx.cmd`}, wantCmd: winStd, wantArgs: serve},
		{name: "npx cache without an install writes npx", goos: "windows", exe: winNpx,
			onPath: map[string]string{"npx": `C:\node\npx.cmd`}, wantCmd: "npx", wantArgs: npx},
		{name: "npx on unix without an install writes npx", goos: "linux", exe: unixNpx,
			onPath: map[string]string{"npx": "/usr/bin/npx"}, wantCmd: "npx", wantArgs: npx},
		{name: "npx cache prefers ~/.local/bin", goos: "linux", exe: unixNpx,
			existing: []string{unixStd}, onPath: map[string]string{"npx": "/usr/bin/npx"}, wantCmd: unixStd, wantArgs: serve},
		{name: "npm-cache folder counts as npm", goos: "windows",
			exe:    j(winLocal, "npm-cache", "x", "sourcebolt.exe"),
			onPath: map[string]string{"npx": `C:\node\npx.cmd`}, wantCmd: "npx", wantArgs: npx},
		{name: "global npm install writes npx", goos: "darwin",
			exe:    j("/usr/local/lib", "node_modules", "@candidloom-hub", "sourcebolt-darwin-arm64", "bin", "sourcebolt"),
			onPath: map[string]string{"npx": "/usr/local/bin/npx"}, wantCmd: "npx", wantArgs: npx},
		{name: "npx cache without npx or an install is refused", goos: "linux", exe: unixNpx, wantErr: true},

		{name: "temp dir prefers ~/.local/bin", goos: "linux", exe: goBuild,
			existing: []string{unixStd}, onPath: map[string]string{"npx": "/usr/bin/npx"}, wantCmd: unixStd, wantArgs: serve},
		{name: "temp dir uses sourcebolt from PATH", goos: "linux", exe: goBuild,
			onPath: map[string]string{"sourcebolt": "/opt/sb/sourcebolt", "npx": "/usr/bin/npx"}, wantCmd: "/opt/sb/sourcebolt", wantArgs: serve},
		{name: "windows temp dir uses sourcebolt from PATH", goos: "windows", exe: j(winTemp, "x", "sourcebolt.exe"),
			onPath: map[string]string{"sourcebolt": j(`D:\tools`, "sourcebolt.exe")}, wantCmd: j(`D:\tools`, "sourcebolt.exe"), wantArgs: serve},
		{name: "PATH copy inside the npx cache is skipped", goos: "linux", exe: goBuild,
			onPath:  map[string]string{"sourcebolt": j("/home/u", ".npm", "_npx", "9c1d", "node_modules", ".bin", "sourcebolt"), "npx": "/usr/bin/npx"},
			wantCmd: "npx", wantArgs: npx},
		{name: "PATH copy inside the temp dir is skipped", goos: "linux", exe: goBuild,
			onPath: map[string]string{"sourcebolt": j("/tmp", "other", "sourcebolt")}, wantErr: true},
		{name: "temp dir with npx on PATH writes npx", goos: "linux", exe: j("/tmp", "x", "sourcebolt"),
			onPath: map[string]string{"npx": "/usr/bin/npx"}, wantCmd: "npx", wantArgs: npx},
		{name: "temp dir without an install is refused", goos: "linux", exe: goBuild, wantErr: true},
		{name: "windows temp dir, any case, is refused", goos: "windows", exe: strings.ToUpper(j(winTemp, "go-build1", "sourcebolt.exe")), wantErr: true},
		{name: "go-build dir outside the temp dir is refused", goos: "linux", exe: j("/work", "gotmp", "go-build9", "b001", "exe", "sourcebolt"), wantErr: true},

		{name: "homebrew default prefix", goos: "darwin", exe: j("/tmp", "sourcebolt"),
			existing: []string{j("/opt/homebrew", "bin", "sourcebolt")}, wantCmd: j("/opt/homebrew", "bin", "sourcebolt"), wantArgs: serve},
		{name: "homebrew prefix from env", goos: "linux", exe: j("/tmp", "sourcebolt"), brew: "/hb",
			existing: []string{j("/hb", "bin", "sourcebolt")}, wantCmd: j("/hb", "bin", "sourcebolt"), wantArgs: serve},
		{name: "linuxbrew default prefix", goos: "linux", exe: j("/tmp", "sourcebolt"),
			existing: []string{j("/home/linuxbrew/.linuxbrew", "bin", "sourcebolt")}, wantCmd: j("/home/linuxbrew/.linuxbrew", "bin", "sourcebolt"), wantArgs: serve},
		{name: "homebrew versioned cask path prefers the bin link", goos: "darwin",
			exe:      j("/opt/homebrew", "Caskroom", "sourcebolt", "0.2.0", "sourcebolt"),
			existing: []string{j("/opt/homebrew", "bin", "sourcebolt")}, wantCmd: j("/opt/homebrew", "bin", "sourcebolt"), wantArgs: serve},
		{name: "homebrew versioned path without a link is kept, not npx", goos: "darwin",
			exe:     j("/opt/homebrew", "Cellar", "sourcebolt", "0.2.0", "bin", "sourcebolt"),
			onPath:  map[string]string{"npx": "/opt/homebrew/bin/npx"},
			wantCmd: j("/opt/homebrew", "Cellar", "sourcebolt", "0.2.0", "bin", "sourcebolt"), wantArgs: serve},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := fakeEnv(tc.goos, tc.existing)
			env.HomebrewPrefix = tc.brew
			env.LookPath = func(name string) (string, error) {
				if p, ok := tc.onPath[name]; ok {
					return p, nil
				}
				return "", errors.New("not found")
			}
			switch tc.goos {
			case "windows":
				env.LocalAppData, env.TempDir = winLocal, winTemp
			default:
				env.TempDir = "/tmp"
			}
			cmd, args, err := ServerCommand(tc.exe, env)
			if tc.wantErr {
				if !errors.Is(err, ErrNotInstalled) || !strings.Contains(err.Error(), "Install sourcebolt first") {
					t.Fatalf("got %q %q err=%v, want ErrNotInstalled with install advice", cmd, args, err)
				}
				return
			}
			if err != nil || cmd != tc.wantCmd || !slices.Equal(args, tc.wantArgs) {
				t.Fatalf("got %q %q err=%v, want %q %q", cmd, args, err, tc.wantCmd, tc.wantArgs)
			}
		})
	}
}

// SelfPath follows symlinks, so a link into the npx cache counts as the
// cache, and a link from a stable place to a stable place gives the target.
func TestSelfPath(t *testing.T) {
	link := filepath.Join("/usr", "local", "bin", "sourcebolt")
	target := filepath.Join("/home", "u", ".npm", "_npx", "1", "node_modules", "@candidloom-hub", "sourcebolt-linux-x64", "bin", "sourcebolt")
	exe := func() (string, error) { return link, nil }
	resolve := func(p string) (string, error) {
		if p == link {
			return target, nil
		}
		return p, nil
	}
	got, err := SelfPath(exe, resolve)
	if err != nil || got != target {
		t.Fatalf("SelfPath = %q, %v; want %q", got, err, target)
	}

	// A path that cannot be resolved is used as is.
	got, err = SelfPath(exe, func(string) (string, error) { return "", errors.New("broken link") })
	if err != nil || got != link {
		t.Fatalf("unresolvable: SelfPath = %q, %v; want %q", got, err, link)
	}

	// No executable path at all is an error.
	if _, err := SelfPath(func() (string, error) { return "", errors.New("no exe") }, resolve); err == nil {
		t.Fatal("want error when the executable path is unknown")
	}
}
