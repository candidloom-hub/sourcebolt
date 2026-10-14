package browser

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
	"github.com/chromedp/chromedp"
)

// In-page fetch transport.
//
// It runs the same batchexecute request as the rpc transport, but from
// inside a signed-in NotebookLM tab: the browser adds the cookies, Origin
// and its own TLS/HTTP fingerprint, and the CSRF token, f.sid, build label
// and language come from the page's WIZ_global_data. The request URL,
// query, body (wire.EncodeBatch) and the headers that a page may set match
// rpc.go exactly; Origin, Referer and Cookie are set by the browser
// (Referer via the fetch referrer option).
//
// Risks (recorded for P4):
//   - Headless by default. Google may treat headless Chromium differently
//     from a visible window (bot checks, "browser may not be secure"); the
//     fallback must then run visible (Options.Headless=false).
//   - The profile dir is the login profile (<config>/browser/<profile>). A
//     session imported with --attach or --import lives only in the rpc
//     profile's cookie jar, not in this browser profile, so the tab is
//     signed out until `login --browser` has run on it (KindAuth).
//   - navigator.webdriver is true in any CDP-launched browser.
//   - A page CSP could block fetch; today batchexecute is same-origin.

const (
	batchPath           = "/_/LabsTailwindUi/data/batchexecute"
	maxBody             = 32 << 20
	defaultFetchTimeout = 45 * time.Second
	// tokenMaxAge reloads the warm tab when its tokens are this old, so a
	// long-lived tab does not run on an expired CSRF token.
	tokenMaxAge = 30 * time.Minute
	// DefaultIdle closes a fallback browser after this long without a call.
	DefaultIdle = 5 * time.Minute
)

// FetchOptions are the launch options for the fetch transport: headless,
// reaped after DefaultIdle, on the same dedicated profile dir that
// `login --browser` signs in (<config>/browser/<profile>).
func FetchOptions(configDir, profile string) Options {
	return Options{ProfileDir: filepath.Join(configDir, "browser", profile), Headless: true, Idle: DefaultIdle}
}

// page is the part of a browser tab the fetch transport uses. Eval
// evaluates a script (awaiting a returned promise) and returns its result
// as JSON. Tests substitute a fake.
type page interface {
	Eval(ctx context.Context, js string) ([]byte, error)
	Navigate(ctx context.Context, url string) error
}

// tokenState is what tokensJS reads from the page. The token values never
// leave this package and are never logged.
type tokenState struct {
	Href   string `json:"href"`
	Origin string `json:"origin"`
	AT     string `json:"at"`    // SNlM0e
	FSID   string `json:"fsid"`  // FdrFJe
	BL     string `json:"bl"`    // cfb2h
	HL     string `json:"hl"`    // <html lang>
	Email  bool   `json:"email"` // oPEP7c present (value not read)
}

const tokensJS = `(() => {
	const w = window.WIZ_global_data || {};
	const s = (v) => (v == null ? "" : String(v));
	return {href: s(location.href), origin: s(location.origin), at: s(w.SNlM0e), fsid: s(w.FdrFJe),
		bl: s(w.cfb2h), hl: s(document.documentElement && document.documentElement.lang), email: !!w.oPEP7c};
})()`

// fetchRequest is the argument of fetchJS: exactly what fetch() receives.
type fetchRequest struct {
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	Referrer    string            `json:"referrer"`
	Credentials string            `json:"credentials"`
	TimeoutMS   int64             `json:"timeoutMs"`
}

type fetchReply struct {
	Status     int    `json:"status"`
	RetryAfter string `json:"retryAfter"`
	Body       string `json:"body"`
	Error      string `json:"error"`
}

// fetchJS is a function expression; the transport appends "(<request JSON>)".
const fetchJS = `(async (q) => {
	const ctl = new AbortController();
	const timer = setTimeout(() => ctl.abort(), q.timeoutMs);
	try {
		const r = await fetch(q.url, {method: q.method, headers: q.headers, body: q.body,
			referrer: q.referrer, credentials: q.credentials, signal: ctl.signal});
		const body = await r.text();
		return {status: r.status, retryAfter: r.headers.get("Retry-After") || "", body, error: ""};
	} catch (e) {
		return {status: 0, retryAfter: "", body: "", error: String((e && e.message) || e)};
	} finally {
		clearTimeout(timer);
	}
})`

type fetchTransport struct {
	acquire func(ctx context.Context) (page, func(), error)
	host    session.Host
	origin  string // https://<host>; tests point it at an httptest server
	timeout time.Duration

	mu       sync.Mutex
	stale    bool      // reload the tab before the next call
	loadedAt time.Time // when the tab last navigated to the notebook host
}

// NewTransport returns the browser transport (Name "browser"). get returns
// the running browser (see Launcher.Get); each call locks the browser's
// warm tab (FIFO), makes sure it is on the notebook host, reads the page
// tokens and runs batchexecute with fetch() inside the page.
func NewTransport(get func(ctx context.Context) (*Browser, error), host session.Host) transport.Transport {
	return newFetchTransport(tabAcquirer(get), host)
}

func newFetchTransport(acquire func(ctx context.Context) (page, func(), error), host session.Host) *fetchTransport {
	if host == "" {
		host = session.HostDefault
	}
	return &fetchTransport{acquire: acquire, host: host, origin: host.Origin(), timeout: defaultFetchTimeout}
}

func (t *fetchTransport) Name() string         { return "browser" }
func (t *fetchTransport) Supports(string) bool { return true }

// Do runs one call. It does not retry: the router and the domain client own
// retry policy, and a fallback call must not multiply requests.
func (t *fetchTransport) Do(ctx context.Context, c transport.Call) (transport.Result, error) {
	p, release, err := t.acquire(ctx)
	if err != nil {
		var te *transport.Error
		if errors.As(err, &te) {
			return transport.Result{}, err
		}
		return transport.Result{}, t.fail(c, transport.KindTransport, ctx.Err() == nil, false, err)
	}
	defer release()

	st, err := t.ready(ctx, p, c)
	if err != nil {
		return transport.Result{}, err
	}
	payload, err := t.fetch(ctx, p, c, st)
	if err != nil {
		var te *transport.Error
		if errors.As(err, &te) && (te.Kind == transport.KindAuth || te.Kind == transport.KindDrift) {
			t.markStale()
		}
		return transport.Result{}, err
	}
	return transport.Result{Payload: payload, Transport: t.Name()}, nil
}

// ready returns the page tokens, navigating the tab to the notebook host
// first when it is elsewhere (a fresh about:blank tab), signed out, or due
// for a reload.
func (t *fetchTransport) ready(ctx context.Context, p page, c transport.Call) (tokenState, error) {
	if !t.needsReload() {
		st, err := t.readTokens(ctx, p, c)
		if err != nil {
			return tokenState{}, err
		}
		if t.onNotebookHost(st) && st.AT != "" {
			return st, nil
		}
	}
	nctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	if err := p.Navigate(nctx, t.origin+"/"); err != nil {
		return tokenState{}, t.fail(c, transport.KindTransport, ctx.Err() == nil, false, fmt.Errorf("browser: open %s: %w", t.origin, err))
	}
	st, err := t.readTokens(ctx, p, c)
	if err != nil {
		return tokenState{}, err
	}
	if err := t.classify(st); err != nil {
		return tokenState{}, t.wireErr(err, c, false)
	}
	t.mu.Lock()
	t.stale, t.loadedAt = false, time.Now()
	t.mu.Unlock()
	return st, nil
}

func (t *fetchTransport) readTokens(ctx context.Context, p page, c transport.Call) (tokenState, error) {
	ectx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	raw, err := p.Eval(ectx, tokensJS)
	if err != nil {
		return tokenState{}, t.fail(c, transport.KindTransport, ctx.Err() == nil, false, fmt.Errorf("browser: read page tokens: %w", err))
	}
	var st tokenState
	if err := json.Unmarshal(raw, &st); err != nil {
		return tokenState{}, t.fail(c, transport.KindDrift, false, false, fmt.Errorf("browser: page tokens: %w", err))
	}
	return st, nil
}

// onNotebookHost reports whether the page is on the configured host or on
// any notebook host (the account may have landed on another one).
func (t *fetchTransport) onNotebookHost(st tokenState) bool {
	if st.Origin == t.origin {
		return true
	}
	u, err := url.Parse(st.Href)
	return err == nil && (u.Scheme == "https" && session.IsNotebookHost(u))
}

// classify mirrors session.Bootstrap for a page that the browser loaded.
func (t *fetchTransport) classify(st tokenState) error {
	u, err := url.Parse(st.Href)
	if err != nil {
		return session.ErrPageDrift
	}
	switch {
	case u.Hostname() == "accounts.google.com" && strings.Contains(u.Path, "CookieMismatch"):
		return session.ErrCookieMismatch
	case u.Hostname() == "accounts.google.com":
		return fmt.Errorf("%w (sign-in page)", session.ErrSignedOut)
	case !t.onNotebookHost(st):
		return fmt.Errorf("%w: page ended on %s", session.ErrSignedOut, u.Hostname())
	case st.AT == "" && !st.Email:
		return fmt.Errorf("%w (signed-out page)", session.ErrSignedOut)
	case st.AT == "":
		return session.ErrPageDrift
	}
	return nil
}

func (t *fetchTransport) fetch(ctx context.Context, p page, c transport.Call, st tokenState) ([]byte, error) {
	body, err := wire.EncodeBatch([]wire.BatchItem{{ID: c.ID, Params: c.Params}}, st.AT)
	if err != nil {
		return nil, t.fail(c, transport.KindInternal, false, false, err)
	}
	origin := st.Origin
	if origin == "" || origin == "null" {
		origin = t.origin
	}
	req := fetchRequest{
		URL:    origin + batchPath + "?" + query(c, st).Encode(),
		Method: http.MethodPost,
		Headers: map[string]string{
			"Content-Type":  "application/x-www-form-urlencoded;charset=UTF-8",
			"X-Same-Domain": "1",
		},
		Body:        body,
		Referrer:    origin + "/",
		Credentials: "same-origin",
		TimeoutMS:   t.timeout.Milliseconds(),
	}
	arg, err := json.Marshal(req)
	if err != nil {
		return nil, t.fail(c, transport.KindInternal, false, false, err)
	}
	// The page aborts the fetch at TimeoutMS; this deadline only guards a
	// hung DevTools connection.
	ectx, cancel := context.WithTimeout(ctx, t.timeout+5*time.Second)
	defer cancel()
	raw, err := p.Eval(ectx, fetchJS+"("+string(arg)+")")
	// From here on the request may have reached the server: Sent is set.
	if err != nil {
		return nil, t.fail(c, transport.KindTransport, ctx.Err() == nil, true, fmt.Errorf("browser: in-page fetch: %w", err))
	}
	var rep fetchReply
	if err := json.Unmarshal(raw, &rep); err != nil {
		return nil, t.fail(c, transport.KindTransport, false, true, fmt.Errorf("browser: fetch reply: %w", err))
	}
	switch {
	case rep.Error != "":
		return nil, t.fail(c, transport.KindTransport, true, true, fmt.Errorf("browser: in-page fetch: %s", rep.Error))
	case rep.Status == http.StatusTooManyRequests:
		e := t.fail(c, transport.KindRateLimited, true, true, fmt.Errorf("HTTP %d", rep.Status))
		e.RetryAfter = parseRetryAfter(rep.RetryAfter)
		return nil, e
	case rep.Status == http.StatusUnauthorized:
		return nil, t.fail(c, transport.KindAuth, false, true, fmt.Errorf("HTTP %d", rep.Status))
	case rep.Status >= 500:
		return nil, t.fail(c, transport.KindTransport, true, true, fmt.Errorf("HTTP %d", rep.Status))
	case rep.Status != http.StatusOK:
		return nil, t.fail(c, transport.KindInternal, false, true, fmt.Errorf("HTTP %d", rep.Status))
	case len(rep.Body) > maxBody:
		return nil, t.fail(c, transport.KindTransport, false, true, fmt.Errorf("browser: response over %d bytes", maxBody))
	}
	frames, _, err := wire.Decode([]byte(rep.Body))
	if err != nil {
		return nil, t.wireErr(err, c, true)
	}
	payload, err := wire.Pick(frames, c.ID, c.Null)
	if err != nil {
		return nil, t.wireErr(err, c, true)
	}
	return payload, nil
}

// query builds the batchexecute query exactly like rpc.go: rpcids,
// source-path, f.sid, bl, hl (Call.HL overrides the page language), rt=c.
func query(c transport.Call, st tokenState) url.Values {
	q := url.Values{}
	q.Set("rpcids", c.ID)
	sp := c.SourcePath
	if sp == "" {
		sp = "/"
	}
	q.Set("source-path", sp)
	if st.FSID != "" {
		q.Set("f.sid", st.FSID)
	}
	if st.BL != "" {
		q.Set("bl", st.BL)
	}
	hl := c.HL
	if hl == "" {
		hl = st.HL
	}
	if hl != "" {
		q.Set("hl", hl)
	}
	q.Set("rt", "c")
	return q
}

func (t *fetchTransport) needsReload() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stale || (!t.loadedAt.IsZero() && time.Since(t.loadedAt) > tokenMaxAge)
}

func (t *fetchTransport) markStale() {
	t.mu.Lock()
	t.stale = true
	t.mu.Unlock()
}

func (t *fetchTransport) fail(c transport.Call, k transport.Kind, retriable, sent bool, err error) *transport.Error {
	return &transport.Error{Kind: k, RPC: c.RPC, Retriable: retriable, Transport: t.Name(), Sent: sent, Cause: err}
}

func (t *fetchTransport) wireErr(err error, c transport.Call, sent bool) *transport.Error {
	e := transport.FromWire(err, c)
	e.Transport, e.Sent = t.Name(), sent
	return e
}

func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if s, err := strconv.Atoi(v); err == nil {
		return time.Duration(s) * time.Second
	}
	if at, err := http.ParseTime(v); err == nil {
		if d := time.Until(at); d > 0 {
			return d
		}
	}
	return 0
}

// tabAcquirer locks the warm tab of the browser that get returns. If that
// browser closed in between (idle reap, user closed it), it asks get once
// more, which relaunches.
func tabAcquirer(get func(ctx context.Context) (*Browser, error)) func(ctx context.Context) (page, func(), error) {
	return func(ctx context.Context) (page, func(), error) {
		for try := 0; ; try++ {
			b, err := get(ctx)
			if err != nil {
				return nil, nil, err
			}
			tab, release, err := b.Tab(ctx)
			if errors.Is(err, ErrClosed) && try == 0 {
				continue
			}
			if err != nil {
				return nil, nil, err
			}
			return cdpPage{tab: tab}, release, nil
		}
	}
}

// cdpPage runs on a locked chromedp tab context.
type cdpPage struct{ tab context.Context }

// run derives a context that ends with the tab or with ctx.
func (p cdpPage) run(ctx context.Context) (context.Context, func()) {
	rctx, cancel := context.WithCancel(p.tab)
	stop := context.AfterFunc(ctx, cancel)
	return rctx, func() { stop(); cancel() }
}

func (p cdpPage) Eval(ctx context.Context, js string) ([]byte, error) {
	rctx, done := p.run(ctx)
	defer done()
	return chromedp.Run(rctx, chromedp.Evaluate[[]byte](js, chromedp.EvalAwaitPromise))
}

func (p cdpPage) Navigate(ctx context.Context, u string) error {
	rctx, done := p.run(ctx)
	defer done()
	return chromedp.Do(rctx, chromedp.Navigate(u))
}

// Launcher starts the fallback browser on first use and again after it was
// reaped or closed; concurrent callers share one browser.
type Launcher struct {
	o            Options
	launch       func(ctx context.Context, o Options) (*Browser, error)
	now          func() time.Time
	startTimeout time.Duration

	base    context.Context // ended by Close
	cancel  context.CancelFunc
	sem     chan struct{}
	b       *Browser
	bCancel context.CancelFunc // ends b's launch context
	closed  bool
	failAt  time.Time // last failed launch
	failErr error
}

const (
	// launchBackoff is how long a failed launch is remembered: calls in
	// that window fail at once instead of starting a browser again.
	launchBackoff = 60 * time.Second
	// defaultStartTimeout bounds one browser start.
	defaultStartTimeout = 30 * time.Second
)

// NewLauncher returns a Launcher for o (see FetchOptions).
func NewLauncher(o Options) *Launcher {
	base, cancel := context.WithCancel(context.Background())
	return &Launcher{o: o, launch: Launch, now: time.Now, startTimeout: defaultStartTimeout,
		base: base, cancel: cancel, sem: make(chan struct{}, 1)}
}

// Get returns the running browser, launching it if needed. The browser's
// lifetime is the Launcher's, not ctx's; ctx bounds only the wait for
// another caller's launch. A launch is bounded by the start timeout; after
// a failed launch, Get fails fast (KindTransport) for launchBackoff.
func (l *Launcher) Get(ctx context.Context) (*Browser, error) {
	select {
	case l.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-l.base.Done():
		return nil, ErrClosed
	}
	defer func() { <-l.sem }()
	if l.closed || l.base.Err() != nil {
		return nil, ErrClosed
	}
	if l.b != nil && !l.b.gone() {
		return l.b, nil
	}
	l.dropBrowser()
	if l.failErr != nil {
		if ago := l.now().Sub(l.failAt); ago < launchBackoff {
			return nil, &transport.Error{Kind: transport.KindTransport, Transport: "browser",
				Cause: fmt.Errorf("browser: launch failed %s ago, not retrying for %s: %w",
					ago.Round(time.Second), (launchBackoff - ago).Round(time.Second), l.failErr)}
		}
	}
	b, cancel, err := l.start()
	if err != nil {
		if l.base.Err() != nil { // closed during the launch: not a launch failure
			return nil, ErrClosed
		}
		l.failErr, l.failAt = err, l.now()
		return nil, err
	}
	l.failErr = nil
	l.b, l.bCancel = b, cancel
	return b, nil
}

// start runs one launch under the start timeout. The launch context lives
// on as the browser's lifetime; it is cancelled on failure, on timeout and
// when Close aborts the launch. Caller holds l.sem.
func (l *Launcher) start() (*Browser, context.CancelFunc, error) {
	lctx, cancel := context.WithCancel(l.base)
	type result struct {
		b   *Browser
		err error
	}
	ch := make(chan result, 1)
	go func() {
		b, err := l.launch(lctx, l.o)
		ch <- result{b, err}
	}()
	timer := time.NewTimer(l.startTimeout)
	defer timer.Stop()
	var err error
	select {
	case r := <-ch:
		if r.err != nil {
			cancel()
			return nil, nil, r.err
		}
		return r.b, cancel, nil
	case <-timer.C:
		err = fmt.Errorf("browser: start did not finish within %s", l.startTimeout)
	case <-l.base.Done():
		err = ErrClosed
	}
	cancel()
	go func() { // a launch that still succeeds after cancellation is closed
		if r := <-ch; r.b != nil {
			r.b.Close()
		}
	}()
	return nil, nil, err
}

// dropBrowser closes the current browser, if any. Caller holds l.sem.
func (l *Launcher) dropBrowser() error {
	var err error
	if l.b != nil {
		err = l.b.Close()
		l.b = nil
	}
	if l.bCancel != nil {
		l.bCancel()
		l.bCancel = nil
	}
	return err
}

// Close stops the browser and refuses later launches. It does not wait
// behind a launch in progress: that launch is aborted.
func (l *Launcher) Close() error {
	select {
	case l.sem <- struct{}{}:
	default:
		// A Get holds the semaphore, possibly in a hung launch: end the
		// launcher's context so it returns, then take the semaphore.
		l.cancel()
		l.sem <- struct{}{}
	}
	defer func() { <-l.sem }()
	l.closed = true
	err := l.dropBrowser()
	l.cancel()
	return err
}
