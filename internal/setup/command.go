package setup

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrNotInstalled means sourcebolt runs from a throwaway place (a temp
// folder or the npx cache) and neither an installed copy nor npx exists, so
// there is no command an app could keep starting.
var ErrNotInstalled = errors.New("sourcebolt is not installed")

// npxArgs start the published npm package; npx fetches or reuses it.
var npxArgs = []string{"-y", "sourcebolt", "serve"}

// SelfPath returns this program's path with symlinks followed, so a link
// into the npx cache is seen as the cache. exe is os.Executable and resolve is
// filepath.EvalSymlinks; a path that cannot be resolved is used as is.
func SelfPath(exe func() (string, error), resolve func(string) (string, error)) (string, error) {
	p, err := exe()
	if err != nil {
		return "", err
	}
	if r, err := resolve(p); err == nil {
		p = r
	}
	return p, nil
}

// normPath makes a path comparable across hosts: forward slashes, and
// case-folded where the file system ignores case.
func normPath(goos, p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	if goos == "windows" || goos == "darwin" {
		p = strings.ToLower(p)
	}
	return strings.TrimSuffix(p, "/")
}

// hasSegment reports whether a path element of p equals one of segs, or
// starts with one of prefixes. Matching ignores case.
func hasSegment(p string, segs, prefixes []string) bool {
	for _, el := range strings.Split(strings.ToLower(strings.ReplaceAll(p, `\`, "/")), "/") {
		for _, s := range segs {
			if el == s {
				return true
			}
		}
		for _, s := range prefixes {
			if strings.HasPrefix(el, s) {
				return true
			}
		}
	}
	return false
}

// npmManaged: npm and npx own the folder and may replace or delete it.
func npmManaged(p string) bool {
	return hasSegment(p, []string{"_npx", "npm-cache", "node_modules"}, nil)
}

// ephemeral: a path an app config must not point at, because it lives in
// the temp dir, a Go build dir or an npm-managed folder.
func (e Env) ephemeral(p string) bool {
	if npmManaged(p) || hasSegment(p, nil, []string{"go-build"}) {
		return true
	}
	return e.TempDir != "" && strings.HasPrefix(normPath(e.GOOS, p)+"/", normPath(e.GOOS, e.TempDir)+"/")
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

// installed finds a stable installed copy: an installer location first,
// then sourcebolt on PATH unless that copy is itself ephemeral.
func (e Env) installed() (string, bool) {
	for _, c := range e.installedPaths() {
		if e.Exists != nil && e.Exists(c) {
			return c, true
		}
	}
	if e.LookPath != nil {
		if p, err := e.LookPath("sourcebolt"); err == nil && !e.ephemeral(p) {
			return p, true
		}
	}
	return "", false
}

// ServerCommand picks the command and arguments an MCP app should run to
// start sourcebolt, given this process's executable (see SelfPath):
//
//   - a stable path is used as is;
//   - from a temp dir, Go build dir or npm/npx folder: an installed copy
//     (installer location, Homebrew, or PATH outside those places);
//   - otherwise "npx -y sourcebolt serve" when npx is on PATH;
//   - otherwise ErrNotInstalled.
//
// A versioned Homebrew path (Cellar, Caskroom) changes on every upgrade, so
// the bin link is preferred, but the path itself is kept when there is none.
func ServerCommand(exe string, env Env) (string, []string, error) {
	versioned := hasSegment(exe, []string{"caskroom", "cellar"}, nil)
	if !env.ephemeral(exe) && !versioned {
		return exe, []string{"serve"}, nil
	}
	if p, ok := env.installed(); ok {
		return p, []string{"serve"}, nil
	}
	if versioned {
		return exe, []string{"serve"}, nil
	}
	if env.LookPath != nil {
		if _, err := env.LookPath("npx"); err == nil {
			return "npx", append([]string(nil), npxArgs...), nil
		}
	}
	return "", nil, fmt.Errorf("%w: this copy runs from a temporary or npm-managed folder (%s), which apps cannot keep using, and npx was not found. Install sourcebolt first (see docs/getting-started.md, Step 1), then run setup again from the installed program", ErrNotInstalled, exe)
}
