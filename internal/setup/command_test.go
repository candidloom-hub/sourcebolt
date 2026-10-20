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
	npx := []string{"-y", "sourcebolt", "serve"}
	serve := []string{"serve"}

	tests := []struct {
		name     string
		goos     string
		exe      string
		existing []string
		brew     string // $HOMEBREW_PREFIX
		npmCmd   string // $npm_command
		wantCmd  string
		wantArgs []string
		wantErr  bool
	}{
		{name: "installed binary is used as is", goos: "linux", exe: unixStd, wantCmd: unixStd, wantArgs: serve},
		{name: "any stable path is used as is", goos: "linux", exe: "/usr/bin/sourcebolt", wantCmd: "/usr/bin/sourcebolt", wantArgs: serve},
		{name: "windows installed binary", goos: "windows", exe: winStd, wantCmd: winStd, wantArgs: serve},
		{name: "npx cache prefers the installed binary", goos: "windows",
			exe:      j(winLocal, "npm-cache", "_npx", "3f2a", "node_modules", "@sourcebolt", "win32-x64", "bin", "sourcebolt.exe"),
			existing: []string{winStd}, wantCmd: winStd, wantArgs: serve},
		{name: "npx cache without an install writes npx", goos: "windows",
			exe:     j(winLocal, "npm-cache", "_npx", "3f2a", "node_modules", "@sourcebolt", "win32-x64", "bin", "sourcebolt.exe"),
			wantCmd: "npx", wantArgs: npx},
		{name: "npx on unix without an install writes npx", goos: "linux",
			exe:     j("/home/u", ".npm", "_npx", "9c1d", "node_modules", "@sourcebolt", "linux-x64", "bin", "sourcebolt"),
			wantCmd: "npx", wantArgs: npx},
		{name: "global npm install writes npx", goos: "darwin",
			exe:     j("/usr/local/lib", "node_modules", "@sourcebolt", "darwin-arm64", "bin", "sourcebolt"),
			wantCmd: "npx", wantArgs: npx},
		{name: "temp dir prefers ~/.local/bin", goos: "linux", exe: j("/tmp", "go-build123", "b001", "exe", "sourcebolt"),
			existing: []string{unixStd}, wantCmd: unixStd, wantArgs: serve},
		{name: "temp dir under npm exec writes npx", goos: "linux", exe: j("/tmp", "x", "sourcebolt"), npmCmd: "exec",
			wantCmd: "npx", wantArgs: npx},
		{name: "temp dir without an install is refused", goos: "linux", exe: j("/tmp", "go-build123", "b001", "exe", "sourcebolt"), wantErr: true},
		{name: "windows temp dir, any case, is refused", goos: "windows", exe: strings.ToUpper(j(winTemp, "go-build1", "sourcebolt.exe")), wantErr: true},
		{name: "homebrew default prefix", goos: "darwin", exe: j("/tmp", "sourcebolt"),
			existing: []string{j("/opt/homebrew", "bin", "sourcebolt")}, wantCmd: j("/opt/homebrew", "bin", "sourcebolt"), wantArgs: serve},
		{name: "homebrew prefix from env", goos: "linux", exe: j("/tmp", "sourcebolt"), brew: "/hb",
			existing: []string{j("/hb", "bin", "sourcebolt")}, wantCmd: j("/hb", "bin", "sourcebolt"), wantArgs: serve},
		{name: "linuxbrew default prefix", goos: "linux", exe: j("/tmp", "sourcebolt"),
			existing: []string{j("/home/linuxbrew/.linuxbrew", "bin", "sourcebolt")}, wantCmd: j("/home/linuxbrew/.linuxbrew", "bin", "sourcebolt"), wantArgs: serve},
		{name: "homebrew versioned cask path prefers the bin link", goos: "darwin",
			exe:      j("/opt/homebrew", "Caskroom", "sourcebolt", "0.2.0", "sourcebolt"),
			existing: []string{j("/opt/homebrew", "bin", "sourcebolt")}, wantCmd: j("/opt/homebrew", "bin", "sourcebolt"), wantArgs: serve},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := fakeEnv(tc.goos, tc.existing)
			env.HomebrewPrefix, env.NPMCommand = tc.brew, tc.npmCmd
			switch tc.goos {
			case "windows":
				env.LocalAppData, env.TempDir = winLocal, winTemp
			default:
				env.TempDir = "/tmp"
			}
			cmd, args, err := ServerCommand(tc.exe, env)
			if tc.wantErr {
				if !errors.Is(err, ErrNotInstalled) || !strings.Contains(err.Error(), "install") {
					t.Fatalf("err = %v, want ErrNotInstalled with install advice", err)
				}
				return
			}
			if err != nil || cmd != tc.wantCmd || !slices.Equal(args, tc.wantArgs) {
				t.Fatalf("got %q %q err=%v, want %q %q", cmd, args, err, tc.wantCmd, tc.wantArgs)
			}
		})
	}
}
