package safety

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrBlockedPath wraps every path rejection.
var ErrBlockedPath = errors.New("path not allowed")

func blockedPath(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrBlockedPath, fmt.Sprintf(format, a...))
}

var winDevices = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"CONIN$": true, "CONOUT$": true, "CLOCK$": true,
}

func isWinDevice(name string) bool {
	base := name
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	base = strings.ToUpper(strings.TrimRight(base, " "))
	if winDevices[base] {
		return true
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) {
		return base[3] >= '1' && base[3] <= '9'
	}
	return false
}

// checkWindowsSyntax rejects the raw string forms that Windows treats
// specially. It runs before filepath.Clean, which would hide some of them.
func checkWindowsSyntax(p string) error {
	norm := strings.ReplaceAll(p, "/", `\`)
	if strings.HasPrefix(norm, `\\`) {
		return blockedPath("device, extended-length or UNC path")
	}
	rest := norm
	if len(rest) >= 2 && rest[1] == ':' {
		rest = rest[2:]
	}
	if strings.Contains(rest, ":") {
		return blockedPath("alternate data stream or stray colon")
	}
	for _, c := range strings.Split(rest, `\`) {
		if c == "" || c == "." || c == ".." {
			continue
		}
		if isWinDevice(c) {
			return blockedPath("reserved device name %q", c)
		}
		if strings.HasSuffix(c, ".") || strings.HasSuffix(c, " ") {
			return blockedPath("component %q ends in dot or space", c)
		}
	}
	return nil
}

// resolveExisting evaluates symlinks on the longest existing prefix of p and
// re-appends the rest. A dangling symlink is rejected: writing through it
// would create a file wherever it points.
func resolveExisting(p string) (string, error) {
	cur, tail := p, ""
	for {
		if _, err := os.Lstat(cur); err == nil {
			r, err := filepath.EvalSymlinks(cur)
			if err != nil {
				return "", blockedPath("cannot resolve %q: %v", cur, err)
			}
			return filepath.Join(r, tail), nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", blockedPath("cannot stat %q: %v", cur, err)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", blockedPath("no existing ancestor of %q", p)
		}
		tail = filepath.Join(filepath.Base(cur), tail)
		cur = parent
	}
}

// rejectReparse fails closed on Windows junctions and other reparse points
// below root. filepath.EvalSymlinks does not follow them, so they could point
// anywhere. Go reports them as fs.ModeIrregular.
func rejectReparse(root, p string) error {
	if runtime.GOOS != "windows" || len(p) <= len(root) {
		return nil
	}
	cur := root
	for _, c := range strings.Split(strings.Trim(p[len(root):], `\`), `\`) {
		if c == "" {
			continue
		}
		cur = filepath.Join(cur, c)
		fi, err := os.Lstat(cur)
		if err != nil {
			return nil // remainder does not exist yet
		}
		if fi.Mode()&(fs.ModeIrregular|fs.ModeSymlink) != 0 {
			return blockedPath("%q is a junction or reparse point", cur)
		}
	}
	return nil
}

func within(root, p string) bool {
	if runtime.GOOS == "windows" {
		root, p = strings.ToLower(root), strings.ToLower(p)
	}
	if p == root {
		return true
	}
	if !strings.HasSuffix(root, string(filepath.Separator)) {
		root += string(filepath.Separator)
	}
	return strings.HasPrefix(p, root)
}

// CheckPath returns the cleaned, symlink-resolved absolute form of p when it
// lies inside one of roots. It rejects relative paths, any ".." element,
// symlinks that leave the roots and, on Windows, device paths, UNC paths,
// reserved device names and alternate data streams. p need not exist yet.
func CheckPath(p string, roots []string) (string, error) {
	if p == "" || strings.ContainsRune(p, 0) {
		return "", blockedPath("empty path or NUL byte")
	}
	if runtime.GOOS == "windows" {
		if err := checkWindowsSyntax(p); err != nil {
			return "", err
		}
	}
	for _, c := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == filepath.Separator }) {
		if c == ".." {
			return "", blockedPath("parent-directory element")
		}
	}
	if !filepath.IsAbs(p) {
		return "", blockedPath("path must be absolute")
	}
	resolved, err := resolveExisting(filepath.Clean(p))
	if err != nil {
		return "", err
	}
	for _, r := range roots {
		if r == "" {
			continue
		}
		abs, err := filepath.Abs(r)
		if err != nil {
			continue
		}
		rr, err := resolveExisting(abs)
		if err != nil {
			continue
		}
		if within(rr, resolved) {
			if err := rejectReparse(rr, resolved); err != nil {
				return "", err
			}
			return resolved, nil
		}
	}
	return "", blockedPath("outside allowed roots")
}
