package session

import (
	"errors"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

// flakyKeyring stores keys in memory and can fail Get/Set on demand.
type flakyKeyring struct {
	mu      sync.Mutex
	m       map[string]string
	failGet error
	sets    int
}

func (k *flakyKeyring) Get(s, u string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.failGet != nil {
		return "", k.failGet
	}
	v, ok := k.m[s+"/"+u]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return v, nil
}
func (k *flakyKeyring) Set(s, u, v string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.sets++
	k.m[s+"/"+u] = v
	return nil
}
func (k *flakyKeyring) Delete(s, u string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, s+"/"+u)
	return nil
}

func TestTransientKeyringErrorKeepsKey(t *testing.T) {
	kr := &flakyKeyring{m: map[string]string{}}
	s := NewFileStore(t.TempDir(), kr)
	if err := s.Save(testProfile()); err != nil {
		t.Fatal(err)
	}
	p, _ := s.Load("work")
	kr.failGet = errors.New("secret service locked")
	if err := s.Save(p); err == nil {
		t.Fatal("save must fail while the keyring is unavailable")
	}
	if kr.sets != 1 {
		t.Fatalf("keyring Set called %d times; a transient error must never mint a new key", kr.sets)
	}
	kr.failGet = nil
	if _, err := s.Load("work"); err != nil {
		t.Fatalf("profile must still decrypt: %v", err)
	}
}

func TestSaveRetriesRenameOnAccessDenied(t *testing.T) {
	s, _ := newTestStore(t)
	fails := 2
	renameFn = func(a, b string) error {
		if fails > 0 {
			fails--
			return &fs.PathError{Op: "rename", Path: b, Err: fs.ErrPermission}
		}
		return osRename(a, b)
	}
	t.Cleanup(func() { renameFn = osRename })
	if err := s.Save(testProfile()); err != nil {
		t.Fatalf("save must retry a briefly locked file: %v", err)
	}
}

func TestConcurrentSavesOneWins(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Save(testProfile()); err != nil {
		t.Fatal(err)
	}
	a, _ := s.Load("work")
	b, _ := s.Load("work")
	beforeRename = func() error { time.Sleep(50 * time.Millisecond); return nil }
	t.Cleanup(func() { beforeRename = nil })
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, p := range []*Profile{a, b} {
		wg.Add(1)
		go func() { defer wg.Done(); errs[i] = s.Save(p) }()
	}
	wg.Wait()
	ok, stale := 0, 0
	for _, e := range errs {
		switch {
		case e == nil:
			ok++
		case errors.Is(e, ErrStaleRevision):
			stale++
		}
	}
	if ok != 1 || stale != 1 {
		t.Fatalf("want one winner and one stale, got %v", errs)
	}
}

func TestNetscapeIncludeSubdomains(t *testing.T) {
	in := "google.com\tTRUE\t/\tTRUE\t4102444800\tSID\ts\n"
	j, err := ImportNetscape(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(j.HeaderFor("notebook.google.com"), "SID=s") {
		t.Fatalf("include-subdomains row must reach subdomains: %q", j.HeaderFor("notebook.google.com"))
	}
}

func TestDomainPrecedenceCaseInsensitive(t *testing.T) {
	j := jarOf(ck("SID", "host", "notebook.google.com"), ck("SID", "wide", ".Google.com"))
	if h := j.HeaderFor("notebook.google.com"); h != "SID=wide" {
		t.Fatalf("header = %q", h)
	}
}

func TestJarReplace(t *testing.T) {
	j := jarOf(ck("SID", "old", ".google.com"))
	j.Replace([]Cookie{ck("SID", "new", ".google.com"), ck("HSID", "h", ".google.com")})
	if h := j.HeaderFor("notebook.google.com"); h != "HSID=h; SID=new" {
		t.Fatalf("header = %q", h)
	}
}
