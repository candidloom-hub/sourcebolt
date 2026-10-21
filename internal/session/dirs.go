package session

import (
	"encoding/base64"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DirEnv resolves the default config folders. The lookups are functions so
// tests can point them at temp folders and never read the real home or
// %APPDATA%.
type DirEnv struct {
	GOOS   string
	Home   func() (string, error) // os.UserHomeDir
	Config func() (string, error) // os.UserConfigDir
}

// ConfigDir is the default sourcebolt config folder: %USERPROFILE%\.sourcebolt
// on Windows, <user config dir>/sourcebolt elsewhere.
//
// Windows moved out of %APPDATA% because a process started by a packaged
// (MSIX) app, such as Claude Desktop from the Microsoft Store, gets AppData
// file-system virtualization: its writes to %APPDATA% land in a private
// copy, so two apps on one PC saw two different profile files. The user
// profile folder outside AppData is not virtualized.
func (e DirEnv) ConfigDir() (string, error) {
	if e.GOOS == "windows" {
		home, err := e.Home()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".sourcebolt"), nil
	}
	base, err := e.Config()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "sourcebolt"), nil
}

// LegacyConfigDir is the folder versions up to 0.3.1 used on Windows
// (%APPDATA%\sourcebolt), or "" when there is none to migrate from.
func (e DirEnv) LegacyConfigDir() string {
	if e.GOOS != "windows" {
		return ""
	}
	base, err := e.Config()
	if err != nil || base == "" {
		return ""
	}
	return filepath.Join(base, "sourcebolt")
}

// OSDirEnv is the real environment.
func OSDirEnv() DirEnv {
	return DirEnv{GOOS: runtime.GOOS, Home: os.UserHomeDir, Config: os.UserConfigDir}
}

// DefaultDir is <default config dir>/profiles.
func DefaultDir() (string, error) {
	d, err := OSDirEnv().ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "profiles"), nil
}

// MigrateProfiles copies profiles from an old profiles folder into newDir.
// A profile is copied only when newDir has none of that name and the old
// one decrypts with its own key; the key goes along (key file, or keyring
// entry under newDir's scoped name). The old folder and its keyring entries
// are never changed, so nothing is lost. A profile that does not decrypt
// (it belongs to another view of the folder, or its key is gone) is
// skipped. It is cheap when there is nothing to do and safe to run on every
// start; it returns the names it copied.
func MigrateProfiles(oldDir, newDir string, kr Keyring) ([]string, error) {
	if samePath(oldDir, newDir) {
		return nil, nil
	}
	old := &fileStore{dir: oldDir, kr: kr}
	names, err := old.List()
	if err != nil || len(names) == 0 {
		return nil, err
	}
	nw := &fileStore{dir: newDir, kr: kr}
	var copied []string
	var errs []error
	for _, name := range names {
		if pathName(name) != nil || exists(nw.path(name, profileExt)) {
			continue
		}
		ok, err := migrateOne(old, nw, name)
		if err != nil {
			errs = append(errs, err)
		}
		if ok {
			copied = append(copied, name)
		}
	}
	return copied, errors.Join(errs...)
}

func migrateOne(old, nw *fileStore, name string) (bool, error) {
	blob, err := os.ReadFile(old.path(name, profileExt))
	if err != nil {
		return false, nil
	}
	key, inFile, _, err := old.getKey(name, blob)
	if err != nil {
		return false, nil // no usable key: not ours to move
	}
	if _, err := decrypt(key, blob); err != nil {
		return false, nil
	}
	if err := os.MkdirAll(nw.dir, 0o700); err != nil {
		return false, err
	}
	unlock, err := nw.lock(name)
	if err != nil {
		return false, err
	}
	defer unlock()
	if exists(nw.path(name, profileExt)) {
		return false, nil // another process got here first
	}
	enc := base64.StdEncoding.EncodeToString(key)
	if inFile || nw.kr.Set(keyringService, nw.keyUser(name), enc) != nil {
		if err := writeAtomic(nw.path(name, keyExt), []byte(enc)); err != nil {
			return false, err
		}
	}
	// The profile goes last: until it is in place the copy does not exist,
	// so a crash part way leaves nothing half-migrated.
	if err := writeAtomic(nw.path(name, profileExt), blob); err != nil {
		return false, err
	}
	return true, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !errors.Is(err, fs.ErrNotExist)
}

func samePath(a, b string) bool {
	return strings.EqualFold(normDir(a), normDir(b))
}
