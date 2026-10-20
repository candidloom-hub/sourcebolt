package browser

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

// nullPage is a tab where no selector ever matches: the resolver is
// injected on first use and then answers null for every lookup.
type nullPage struct {
	mu       sync.Mutex
	injected bool
	injects  int
}

func (p *nullPage) Eval(ctx context.Context, js string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if js == domJS {
		p.injected = true
		p.injects++
		return []byte("null"), nil
	}
	if !p.injected {
		return []byte(`{"missing":true}`), nil
	}
	return []byte(`{"missing":false,"v":null}`), nil
}

func (p *nullPage) Navigate(ctx context.Context, u string) error { return nil }

func TestFindMissingKeyIsDrift(t *testing.T) {
	s := LoadSelectors("", nil)
	p := &nullPage{}
	start := time.Now()
	err := s.find(t.Context(), p, "chat.input", 200*time.Millisecond)
	if d := time.Since(start); d > time.Second {
		t.Fatalf("find took %s, want < 1s", d)
	}
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindDrift {
		t.Fatalf("err = %v, want KindDrift", err)
	}
	if !strings.Contains(err.Error(), `"chat.input"`) {
		t.Errorf("message %q does not name the key", err)
	}
	if p.injects != 1 {
		t.Errorf("resolver injected %d times, want 1", p.injects)
	}
}

func TestFindCancelReturnsCallerError(t *testing.T) {
	s := LoadSelectors("", nil)
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(50*time.Millisecond, cancel)
	err := s.find(ctx, &nullPage{}, "chat.input", 5*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestOverrideReplacesKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "selectors.json"), []byte(`{"chat.input":["#mine"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	s := LoadSelectors(dir, &warn)
	if got := s["chat.input"]; !slices.Equal(got, []string{"#mine"}) {
		t.Errorf("chat.input = %q, want [#mine]", got)
	}
	def := LoadSelectors("", nil)
	for k, v := range def {
		if k == "chat.input" {
			continue
		}
		if !slices.Equal(s[k], v) {
			t.Errorf("%s = %q, want default %q", k, s[k], v)
		}
	}
	if warn.Len() != 0 {
		t.Errorf("unexpected warning %q", warn.String())
	}
}

func TestBadOverrideKeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "selectors.json"), []byte(`{"chat.input":[`), 0o600); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	s := LoadSelectors(dir, &warn)
	def := LoadSelectors("", nil)
	if len(s) != len(def) {
		t.Fatalf("got %d keys, want %d", len(s), len(def))
	}
	for k, v := range def {
		if !slices.Equal(s[k], v) {
			t.Errorf("%s = %q, want default %q", k, s[k], v)
		}
	}
	if n := strings.Count(warn.String(), "\n"); n != 1 || !strings.HasSuffix(warn.String(), "\n") {
		t.Errorf("warn = %q, want exactly one line", warn.String())
	}
}

func TestEmbeddedRegistryHasAllKeys(t *testing.T) {
	s := LoadSelectors("", nil)
	for _, k := range []string{
		"chat.input", "chat.submit", "chat.bubble", "chat.bubble_text", "chat.done", "chat.alert", "chat.panel",
		"sources.add", "sources.dialog", "sources.type_link", "sources.type_text",
		"sources.url_input", "sources.text_input", "sources.insert", "sources.row",
	} {
		if len(s[k]) == 0 {
			t.Errorf("registry key %q missing or empty", k)
		}
		for _, c := range s[k] {
			if strings.TrimSpace(c) == "" {
				t.Errorf("registry key %q has an empty candidate", k)
			}
		}
	}
	// Page-wide icon or text matches would hit any add control (finding 2).
	for _, c := range s["sources.add"] {
		if strings.HasPrefix(c, "icon:") || strings.HasPrefix(c, "text:") {
			t.Errorf("sources.add candidate %q is not scoped to the sources panel", c)
		}
	}
	// The modal anchor must never be the overlay classes (Global Constraints).
	for _, c := range s["sources.dialog"] {
		if strings.Contains(c, "mdc-dialog--open") || strings.Contains(c, "cdk-overlay-pane") {
			t.Errorf("sources.dialog candidate %q uses an overlay class", c)
		}
	}
}
