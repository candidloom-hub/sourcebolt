package rpcids

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLookupDefault(t *testing.T) {
	Reset()
	if id, ok := Lookup("notebooks.list"); !ok || id != "wXbhsf" {
		t.Fatalf("Lookup = %q %v", id, ok)
	}
	if _, ok := Lookup("nope.nothing"); ok {
		t.Fatal("unknown name must not resolve")
	}
}

func TestOverride(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	p := filepath.Join(t.TempDir(), "rpc-ids.json")
	os.WriteFile(p, []byte(`{"notebooks.list":"zzzzzz"}`), 0o600)
	if err := LoadOverrides(p); err != nil {
		t.Fatal(err)
	}
	if id, _ := Lookup("notebooks.list"); id != "zzzzzz" {
		t.Fatalf("override not applied: %q", id)
	}
}

func TestOverrideUnknownName(t *testing.T) {
	Reset()
	p := filepath.Join(t.TempDir(), "rpc-ids.json")
	os.WriteFile(p, []byte(`{"notebooks.lsit":"x"}`), 0o600)
	err := LoadOverrides(p)
	if err == nil || !strings.Contains(err.Error(), "notebooks.lsit") {
		t.Fatalf("want error naming the key, got %v", err)
	}
}

func TestNoStaleIDs(t *testing.T) {
	Reset()
	stale := strings.Fields("qXyaNe AHyHrd VUsiyb sJDbic xpWGLf WxBZtb BnLyuf Z8UXi zVGIdd Y0vGub ZAnZ8")
	for name, id := range All() {
		for _, s := range stale {
			if id == s {
				t.Errorf("%s uses stale id %s", name, s)
			}
		}
	}
}
