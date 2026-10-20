package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// ErrNoBackup means setup never changed this file (or every change was
// already undone).
var ErrNoBackup = errors.New("no sourcebolt backup found")

const (
	backupInfix  = ".sourcebolt-backup-"
	absentSuffix = ".absent" // marks "the file did not exist before setup"
)

// Backup copies path to <path>.sourcebolt-backup-<timestamp> before setup
// changes it. When path does not exist yet, it writes an empty
// <...>.absent marker so undo can remove the file again.
func Backup(path string, now time.Time) (string, error) {
	data, err := os.ReadFile(path)
	absent := errors.Is(err, os.ErrNotExist)
	if err != nil && !absent {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	base := path + backupInfix + now.Format("20060102-150405")
	for i := 0; ; i++ {
		name := base
		if i > 0 {
			name = fmt.Sprintf("%s-%02d", base, i)
		}
		if absent {
			name += absentSuffix
		}
		if _, err := os.Lstat(strings.TrimSuffix(name, absentSuffix)); err == nil {
			continue
		}
		if _, err := os.Lstat(strings.TrimSuffix(name, absentSuffix) + absentSuffix); err == nil {
			continue
		}
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, werr := f.Write(data)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			os.Remove(name)
			return "", werr
		}
		return name, nil
	}
}

// LatestBackup returns the newest backup of path.
func LatestBackup(path string) (string, error) {
	entries, err := os.ReadDir(filepath.Dir(path))
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNoBackup
	}
	if err != nil {
		return "", err
	}
	prefix := filepath.Base(path) + backupInfix
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return "", ErrNoBackup
	}
	// The stamp sorts by time; ".absent" must not change the order.
	slices.SortFunc(names, func(a, b string) int {
		return strings.Compare(strings.TrimSuffix(a, absentSuffix), strings.TrimSuffix(b, absentSuffix))
	})
	return filepath.Join(filepath.Dir(path), names[len(names)-1]), nil
}

// Restore puts the newest backup back in place of path (or removes path when
// the backup records that it did not exist) and then deletes that backup,
// so a second undo goes one step further back.
func Restore(path string) (string, error) {
	from, err := LatestBackup(path)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(from, absentSuffix) {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	} else {
		data, err := os.ReadFile(from)
		if err != nil {
			return "", err
		}
		if err := writeFile(path, data); err != nil {
			return "", err
		}
	}
	return from, os.Remove(from)
}

// RestoreEntry undoes only the entry name under key: it puts back the entry
// the newest backup had, or removes it when the backup had none, and keeps
// the rest of the current file. It is for files the app keeps rewriting
// (Claude Code's ~/.claude.json), where restoring the whole file would lose
// the app's newer state. The used backup is deleted.
func RestoreEntry(path, key, name string) (string, error) {
	from, err := LatestBackup(path)
	if err != nil {
		return "", err
	}
	var old []byte
	if !strings.HasSuffix(from, absentSuffix) {
		if old, err = os.ReadFile(from); err != nil {
			return "", err
		}
	}
	cur, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	e, had, err := LookupRaw(old, key, name)
	if err != nil {
		return "", err
	}
	var out []byte
	if had {
		out, _, err = Merge(cur, key, name, e)
	} else {
		out, err = Remove(cur, key, name)
	}
	if err != nil {
		return "", err
	}
	if err := writeFile(path, out); err != nil {
		return "", err
	}
	return from, os.Remove(from)
}

// writeFile replaces path through a temp file in the same dir, so a crash
// never leaves a half-written config. It keeps the old file's mode.
func writeFile(path string, data []byte) error {
	mode := os.FileMode(0o600)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".sourcebolt-tmp-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmp.Name(), mode)
	}
	if werr == nil {
		werr = os.Rename(tmp.Name(), path)
	}
	if werr != nil {
		os.Remove(tmp.Name())
	}
	return werr
}

// WriteFile is writeFile for callers outside the package.
func WriteFile(path string, data []byte) error { return writeFile(path, data) }
