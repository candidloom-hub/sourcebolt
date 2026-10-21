package session

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

// Every test here uses temp folders and the in-memory keyring; none reads
// the real home, %APPDATA% or Credential Manager.

func noLookup(t *testing.T, what string) func() (string, error) {
	return func() (string, error) {
		t.Helper()
		t.Errorf("%s must not be looked up", what)
		return "", errors.New("not allowed")
	}
}

func TestDirEnvWindowsDefaultIsHomeDotSourcebolt(t *testing.T) {
	home := filepath.Join("C:", "Users", "u")
	appdata := filepath.Join(home, "AppData", "Roaming")
	e := DirEnv{GOOS: "windows",
		Home:   func() (string, error) { return home, nil },
		Config: func() (string, error) { return appdata, nil }}
	got, err := e.ConfigDir()
	if err != nil || got != filepath.Join(home, ".sourcebolt") {
		t.Fatalf("ConfigDir = %q, %v; want %q", got, err, filepath.Join(home, ".sourcebolt"))
	}
	if old := e.LegacyConfigDir(); old != filepath.Join(appdata, "sourcebolt") {
		t.Fatalf("LegacyConfigDir = %q", old)
	}
}

func TestDirEnvOtherSystemsUnchanged(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "freebsd"} {
		base := filepath.Join("cfg", goos)
		e := DirEnv{GOOS: goos, Home: noLookup(t, "home"),
			Config: func() (string, error) { return base, nil }}
		got, err := e.ConfigDir()
		if err != nil || got != filepath.Join(base, "sourcebolt") {
			t.Fatalf("%s: ConfigDir = %q, %v", goos, got, err)
		}
		if old := e.LegacyConfigDir(); old != "" {
			t.Fatalf("%s: LegacyConfigDir = %q, want none", goos, old)
		}
	}
}

// wantUser is the documented keyring user name, computed independently.
func wantUser(t *testing.T, dir, name string) string {
	t.Helper()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		abs = r
	}
	abs = filepath.Clean(abs)
	if runtime.GOOS == "windows" {
		abs = strings.ToLower(abs)
	}
	sum := sha256.Sum256([]byte(abs))
	return name + "@" + hex.EncodeToString(sum[:])[:12]
}

func randKey(t *testing.T) []byte {
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

// writeProfileBlob writes a profile encrypted with key, as any version would.
func writeProfileBlob(t *testing.T, dir, name string, key []byte, sid string) {
	t.Helper()
	dp := diskProfile{Host: HostDefault, Cookies: jarOf(
		ck("SID", sid, ".google.com"),
		ck("__Secure-1PSIDTS", "ts", ".google.com"),
		ck("OSID", "osid", "notebook.google.com"),
	).All(), Rev: 1}
	blob, err := encrypt(key, dp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+profileExt), blob, 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeLegacy writes a profile the way v0.3.1 and older did: key under user = name.
// lockedKeyring serializes calls to a keyring that is not safe for concurrent use.
type lockedKeyring struct {
	mu    sync.Mutex
	inner Keyring
}

func (k *lockedKeyring) Get(s, u string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.inner.Get(s, u)
}

func (k *lockedKeyring) Set(s, u, v string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.inner.Set(s, u, v)
}

func (k *lockedKeyring) Delete(s, u string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.inner.Delete(s, u)
}

func writeLegacy(t *testing.T, dir, name string, key []byte, sid string) {
	t.Helper()
	writeProfileBlob(t, dir, name, key, sid)
	if err := keyring.Set(keyringService, name, base64.StdEncoding.EncodeToString(key)); err != nil {
		t.Fatal(err)
	}
}

func sidOf(t *testing.T, s Store, name string) string {
	t.Helper()
	p, err := s.Load(name)
	if err != nil {
		t.Fatalf("Load(%q): %v", name, err)
	}
	for _, c := range p.Jar.All() {
		if c.Name == "SID" {
			return c.Value
		}
	}
	return ""
}

func TestKeyringUserIsScopedToTheFolder(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	s := NewFileStore(dir, OSKeyring{})
	if err := s.Save(testProfile()); err != nil {
		t.Fatal(err)
	}
	if _, err := keyring.Get(keyringService, wantUser(t, dir, "work")); err != nil {
		t.Fatalf("no key under the scoped name %q: %v", wantUser(t, dir, "work"), err)
	}
	if _, err := keyring.Get(keyringService, "work"); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("a new key must not use the legacy name; Get = %v", err)
	}
}

// The original bug: two folders, one profile name, one shared keyring
// entry. A logout and a new login in folder B replaced the key that folder
// A's profile was encrypted with.
func TestTwoFoldersSameProfileNameKeepSeparateKeys(t *testing.T) {
	keyring.MockInit()
	a, b := NewFileStore(t.TempDir(), OSKeyring{}), NewFileStore(t.TempDir(), OSKeyring{})
	if err := a.Save(testProfile()); err != nil {
		t.Fatal(err)
	}
	if err := b.Save(testProfile()); err != nil {
		t.Fatal(err)
	}
	if err := b.Delete("work"); err != nil {
		t.Fatal(err)
	}
	if err := b.Save(testProfile()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Load("work"); err != nil {
		t.Fatalf("folder A after a save in folder B: %v", err)
	}
	if _, err := b.Load("work"); err != nil {
		t.Fatalf("folder B: %v", err)
	}
}

func TestLegacyKeyIsReadAndRestoredScopedOnSave(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	key := randKey(t)
	writeLegacy(t, dir, "work", key, "old-sid")
	s := NewFileStore(dir, OSKeyring{})
	p, err := s.Load("work")
	if err != nil {
		t.Fatalf("a legacy profile must still load: %v", err)
	}
	if err := s.Save(p); err != nil {
		t.Fatal(err)
	}
	v, err := keyring.Get(keyringService, wantUser(t, dir, "work"))
	if err != nil || v != base64.StdEncoding.EncodeToString(key) {
		t.Fatalf("save must re-store the legacy key under the scoped name: %q, %v", v, err)
	}
	if v, err := keyring.Get(keyringService, "work"); err != nil || v != base64.StdEncoding.EncodeToString(key) {
		t.Fatalf("the legacy entry must be left alone (another folder may use it): %v", err)
	}
	if got := sidOf(t, s, "work"); got != "old-sid" {
		t.Fatalf("SID = %q", got)
	}
}

func TestLegacyKeyOfAnotherFolderIsNotOurs(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	writeProfileBlob(t, dir, "work", randKey(t), "mine")
	other := base64.StdEncoding.EncodeToString(randKey(t))
	if err := keyring.Set(keyringService, "work", other); err != nil {
		t.Fatal(err)
	}
	s := NewFileStore(dir, OSKeyring{})
	if _, err := s.Load("work"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a legacy key that does not decrypt must count as missing; Load = %v", err)
	}
	// A new profile in another folder must not take or replace that entry.
	b := NewFileStore(t.TempDir(), OSKeyring{})
	if err := b.Save(testProfile()); err != nil {
		t.Fatal(err)
	}
	if v, _ := keyring.Get(keyringService, "work"); v != other {
		t.Fatal("a save in another folder replaced the legacy keyring entry")
	}
}

func TestDeleteRemovesScopedAndOnlyOurLegacyEntry(t *testing.T) {
	keyring.MockInit()
	// Folder A holds a legacy profile (legacy entry); folder B a new one.
	dirA, dirB := t.TempDir(), t.TempDir()
	writeLegacy(t, dirA, "work", randKey(t), "a")
	b := NewFileStore(dirB, OSKeyring{})
	if err := b.Save(testProfile()); err != nil {
		t.Fatal(err)
	}
	if err := b.Delete("work"); err != nil {
		t.Fatal(err)
	}
	if _, err := keyring.Get(keyringService, wantUser(t, dirB, "work")); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("Delete must remove the scoped entry: %v", err)
	}
	if _, err := keyring.Get(keyringService, "work"); err != nil {
		t.Fatalf("Delete in B removed A's legacy entry: %v", err)
	}
	a := NewFileStore(dirA, OSKeyring{})
	if got := sidOf(t, a, "work"); got != "a" {
		t.Fatalf("A after B's delete: SID %q", got)
	}
	if err := a.Delete("work"); err != nil {
		t.Fatal(err)
	}
	if _, err := keyring.Get(keyringService, "work"); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("Delete must remove a legacy entry that is ours: %v", err)
	}
}

// snapshot reads every file in dir, so a test can prove nothing changed.
func snapshot(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	m := map[string][]byte{}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			t.Fatal(rerr)
		}
		m[p] = b
		return nil
	})
	return m
}

func sameSnapshot(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if !bytes.Equal(v, b[k]) {
			return false
		}
	}
	return true
}

func TestMigrateCopiesADecryptableProfileAndLeavesTheOldFolder(t *testing.T) {
	keyring.MockInit()
	old, nw := t.TempDir(), filepath.Join(t.TempDir(), "profiles")
	key := randKey(t)
	writeLegacy(t, old, "default", key, "moved")
	before := snapshot(t, old)
	got, err := MigrateProfiles(old, nw, OSKeyring{})
	if err != nil || !slices.Equal(got, []string{"default"}) {
		t.Fatalf("MigrateProfiles = %v, %v", got, err)
	}
	if s := sidOf(t, NewFileStore(nw, OSKeyring{}), "default"); s != "moved" {
		t.Fatalf("SID = %q", s)
	}
	if !sameSnapshot(before, snapshot(t, old)) {
		t.Fatal("migration changed the old folder")
	}
	if v, _ := keyring.Get(keyringService, "default"); v != base64.StdEncoding.EncodeToString(key) {
		t.Fatal("migration changed the legacy keyring entry")
	}
	if _, err := NewFileStore(old, OSKeyring{}).Load("default"); err != nil {
		t.Fatalf("old profile must still load: %v", err)
	}
}

func TestMigrateCopiesAKeyFileProfile(t *testing.T) {
	keyring.MockInit()
	old, nw := t.TempDir(), t.TempDir()
	key := randKey(t)
	writeProfileBlob(t, old, "work", key, "filekey")
	if err := os.WriteFile(filepath.Join(old, "work"+keyExt), []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := MigrateProfiles(old, nw, OSKeyring{}); err != nil || len(got) != 1 {
		t.Fatalf("MigrateProfiles = %v, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(nw, "work"+keyExt)); err != nil {
		t.Fatalf("key file not copied: %v", err)
	}
	if s := sidOf(t, NewFileStore(nw, OSKeyring{}), "work"); s != "filekey" {
		t.Fatalf("SID = %q", s)
	}
}

func TestMigrateSkipsAnUndecryptableProfile(t *testing.T) {
	keyring.MockInit()
	old, nw := t.TempDir(), t.TempDir()
	writeProfileBlob(t, old, "default", randKey(t), "stale")
	keyring.Set(keyringService, "default", base64.StdEncoding.EncodeToString(randKey(t)))
	got, err := MigrateProfiles(old, nw, OSKeyring{})
	if err != nil || len(got) != 0 {
		t.Fatalf("MigrateProfiles = %v, %v; want nothing copied", got, err)
	}
	if _, err := os.Stat(filepath.Join(nw, "default"+profileExt)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("an undecryptable profile was copied: %v", err)
	}
}

func TestMigrateNeverOverwritesANewProfile(t *testing.T) {
	keyring.MockInit()
	old, nw := t.TempDir(), t.TempDir()
	writeLegacy(t, old, "work", randKey(t), "old")
	ns := NewFileStore(nw, OSKeyring{})
	if err := ns.Save(testProfile()); err != nil {
		t.Fatal(err)
	}
	if got, err := MigrateProfiles(old, nw, OSKeyring{}); err != nil || len(got) != 0 {
		t.Fatalf("MigrateProfiles = %v, %v", got, err)
	}
	if s := sidOf(t, ns, "work"); s != "secret-sid-value" {
		t.Fatalf("new profile was overwritten: SID %q", s)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	keyring.MockInit()
	old, nw := t.TempDir(), t.TempDir()
	writeLegacy(t, old, "default", randKey(t), "once")
	if got, err := MigrateProfiles(old, nw, OSKeyring{}); err != nil || len(got) != 1 {
		t.Fatalf("first run = %v, %v", got, err)
	}
	after := snapshot(t, nw)
	if got, err := MigrateProfiles(old, nw, OSKeyring{}); err != nil || len(got) != 0 {
		t.Fatalf("second run = %v, %v; want nothing", got, err)
	}
	if !sameSnapshot(after, snapshot(t, nw)) {
		t.Fatal("second run changed the new folder")
	}
}

func TestMigrateWithoutAnOldFolderIsANoOp(t *testing.T) {
	keyring.MockInit()
	nw := filepath.Join(t.TempDir(), "profiles")
	got, err := MigrateProfiles(filepath.Join(t.TempDir(), "missing"), nw, OSKeyring{})
	if err != nil || len(got) != 0 {
		t.Fatalf("MigrateProfiles = %v, %v", got, err)
	}
	if _, err := os.Stat(nw); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("a no-op migration created the new folder")
	}
}

// countingKeyring counts Get calls before handing them to Inner.
type countingKeyring struct {
	Keyring
	gets *atomic.Int32
}

func (c countingKeyring) Get(s, u string) (string, error) { c.gets.Add(1); return c.Keyring.Get(s, u) }

// After a keyring timeout the legacy lookup is skipped: a hung macOS
// keychain must not double the wait before the key file is used.
func TestLegacyLookupSkippedAfterTimeout(t *testing.T) {
	b := blockingKeyring{release: make(chan struct{})}
	defer close(b.release)
	gets := &atomic.Int32{}
	kr := TimeoutKeyring{Inner: countingKeyring{Keyring: b, gets: gets}, Timeout: 50 * time.Millisecond}
	s := NewFileStore(t.TempDir(), kr)
	if err := s.Save(testProfile()); err != nil {
		t.Fatal(err)
	}
	gets.Store(0)
	if _, err := s.Load("work"); err != nil {
		t.Fatal(err)
	}
	if n := gets.Load(); n != 1 {
		t.Fatalf("Load made %d keyring lookups after a timeout, want 1", n)
	}
}

// Several processes starting at once copy each profile exactly once.
func TestMigrateConcurrentFirstRuns(t *testing.T) {
	keyring.MockInit()
	old, nw := t.TempDir(), t.TempDir()
	writeLegacy(t, old, "default", randKey(t), "race")
	var wg sync.WaitGroup
	var total atomic.Int32
	// go-keyring's mock is a plain map with no lock; the real OS keyrings are
	// safe for concurrent use. Serialize calls so -race tests our code, not the mock.
	kr := &lockedKeyring{inner: OSKeyring{}}
	for range 8 {
		wg.Go(func() {
			got, _ := MigrateProfiles(old, nw, kr)
			total.Add(int32(len(got)))
		})
	}
	wg.Wait()
	if n := total.Load(); n != 1 {
		t.Fatalf("profile copied %d times, want 1", n)
	}
	if s := sidOf(t, NewFileStore(nw, OSKeyring{}), "default"); s != "race" {
		t.Fatalf("SID = %q", s)
	}
}

// A key file that does not decrypt the old profile is not a reason to copy it.
func TestMigrateSkipsAProfileWhoseKeyFileIsWrong(t *testing.T) {
	keyring.MockInit()
	old, nw := t.TempDir(), t.TempDir()
	writeProfileBlob(t, old, "work", randKey(t), "x")
	if err := os.WriteFile(filepath.Join(old, "work"+keyExt), []byte(base64.StdEncoding.EncodeToString(randKey(t))), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := MigrateProfiles(old, nw, OSKeyring{}); err != nil || len(got) != 0 {
		t.Fatalf("MigrateProfiles = %v, %v; want nothing", got, err)
	}
	if _, err := os.Stat(filepath.Join(nw, "work"+profileExt)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("a profile with a wrong key file was copied")
	}
}

// Once the new folder has the profile, a start does no keyring lookups
// and no decryption for it.
func TestMigrateIsCheapWhenAlreadyDone(t *testing.T) {
	keyring.MockInit()
	old, nw := t.TempDir(), t.TempDir()
	writeLegacy(t, old, "default", randKey(t), "done")
	if _, err := MigrateProfiles(old, nw, OSKeyring{}); err != nil {
		t.Fatal(err)
	}
	gets := &atomic.Int32{}
	if _, err := MigrateProfiles(old, nw, countingKeyring{Keyring: OSKeyring{}, gets: gets}); err != nil {
		t.Fatal(err)
	}
	if n := gets.Load(); n != 0 {
		t.Fatalf("an already migrated profile cost %d keyring lookups", n)
	}
}
