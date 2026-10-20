package browser

import (
	"context"
	_ "embed"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

// DOM fallback selector registry.
//
// Each UI control is a registry key with an ordered list of candidates,
// tried first to last: Angular class, then Material icon text, then ARIA
// role or label, then (EN only) visible text. The defaults are embedded;
// <config>/selectors.json may replace the list of any key, so a NotebookLM
// UI change can be patched without a rebuild.

//go:embed selectors.json
var defaultSelectors []byte

// domJS is the in-page resolver (window.__sbDom).
//
//go:embed dom.js
var domJS string

const (
	// selectorWait bounds the wait for one control to appear.
	selectorWait = 10 * time.Second
	// findPoll is how often find asks the page again.
	findPoll = 100 * time.Millisecond
	// domTransport is the transport name the DOM fallback reports.
	domTransport = "browser-dom"
)

// Selectors maps a registry key (e.g. "chat.input") to its candidates.
type Selectors map[string][]string

// LoadSelectors returns the embedded registry merged with the override
// file <configDir>/selectors.json, whose keys replace the default list of
// the same key. A missing file is normal; an unreadable or invalid one is
// ignored with one line on warn. An empty list in the file is ignored too.
func LoadSelectors(configDir string, warn io.Writer) Selectors {
	s := Selectors{}
	if err := json.Unmarshal(defaultSelectors, &s); err != nil {
		panic("browser: embedded selectors.json: " + err.Error())
	}
	if configDir == "" {
		return s
	}
	path := filepath.Join(configDir, "selectors.json")
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s
	}
	var over Selectors
	if err == nil {
		err = json.Unmarshal(raw, &over)
	}
	if err != nil {
		if warn != nil {
			fmt.Fprintf(warn, "sourcebolt: ignoring selector overrides %s: %v\n", path, oneLine(err))
		}
		return s
	}
	for k, v := range over {
		if len(v) > 0 {
			s[k] = v
		}
	}
	return s
}

// oneLine keeps a warning on one line whatever the error text holds.
func oneLine(err error) string {
	b := []byte(err.Error())
	for i, c := range b {
		if c == '\n' || c == '\r' {
			b[i] = ' '
		}
	}
	return string(b)
}

// find polls the page every findPoll until the control key resolves to a
// visible element or wait passes; then it returns KindDrift naming key.
// Promo dialogs are dismissed on every poll.
func (s Selectors) find(ctx context.Context, p page, key string, wait time.Duration) error {
	_, err := s.findIn(ctx, p, "", []string{key}, wait)
	return err
}

// findIn is find for several keys inside root, a JS expression for the
// element to search ("" = the whole document; an expression yielding null
// matches nothing), evaluated again on every poll. It returns the first
// key that resolves; the drift error names the first key.
func (s Selectors) findIn(ctx context.Context, p page, root string, keys []string, wait time.Duration) (string, error) {
	cands := make([][]string, len(keys))
	for i, k := range keys {
		if len(s[k]) == 0 {
			return "", &transport.Error{Kind: transport.KindInternal, Transport: domTransport,
				Message: fmt.Sprintf("selector registry has no key %q", k)}
		}
		cands[i] = s[k]
	}
	arg, err := json.Marshal(cands)
	if err != nil {
		return "", &transport.Error{Kind: transport.KindInternal, Transport: domTransport, Cause: err}
	}
	if root == "" {
		root = "undefined"
	}
	idx, err := s.poll(ctx, p, `((r) => `+string(arg)+`.findIndex((c) => !!__sbDom.find(c, r)))(`+root+`)`, wait, keys[0])
	if err != nil {
		return "", err
	}
	if idx >= len(keys) {
		return "", s.drift(keys[0])
	}
	return keys[idx], nil
}

// poll evaluates expr, an index (-1 = nothing yet), every findPoll until it
// is >= 0 or wait passes; then it returns KindDrift naming key. Promo
// dialogs are dismissed before every evaluation.
func (s Selectors) poll(ctx context.Context, p page, expr string, wait time.Duration, key string) (int, error) {
	expr = `(__sbDom.dismissPromos(), ` + expr + `)`
	deadline := time.Now().Add(wait)
	var lastErr error
	for {
		var idx *int // null (no page answer) is not index 0
		raw, err := domEval(ctx, p, expr)
		if err == nil {
			err = json.Unmarshal(raw, &idx)
		}
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		lastErr = err
		if err == nil && idx != nil && *idx >= 0 {
			return *idx, nil
		}
		if !time.Now().Before(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(findPoll):
		}
	}
	if lastErr != nil {
		return 0, &transport.Error{Kind: transport.KindTransport, Transport: domTransport,
			Cause: fmt.Errorf("browser: resolve %q: %w", key, lastErr)}
	}
	return 0, s.drift(key)
}

// drift is the error for a control that is not on the page.
func (s Selectors) drift(key string) *transport.Error {
	return &transport.Error{Kind: transport.KindDrift, Transport: domTransport,
		Message: fmt.Sprintf("selector %q not found (tried %d candidates); NotebookLM UI may have changed", key, len(s[key]))}
}

// domReply is the envelope domEval wraps every expression in.
type domReply struct {
	Missing bool           `json:"missing"`
	V       jsontext.Value `json:"v"`
}

// domEval evaluates expr with window.__sbDom available, injecting the
// resolver first when the document does not have it yet (a fresh tab or a
// navigation). It returns expr's value as JSON ("null" when undefined).
func domEval(ctx context.Context, p page, expr string) ([]byte, error) {
	js := `(() => { if (typeof window.__sbDom === "undefined") return {missing: true}; ` +
		`const v = (` + expr + `); return {missing: false, v: v === undefined ? null : v}; })()`
	for try := 0; ; try++ {
		raw, err := p.Eval(ctx, js)
		if err != nil {
			return nil, err
		}
		var r domReply
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("browser: dom reply: %w", err)
		}
		if !r.Missing {
			if len(r.V) == 0 {
				return []byte("null"), nil
			}
			return r.V, nil
		}
		if try > 0 {
			return nil, errors.New("browser: dom resolver did not load")
		}
		if _, err := p.Eval(ctx, domJS); err != nil {
			return nil, fmt.Errorf("browser: inject dom resolver: %w", err)
		}
	}
}
