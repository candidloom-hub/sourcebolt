package session

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func newTestStore(t *testing.T) (Store, string) {
	t.Helper()
	keyring.MockInit()
	dir := t.TempDir()
	return NewFileStore(dir, OSKeyring{}), dir
}

func testProfile() *Profile {
	return &Profile{Name: "work", Host: HostDefault, Jar: jarOf(
		ck("SID", "secret-sid-value", ".google.com"),
		ck("__Secure-1PSIDTS", "secret-ts-value", ".google.com"),
		ck("OSID", "secret-osid-value", "notebook.google.com"),
	)}
}

func TestStoreRoundTrip(t *testing.T) {
	s, dir := newTestStore(t)
	if err := s.Save(testProfile()); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("work")
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != HostDefault || got.Jar.HeaderFor(HostDefault) != testProfile().Jar.HeaderFor(HostDefault) {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*"))
	for _, f := range files {
		b, _ := os.ReadFile(f)
		if bytes.Contains(b, []byte("secret-")) {
			t.Fatalf("%s contains a plaintext cookie value", f)
		}
	}
	names, _ := s.List()
	if len(names) != 1 || names[0] != "work" {
		t.Fatalf("List = %v", names)
	}
}

func TestStoreRevision(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Save(testProfile()); err != nil {
		t.Fatal(err)
	}
	a, _ := s.Load("work")
	b, _ := s.Load("work")
	if err := s.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(b); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("second writer: got %v, want ErrStaleRevision", err)
	}
}

func TestStoreAtomic(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Save(testProfile()); err != nil {
		t.Fatal(err)
	}
	p, _ := s.Load("work")
	p.Host = "notebooklm.google.com"
	beforeRename = func() error { return errors.New("injected") }
	t.Cleanup(func() { beforeRename = nil })
	if err := s.Save(p); err == nil {
		t.Fatal("want injected error")
	}
	beforeRename = nil
	got, err := s.Load("work")
	if err != nil || got.Host != HostDefault {
		t.Fatalf("original must survive a failed save: %+v %v", got, err)
	}
}

func TestStoreKeyFileFallback(t *testing.T) {
	keyring.MockInitWithError(errors.New("no keyring"))
	dir := t.TempDir()
	s := NewFileStore(dir, OSKeyring{})
	if err := s.Save(testProfile()); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("work")
	if err != nil || !got.KeyInFile {
		t.Fatalf("want key-file profile, got %+v %v", got, err)
	}
	if err := s.Delete("work"); err != nil {
		t.Fatal(err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*")); len(left) != 0 {
		t.Fatalf("Delete left files: %v", left)
	}
}
