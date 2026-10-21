package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/zalando/go-keyring"
)

// A profile whose key is gone from the keyring can never be read again.
// Login replaces it (the user is signing in afresh) instead of failing with
// "default.key: file not found" and leaving the user stuck.
func TestPrepareSaveReplacesUnreadableProfile(t *testing.T) {
	keyring.MockInit()
	store := session.NewFileStore(t.TempDir(), session.OSKeyring{})
	if err := store.Save(&session.Profile{Name: "default", Jar: session.NewJar()}); err != nil {
		t.Fatal(err)
	}
	if err := keyring.DeleteAll("sourcebolt"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("default"); err == nil {
		t.Fatal("setup: profile still readable")
	}
	rev, replaced, err := prepareSave(store, "default")
	if err != nil || !replaced || rev != 0 {
		t.Fatalf("rev = %d, replaced = %v, err = %v", rev, replaced, err)
	}
	if err := store.Save(&session.Profile{Name: "default", Jar: session.NewJar()}); err != nil {
		t.Fatalf("save after replace: %v", err)
	}
}

func TestPrepareSaveKeepsReadableRevision(t *testing.T) {
	keyring.MockInit()
	store := session.NewFileStore(t.TempDir(), session.OSKeyring{})
	if err := store.Save(&session.Profile{Name: "default", Jar: session.NewJar()}); err != nil {
		t.Fatal(err)
	}
	old, _ := store.Load("default")
	rev, replaced, err := prepareSave(store, "default")
	if err != nil || replaced || rev != old.Rev {
		t.Fatalf("rev = %d (want %d), replaced = %v, err = %v", rev, old.Rev, replaced, err)
	}
	if rev2, replaced2, err := prepareSave(store, "fresh"); err != nil || replaced2 || rev2 != 0 {
		t.Fatalf("missing profile: rev = %d, replaced = %v, err = %v", rev2, replaced2, err)
	}
}

// A profile file that the current key cannot decrypt (its key was replaced)
// or that is damaged can never be read either: login replaces it the same
// way as a missing key.
func TestPrepareSaveReplacesUndecryptableProfile(t *testing.T) {
	for _, tc := range []struct {
		name   string
		break_ func(t *testing.T, store session.Store, path string)
	}{
		{"wrong key", func(t *testing.T, store session.Store, path string) {
			blobA, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Delete("default"); err != nil {
				t.Fatal(err)
			}
			if err := store.Save(&session.Profile{Name: "default", Jar: session.NewJar()}); err != nil {
				t.Fatal(err) // mints key B
			}
			if err := os.WriteFile(path, blobA, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"corrupt file", func(t *testing.T, _ session.Store, path string) {
			if err := os.WriteFile(path, []byte("not a profile"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			keyring.MockInit()
			dir := t.TempDir()
			store := session.NewFileStore(dir, session.OSKeyring{})
			if err := store.Save(&session.Profile{Name: "default", Jar: session.NewJar()}); err != nil {
				t.Fatal(err)
			}
			tc.break_(t, store, filepath.Join(dir, "default.profile"))
			if _, err := store.Load("default"); err == nil {
				t.Fatal("setup: profile still readable")
			}
			rev, replaced, err := prepareSave(store, "default")
			if err != nil || !replaced || rev != 0 {
				t.Fatalf("rev = %d, replaced = %v, err = %v", rev, replaced, err)
			}
			if err := store.Save(&session.Profile{Name: "default", Jar: session.NewJar()}); err != nil {
				t.Fatalf("save after replace: %v", err)
			}
			if _, err := store.Load("default"); err != nil {
				t.Fatalf("load after replace: %v", err)
			}
		})
	}
}

// lockedKeyring fails every call the way a locked keychain does.
type lockedKeyring struct{}

func (lockedKeyring) Get(string, string) (string, error) { return "", errors.New("keychain is locked") }
func (lockedKeyring) Set(string, string, string) error   { return errors.New("keychain is locked") }
func (lockedKeyring) Delete(string, string) error        { return errors.New("keychain is locked") }

// A keyring that cannot answer right now still holds the key: login must
// not delete the profile, only report it.
func TestPrepareSaveKeepsProfileWhenKeyringFails(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	if err := session.NewFileStore(dir, session.OSKeyring{}).Save(&session.Profile{Name: "default", Jar: session.NewJar()}); err != nil {
		t.Fatal(err)
	}
	store := session.NewFileStore(dir, lockedKeyring{})
	_, replaced, err := prepareSave(store, "default")
	if err == nil || replaced || !strings.Contains(err.Error(), "cannot be read") {
		t.Fatalf("replaced = %v, err = %v", replaced, err)
	}
	if _, serr := os.Stat(filepath.Join(dir, "default.profile")); serr != nil {
		t.Fatalf("profile file removed: %v", serr)
	}
}
