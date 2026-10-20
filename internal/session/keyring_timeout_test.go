package session

import (
	"errors"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

// blockingKeyring never answers, like macOS `security` waiting on a locked
// or missing keychain with nobody to unlock it.
type blockingKeyring struct{ release chan struct{} }

func (b blockingKeyring) Get(string, string) (string, error) { <-b.release; return "", nil }
func (b blockingKeyring) Set(string, string, string) error   { <-b.release; return nil }
func (b blockingKeyring) Delete(string, string) error        { <-b.release; return nil }

func TestTimeoutKeyringGivesUp(t *testing.T) {
	b := blockingKeyring{release: make(chan struct{})}
	defer close(b.release)
	k := TimeoutKeyring{Inner: b, Timeout: 50 * time.Millisecond}
	for name, call := range map[string]func() error{
		"Get":    func() error { _, err := k.Get("s", "u"); return err },
		"Set":    func() error { return k.Set("s", "u", "v") },
		"Delete": func() error { return k.Delete("s", "u") },
	} {
		start := time.Now()
		err := call()
		if !errors.Is(err, ErrKeyringTimeout) {
			t.Errorf("%s: err = %v, want ErrKeyringTimeout", name, err)
		}
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("%s took %v", name, d)
		}
	}
}

// A keyring that answers in time is passed through unchanged.
func TestTimeoutKeyringPassesThrough(t *testing.T) {
	keyring.MockInit()
	k := TimeoutKeyring{Inner: OSKeyring{}, Timeout: time.Second}
	if err := k.Set("s", "u", "v"); err != nil {
		t.Fatal(err)
	}
	if v, err := k.Get("s", "u"); err != nil || v != "v" {
		t.Fatalf("Get = %q, %v", v, err)
	}
	if err := k.Delete("s", "u"); err != nil {
		t.Fatal(err)
	}
}

// When the keyring hangs, a new profile still saves: its key goes to the file.
func TestStoreSavesWhenKeyringHangs(t *testing.T) {
	b := blockingKeyring{release: make(chan struct{})}
	defer close(b.release)
	s := NewFileStore(t.TempDir(), TimeoutKeyring{Inner: b, Timeout: 50 * time.Millisecond})
	if err := s.Save(testProfile()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load("work")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !got.KeyInFile {
		t.Fatal("key should be in the key file")
	}
}
