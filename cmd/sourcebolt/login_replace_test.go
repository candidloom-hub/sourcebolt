package main

import (
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
