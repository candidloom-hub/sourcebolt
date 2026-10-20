package setup

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrNotInstalled means sourcebolt runs from a throwaway place (a temp
// folder) and no installed copy exists, so there is no path an app could
// keep starting.
var ErrNotInstalled = errors.New("sourcebolt is not installed")

// npxArgs start the published npm package; npx fetches or reuses it.
var npxArgs = []string{"-y", "sourcebolt", "serve"}

// normPath makes a path comparable across hosts: forward slashes, and
// case-folded where the file system ignores case.
func normPath(goos, p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	if goos == "windows" || goos == "darwin" {
		p = strings.ToLower(p)
	}
	return strings.TrimSuffix(p, "/")
}

func hasSegment(p string, segs ...string) bool {
	lp := "/" + strings.ToLower(p) + "/"
	for _, s := range segs {
		if strings.Contains(lp, "/"+s+"/") {
			return true
		}
	}
	return false
}

// installedPaths are where the installers put the program: the Windows
// installer's %LOCALAPPDATA%\Programs\sourcebolt, ~/.local/bin from the
// Unix installer, and the Homebrew bin link.
func (e Env) installedPaths() []string {
	if e.GOOS == "windows" {
		if e.LocalAppData == "" {
			return nil
		}
		return []string{filepath.Join(e.LocalAppData, "Programs", "sourcebolt", "sourcebolt.exe")}
	}
	out := []string{filepath.Join(e.Home, ".local", "bin", "sourcebolt")}
	prefixes := []string{e.HomebrewPrefix}
	switch e.GOOS {
	case "darwin":
		prefixes = append(prefixes, "/opt/homebrew", "/usr/local")
	case "linux":
		prefixes = append(prefixes, "/home/linuxbrew/.linuxbrew")
	}
	for _, p := range prefixes {
		if p != "" {
			out = append(out, filepath.Join(p, "bin", "sourcebolt"))
		}
	}
	return out
}

// ServerCommand picks the command and arguments an MCP app should run to
// start sourcebolt, given this process's executable:
//
//   - a stable path (not a temp folder, npm/npx cache or versioned
//     Homebrew folder) is used as is;
//   - otherwise an installed copy from the installers or Homebrew;
//   - otherwise, when run through npm or npx, "npx -y sourcebolt serve";
//   - otherwise ErrNotInstalled.
func ServerCommand(exe string, env Env) (string, []string, error) {
	p := normPath(env.GOOS, exe)
	npm := hasSegment(p, "_npx", "node_modules")
	temp := env.TempDir != "" && strings.HasPrefix(p, normPath(env.GOOS, env.TempDir)+"/")
	versioned := hasSegment(p, "caskroom", "cellar") // changes on every brew upgrade
	if !npm && !temp && !versioned {
		return exe, []string{"serve"}, nil
	}
	for _, c := range env.installedPaths() {
		if env.Exists != nil && env.Exists(c) {
			return c, []string{"serve"}, nil
		}
	}
	if npm || env.NPMCommand == "exec" {
		return "npx", append([]string(nil), npxArgs...), nil
	}
	if versioned {
		return exe, []string{"serve"}, nil
	}
	return "", nil, fmt.Errorf("%w: this copy runs from a temporary folder (%s), which apps cannot keep using. Install sourcebolt first (see docs/install.md), then run setup again from the installed program", ErrNotInstalled, exe)
}
