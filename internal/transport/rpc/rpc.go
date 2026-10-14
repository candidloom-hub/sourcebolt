package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/safety"
	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"
)

const (
	batchPath          = "/_/LabsTailwindUi/data/batchexecute"
	maxBody            = 32 << 20
	backoffBase        = time.Second
	backoffCap         = 30 * time.Second
	defaultCallTimeout = 45 * time.Second
)

// Options configures a Transport. Zero values get defaults.
type Options struct {
	Profile     *session.Profile
	Store       session.Store
	HTTP        *http.Client
	Limiter     *rate.Limiter
	MaxRetries  int // retries for retriable transport errors on idempotent calls
	Sleep       func(ctx context.Context, d time.Duration) error
	CallTimeout time.Duration   // per-attempt deadline (default 45 s)
	OnSaveError func(err error) // profile save failures (default: stderr)
}

// Transport is the web RPC transport for one profile.
//
// Locking: mu guards tokens and the Profile's Host and Rev. The Profile's
// Jar pointer never changes (contents are swapped with Jar.Replace) and the
// Jar has its own lock. Network I/O never runs while mu is held.
type Transport struct {
	o      Options
	mu     sync.Mutex
	tokens *session.Tokens
	boot   singleflight.Group
}

// New builds a Transport. Callers share one HTTP client across profiles.
func New(o Options) *Transport {
	if o.HTTP == nil {
		o.HTTP = NewHTTPClient()
	}
	if o.Limiter == nil {
		o.Limiter = rate.NewLimiter(5, 10)
	}
	if o.Sleep == nil {
		o.Sleep = sleepCtx
	}
	if o.CallTimeout <= 0 {
		o.CallTimeout = defaultCallTimeout
	}
	if o.OnSaveError == nil {
		o.OnSaveError = func(err error) {
			fmt.Fprintln(os.Stderr, "sourcebolt: saving profile failed:", safety.Redact(err.Error()))
		}
	}
	return &Transport{o: o}
}

func (t *Transport) Name() string         { return "rpc" }
func (t *Transport) Supports(string) bool { return true }

// Warm fetches page tokens (and with them opens the pooled connection).
func (t *Transport) Warm(ctx context.Context) error {
	_, err := t.ensureTokens(ctx, false)
	return err
}

func (t *Transport) host() session.Host {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.o.Profile.Host
}

// Do executes one batchexecute call. Auth and drift failures re-bootstrap
// tokens once (after picking up a newer login from disk); retriable
// transport failures are retried only when the call is idempotent; rate
// limits are returned, never retried here.
func (t *Transport) Do(ctx context.Context, c transport.Call) (transport.Result, error) {
	refetched := false
	for attempt := 0; ; attempt++ {
		payload, err := t.once(ctx, c)
		if err == nil {
			return transport.Result{Payload: payload, Transport: t.Name()}, nil
		}
		var te *transport.Error
		if !errors.As(err, &te) || ctx.Err() != nil {
			return transport.Result{}, err
		}
		switch {
		case (te.Kind == transport.KindAuth || te.Kind == transport.KindDrift) && !refetched:
			refetched = true
			t.reloadIfNewer()
			if _, berr := t.ensureTokens(ctx, true); berr != nil {
				return transport.Result{}, berr
			}
		case te.Kind == transport.KindTransport && te.Retriable && c.Idempotent && attempt < t.o.MaxRetries:
			if serr := t.o.Sleep(ctx, backoff(attempt)); serr != nil {
				return transport.Result{}, te
			}
		default:
			return transport.Result{}, te
		}
	}
}

func (t *Transport) once(ctx context.Context, c transport.Call) ([]byte, error) {
	if err := t.o.Limiter.Wait(ctx); err != nil {
		return nil, t.fail(c, transport.KindTransport, false, err)
	}
	tok, err := t.ensureTokens(ctx, false)
	if err != nil {
		return nil, err
	}
	body, err := wire.EncodeBatch([]wire.BatchItem{{ID: c.ID, Params: c.Params}}, tok.AT)
	if err != nil {
		return nil, t.fail(c, transport.KindInternal, false, err)
	}
	actx, cancel := context.WithTimeout(ctx, t.o.CallTimeout)
	defer cancel()
	host := t.host()
	req, err := http.NewRequestWithContext(actx, http.MethodPost, host.Origin()+batchPath+"?"+t.query(c, tok).Encode(), strings.NewReader(body))
	if err != nil {
		return nil, t.fail(c, transport.KindInternal, false, err)
	}
	t.setHeaders(req, host)
	resp, err := t.o.HTTP.Do(req)
	if err != nil {
		return nil, t.fail(c, transport.KindTransport, ctx.Err() == nil, err)
	}
	defer resp.Body.Close()
	t.mergeCookies(resp)
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		e := t.fail(c, transport.KindRateLimited, true, fmt.Errorf("HTTP %s", resp.Status))
		e.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, e
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, t.fail(c, transport.KindAuth, false, fmt.Errorf("HTTP %s", resp.Status))
	case resp.StatusCode >= 500:
		return nil, t.fail(c, transport.KindTransport, true, fmt.Errorf("HTTP %s", resp.Status))
	case resp.StatusCode != http.StatusOK:
		return nil, t.fail(c, transport.KindInternal, false, fmt.Errorf("HTTP %s", resp.Status))
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, t.fail(c, transport.KindTransport, ctx.Err() == nil, err)
	}
	frames, tr, err := wire.Decode(raw)
	if tr.FSID != "" {
		t.setFSID(tr.FSID)
	}
	if err != nil {
		return nil, t.wireErr(err, c)
	}
	payload, err := wire.Pick(frames, c.ID, c.Null)
	if err != nil {
		return nil, t.wireErr(err, c)
	}
	return payload, nil
}

func (t *Transport) query(c transport.Call, tok session.Tokens) url.Values {
	q := url.Values{}
	q.Set("rpcids", c.ID)
	sp := c.SourcePath
	if sp == "" {
		sp = "/"
	}
	q.Set("source-path", sp)
	if tok.FSID != "" {
		q.Set("f.sid", tok.FSID)
	}
	if tok.BL != "" {
		q.Set("bl", tok.BL)
	}
	hl := c.HL
	if hl == "" {
		hl = tok.HL
	}
	if hl != "" {
		q.Set("hl", hl)
	}
	q.Set("rt", "c")
	return q
}

func (t *Transport) setHeaders(req *http.Request, h session.Host) {
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("X-Same-Domain", "1")
	req.Header.Set("Origin", h.Origin())
	req.Header.Set("Referer", h.Origin()+"/")
	if ck := t.o.Profile.Jar.HeaderFor(h); ck != "" {
		req.Header.Set("Cookie", ck)
	}
}

// ensureTokens returns cached tokens, or bootstraps them from GET /.
// Concurrent bootstraps share one fetch, which runs without holding mu.
func (t *Transport) ensureTokens(ctx context.Context, force bool) (session.Tokens, error) {
	t.mu.Lock()
	if t.tokens != nil && !force {
		tok := *t.tokens
		t.mu.Unlock()
		return tok, nil
	}
	t.mu.Unlock()
	// Detached from the first caller: one cancelled call must not fail the
	// others waiting on the same bootstrap (CallTimeout still bounds it).
	v, err, _ := t.boot.Do("bootstrap", func() (any, error) { return t.bootstrap(context.WithoutCancel(ctx)) })
	if err != nil {
		return session.Tokens{}, err
	}
	return v.(session.Tokens), nil
}

// bootstrap fetches the page; a redirect to another notebook host switches
// the profile's host once.
func (t *Transport) bootstrap(ctx context.Context) (session.Tokens, error) {
	call := transport.Call{RPC: "session.bootstrap"}
	actx, cancel := context.WithTimeout(ctx, t.o.CallTimeout)
	defer cancel()
	for hop := 0; hop < 2; hop++ {
		hc := *t.o.HTTP
		hc.Jar = t.o.Profile.Jar
		tok, err := session.Bootstrap(actx, &hc, t.host())
		var hr *session.HostRedirectError
		if errors.As(err, &hr) && hop == 0 {
			t.mu.Lock()
			t.o.Profile.Host = hr.To
			t.mu.Unlock()
			t.save(nil)
			continue
		}
		if err != nil {
			if errors.Is(err, session.ErrSignedOut) || errors.Is(err, session.ErrCookieMismatch) ||
				errors.Is(err, session.ErrPageDrift) || errors.As(err, &hr) {
				return session.Tokens{}, t.wireErr(err, call)
			}
			return session.Tokens{}, t.fail(call, transport.KindTransport, actx.Err() == nil, err)
		}
		t.mu.Lock()
		t.tokens = &tok
		t.mu.Unlock()
		return tok, nil
	}
	return session.Tokens{}, t.fail(call, transport.KindAuth, false, errors.New("redirect loop between notebook hosts"))
}

func (t *Transport) setFSID(fsid string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tokens != nil {
		t.tokens.FSID = fsid
	}
}

// reloadIfNewer adopts a profile saved by another process (a fresh
// `sourcebolt login`) and drops cached tokens.
func (t *Transport) reloadIfNewer() {
	if t.o.Store == nil {
		return
	}
	latest, err := t.o.Store.Load(t.o.Profile.Name)
	if err != nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if latest.Rev > t.o.Profile.Rev {
		t.adoptLocked(latest)
		t.tokens = nil
	}
}

func (t *Transport) adoptLocked(latest *session.Profile) {
	p := t.o.Profile
	p.Jar.Replace(latest.Jar.All())
	p.Host, p.Rev, p.Updated, p.KeyInFile = latest.Host, latest.Rev, latest.Updated, latest.KeyInFile
}

// mergeCookies stores rotated cookies and persists the profile.
func (t *Transport) mergeCookies(resp *http.Response) {
	cs := resp.Cookies()
	if len(cs) == 0 {
		return
	}
	t.o.Profile.Jar.Merge(resp)
	t.save(func(j *session.Jar) { j.SetCookies(resp.Request.URL, cs) })
}

// save persists the profile. If another process saved first, it reloads the
// latest version, applies only this response's cookies (never the whole old
// jar, which could resurrect an expired session over a fresh login), saves
// that, and adopts it. Failures are reported, never swallowed.
func (t *Transport) save(apply func(*session.Jar)) {
	if t.o.Store == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	p := t.o.Profile
	snapshot := &session.Profile{Name: p.Name, Host: p.Host, Jar: session.NewJar(), Rev: p.Rev}
	snapshot.Jar.Replace(p.Jar.All())
	err := t.o.Store.Save(snapshot)
	if err == nil {
		p.Rev, p.Updated, p.KeyInFile = snapshot.Rev, snapshot.Updated, snapshot.KeyInFile
		return
	}
	if !errors.Is(err, session.ErrStaleRevision) {
		t.o.OnSaveError(err)
		return
	}
	latest, err := t.o.Store.Load(p.Name)
	if err != nil {
		t.o.OnSaveError(err)
		return
	}
	if apply != nil {
		apply(latest.Jar)
	}
	if err := t.o.Store.Save(latest); err != nil {
		t.o.OnSaveError(err)
		return
	}
	t.adoptLocked(latest)
	t.tokens = nil
}

func (t *Transport) fail(c transport.Call, k transport.Kind, retriable bool, err error) *transport.Error {
	return &transport.Error{Kind: k, RPC: c.RPC, Retriable: retriable, Transport: t.Name(), Cause: stripURL(err)}
}

// stripURL removes the query (f.sid, bl, rpcids) from *url.Error so request
// details never reach error messages, logs or MCP envelopes.
func stripURL(err error) error {
	var ue *url.Error
	if !errors.As(err, &ue) {
		return err
	}
	clean := ue.URL
	if u, perr := url.Parse(ue.URL); perr == nil {
		u.RawQuery = ""
		clean = u.String()
	} else if i := strings.IndexByte(clean, '?'); i >= 0 {
		clean = clean[:i]
	}
	return &url.Error{Op: ue.Op, URL: clean, Err: ue.Err}
}

func (t *Transport) wireErr(err error, c transport.Call) *transport.Error {
	e := transport.FromWire(err, c)
	e.Transport = t.Name()
	return e
}

// backoff is exponential from 1 s, capped at 30 s, with ±20% jitter.
func backoff(attempt int) time.Duration {
	d := backoffBase << attempt
	if d > backoffCap || d <= 0 {
		d = backoffCap
	}
	jitter := 0.8 + 0.4*rand.Float64()
	return time.Duration(float64(d) * jitter)
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if s, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
		return time.Duration(s) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
