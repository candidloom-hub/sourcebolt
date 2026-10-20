//go:build browser

package browser

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

// newFixtureDOM serves testdata/dom/<nb>.html at /notebook/<nb> and returns
// a DOM on a headless browser with a temp profile, pointed at that server.
func newFixtureDOM(t *testing.T) (*DOM, *httptest.Server) {
	t.Helper()
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", t.TempDir())
	mux := http.NewServeMux()
	mux.HandleFunc("/notebook/{nb}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("testdata", "dom", filepath.Base(r.PathValue("nb"))+".html"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	l := NewLauncher(Options{ProfileDir: profileDir(t), Headless: true})
	t.Cleanup(func() { l.Close() })
	d := newDOM(tabAcquirer(l.Get), session.Host(strings.TrimPrefix(srv.URL, "http://")), LoadSelectors("", nil))
	d.origin = srv.URL
	return d, srv
}

// openFixture puts the warm tab on a fixture variant before the call, so
// the DOM finds it already on the notebook and does not navigate.
func openFixture(t *testing.T, d *DOM, u string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	p, release, err := d.acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err := p.Navigate(ctx, u); err != nil {
		t.Fatal(err)
	}
}

func wantKind(t *testing.T, err error, k transport.Kind) *transport.Error {
	t.Helper()
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != k {
		t.Fatalf("err = %v, want %s", err, k)
	}
	return te
}

func TestAskDOMAnswers(t *testing.T) {
	d, srv := newFixtureDOM(t)
	// The answer bubble appears only after 1.5 s, so the previous (complete)
	// answer must not be taken for this one.
	openFixture(t, d, srv.URL+"/notebook/chat?think=1500")
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	var calls atomic.Int32
	text, err := d.Ask(ctx, "chat", "what is up", func(string) { calls.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	if want := "Fixture answer to: what is up [1]"; text != want {
		t.Errorf("text = %q, want %q", text, want)
	}
	if calls.Load() == 0 {
		t.Error("progress was never called")
	}
}

func TestAskDOMSameAnswerCompletes(t *testing.T) {
	d, srv := newFixtureDOM(t)
	openFixture(t, d, srv.URL+"/notebook/chat?same=1")
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	start := time.Now()
	text, err := d.Ask(ctx, "chat", "again", nil)
	if err != nil {
		t.Fatal(err)
	}
	if el := time.Since(start); el > 5*time.Second {
		t.Errorf("took %s, want < 5s", el)
	}
	if text != "Same answer." {
		t.Errorf("text = %q", text)
	}
}

func TestAskDOMRateLimitAlert(t *testing.T) {
	d, srv := newFixtureDOM(t)
	openFixture(t, d, srv.URL+"/notebook/chat?limit=1")
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	_, err := d.Ask(ctx, "chat", "one more", nil)
	wantKind(t, err, transport.KindRateLimited)
}

func TestAskDOMMissingInputIsDrift(t *testing.T) {
	d, srv := newFixtureDOM(t)
	openFixture(t, d, srv.URL+"/notebook/chat?noinput=1")
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	start := time.Now()
	_, err := d.Ask(ctx, "chat", "hello", nil)
	if el := time.Since(start); el > selectorWait+time.Second {
		t.Errorf("took %s, want <= %s", el, selectorWait+time.Second)
	}
	te := wantKind(t, err, transport.KindDrift)
	if !strings.Contains(te.Error(), `"chat.input"`) {
		t.Errorf("drift %q does not name chat.input", te)
	}
}

func TestAskDOMCancel(t *testing.T) {
	d, srv := newFixtureDOM(t)
	openFixture(t, d, srv.URL+"/notebook/chat")
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(100*time.Millisecond, cancel)
	start := time.Now()
	_, err := d.Ask(ctx, "chat", "slow one", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if el := time.Since(start); el > time.Second {
		t.Errorf("cancel took %s, want < 1s", el)
	}
	ctx2, cancel2 := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel2()
	text, err := d.Ask(ctx2, "chat", "next", nil)
	if err != nil {
		t.Fatalf("second Ask: %v", err)
	}
	if want := "Fixture answer to: next [1]"; text != want {
		t.Errorf("second text = %q, want %q", text, want)
	}
}

func TestAskDOMPromoDialog(t *testing.T) {
	d, srv := newFixtureDOM(t)
	openFixture(t, d, srv.URL+"/notebook/chat?promo=1")
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	text, err := d.Ask(ctx, "chat", "past the promo", nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Fixture answer to: past the promo [1]"; text != want {
		t.Errorf("text = %q, want %q", text, want)
	}
}

func TestAskDOMWaitsForStreamingAnswer(t *testing.T) {
	d, srv := newFixtureDOM(t)
	openFixture(t, d, srv.URL+"/notebook/chat?streaming=1")
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	text, err := d.Ask(ctx, "chat", "mine", nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Fixture answer to: mine [1]"; text != want {
		t.Errorf("text = %q, want %q", text, want)
	}
}

func TestAskDOMAfterCancelGetsOwnAnswer(t *testing.T) {
	d, srv := newFixtureDOM(t)
	// The answer bubble appears 1.5 s after submit: the cancel at 0.5 s
	// lands after the question went out but before its bubble exists.
	openFixture(t, d, srv.URL+"/notebook/chat?think=1500")
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(500*time.Millisecond, cancel)
	if _, err := d.Ask(ctx, "chat", "abandoned", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("first Ask err = %v, want context.Canceled", err)
	}
	ctx2, cancel2 := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel2()
	text, err := d.Ask(ctx2, "chat", "next", nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Fixture answer to: next [1]"; text != want {
		t.Errorf("text = %q, want %q (the abandoned answer leaked)", text, want)
	}
}

func TestAskDOMBusyPageNeverSends(t *testing.T) {
	d, srv := newFixtureDOM(t)
	d.idle = time.Second
	openFixture(t, d, srv.URL+"/notebook/chat?streaming=stuck")
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	_, err := d.Ask(ctx, "chat", "never sent", nil)
	te := wantKind(t, err, transport.KindTransport)
	if te.Sent {
		t.Error("Sent set although the question was never sent")
	}
	p, release, err := d.acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	raw, err := p.Eval(ctx, `document.querySelector("textarea").value + "|" + document.querySelectorAll(".from-user-container").length`)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(raw); got != `"|1"` {
		t.Errorf("input|questions = %s, want \"|1\" (nothing typed or sent)", got)
	}
}

// sourcesPage reads the fixture's source rows and whether a dialog is open.
func sourcesPage(t *testing.T, d *DOM) (rows []string, dialog bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	p, release, err := d.acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	raw, err := p.Eval(ctx, `({rows: Array.from(document.querySelectorAll(".single-source-container")).map((e) => e.textContent),
		dialog: !!document.querySelector("mat-dialog-container")})`)
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		Rows   []string `json:"rows"`
		Dialog bool     `json:"dialog"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	return st.Rows, st.Dialog
}

func wantSources(t *testing.T, d *DOM, want ...string) {
	t.Helper()
	rows, dialog := sourcesPage(t, d)
	if want = append([]string{"Existing source"}, want...); !slices.Equal(rows, want) {
		t.Errorf("rows = %q, want %q", rows, want)
	}
	if dialog {
		t.Error("dialog still open")
	}
}

func TestAddURLDOM(t *testing.T) {
	d, _ := newFixtureDOM(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	if err := d.AddURL(ctx, "sources", "https://example.test/page"); err != nil {
		t.Fatal(err)
	}
	wantSources(t, d, "Website: https://example.test/page")
}

func TestAddTextDOM(t *testing.T) {
	d, _ := newFixtureDOM(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	if err := d.AddText(ctx, "sources", "My title", "pasted body"); err != nil {
		t.Fatal(err)
	}
	wantSources(t, d, "Pasted text: pasted body")
}

func TestAddSourceDOMMissingInsertIsDrift(t *testing.T) {
	d, srv := newFixtureDOM(t)
	openFixture(t, d, srv.URL+"/notebook/sources?noinsert=1")
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	err := d.AddURL(ctx, "sources", "https://example.test/x")
	te := wantKind(t, err, transport.KindDrift)
	if !strings.Contains(te.Error(), `"sources.insert"`) {
		t.Errorf("drift %q does not name sources.insert", te)
	}
	if te.Sent {
		t.Error("Sent set although Insert was never clicked")
	}
}

func TestAddSourceDOMAddSourceParamFallback(t *testing.T) {
	d, srv := newFixtureDOM(t)
	openFixture(t, d, srv.URL+"/notebook/sources?hideadd=1")
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	if err := d.AddURL(ctx, "sources", "https://example.test/y"); err != nil {
		t.Fatal(err)
	}
	wantSources(t, d, "Website: https://example.test/y")
}
