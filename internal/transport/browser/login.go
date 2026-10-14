package browser

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

var (
	// ErrCancelled means the user closed the login window (or interrupted)
	// before sign-in finished. Nothing is saved. The MCP code is CANCELLED.
	ErrCancelled = errors.New("CANCELLED: browser window closed before sign-in finished")
	// ErrTimeout means the user did not finish signing in in time.
	ErrTimeout = errors.New("TIMEOUT: sign-in did not finish in time")

	errGone = errors.New("browser: login tab is gone")
)

// probeEvery is how often LoginInteractive checks the page.
const probeEvery = 500 * time.Millisecond

// afterNavigate is a test hook that runs once the login tab has navigated.
var afterNavigate func(b *Browser, tab context.Context)

// pageState is what the login probe reads from the page. It carries only
// booleans for the tokens, never their values.
type pageState struct {
	Href  string `json:"href"`
	AT    bool   `json:"at"`
	Email bool   `json:"email"`
}

const probeJS = `(() => {
	const w = window.WIZ_global_data || {};
	return {href: String(location.href), at: !!w.SNlM0e, email: !!w.oPEP7c};
})()`

// signedInHost reports whether s is a signed-in NotebookLM page: a notebook
// host (or the host the caller asked for) whose WIZ_global_data has both the
// CSRF token SNlM0e and the account email oPEP7c. It returns the host the
// account landed on.
func signedInHost(s pageState, want session.Host) (session.Host, bool) {
	if !s.AT || !s.Email {
		return "", false
	}
	u, err := url.Parse(s.Href)
	if err != nil || u.Host == "" {
		return "", false
	}
	if !session.IsNotebookHost(u) && !strings.EqualFold(u.Host, string(want)) {
		return "", false
	}
	return session.Host(u.Host), true
}

// waitSignedIn polls probe until the page is signed in. errGone from probe
// means the window was closed; other probe errors (navigation in progress)
// are retried.
func waitSignedIn(ctx context.Context, probe func(context.Context) (pageState, error), want session.Host, every time.Duration) (session.Host, error) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return "", ctxErr(err)
		}
		s, err := probe(ctx)
		switch {
		case errors.Is(err, errGone):
			return "", ErrCancelled
		case err == nil:
			if h, ok := signedInHost(s, want); ok {
				return h, nil
			}
		}
		select {
		case <-ctx.Done():
			return "", ctxErr(ctx.Err())
		case <-t.C:
		}
	}
}

func ctxErr(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", ErrTimeout, err)
	}
	return fmt.Errorf("%w: %w", ErrCancelled, err)
}

// notebookHosts mirrors session's notebook host list for cookie filtering.
var notebookHosts = []string{
	"notebook.google.com", "notebooklm.google.com",
	"notebook.cloud.google.com", "notebooklm.cloud.google.com",
}

// cookieHosts lists the hosts whose cookies a session needs: every notebook
// host, the requested and landed hosts, and Google sign-in.
func cookieHosts(want, landed session.Host) []string {
	hosts := slices.Clone(notebookHosts)
	for _, h := range []session.Host{want, landed} {
		if h == "" {
			continue
		}
		u := url.URL{Host: string(h)}
		if n := strings.ToLower(u.Hostname()); !slices.Contains(hosts, n) {
			hosts = append(hosts, n)
		}
	}
	return append(hosts, "accounts.google.com", "www.google.com")
}

// domainMatches applies cookie domain matching: ".google.com" covers
// subdomains, a domain without a dot is host-only.
func domainMatches(domain, host string) bool {
	d := strings.ToLower(domain)
	if rest, ok := strings.CutPrefix(d, "."); ok {
		return host == rest || strings.HasSuffix(host, d)
	}
	return host == d
}

// toSessionCookies converts CDP cookies, keeping only cookies that a browser
// would send to one of hosts. CDP includes HttpOnly cookies.
func toSessionCookies(in []*network.Cookie, hosts []string) []session.Cookie {
	var out []session.Cookie
	for _, c := range in {
		if c == nil || !slices.ContainsFunc(hosts, func(h string) bool { return domainMatches(c.Domain, h) }) {
			continue
		}
		sc := session.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Secure: c.Secure, HTTPOnly: c.HTTPOnly}
		if !c.Session && c.Expires > 0 {
			sec := int64(c.Expires)
			sc.Expires = time.Unix(sec, int64((c.Expires-float64(sec))*1e9))
		}
		out = append(out, sc)
	}
	return out
}

func jarFromCookies(in []*network.Cookie, want, landed session.Host) *session.Jar {
	j := session.NewJar()
	for _, c := range toSessionCookies(in, cookieHosts(want, landed)) {
		j.Add(c)
	}
	return j
}

// readCookies reads every cookie of the browser's default context.
func readCookies(ctx context.Context) ([]*network.Cookie, error) {
	res, err := chromedp.CallBrowser(ctx, storage.GetCookies, storage.GetCookiesParams{})
	if err != nil {
		return nil, fmt.Errorf("browser: read cookies: %w", err)
	}
	return res.Cookies, nil
}

// LoginInteractive opens a browser window on the dedicated profile at
// https://<h>/ and waits until the human has signed in. It never types
// anything into the page. It returns the session cookies and the host the
// account landed on, and closes the browser. Closing the window first gives
// ErrCancelled; ctx's deadline gives ErrTimeout.
func LoginInteractive(ctx context.Context, o Options, h session.Host) (*session.Jar, session.Host, error) {
	if h == "" {
		h = session.HostDefault
	}
	b, err := Launch(ctx, o)
	if err != nil {
		return nil, "", err
	}
	defer b.Close()
	tab, release, err := b.Tab(ctx)
	if err != nil {
		return nil, "", err
	}
	defer release()

	start := o.startURL
	if start == "" {
		start = h.Origin() + "/"
	}
	closed := watchClosed(b, tab)
	// A failed first navigation is not fatal: the user can still navigate in
	// the window. The probe below notices a closed window.
	_ = chromedp.Do(tab, chromedp.Navigate(start))
	if afterNavigate != nil {
		afterNavigate(b, tab)
	}

	landed, err := waitSignedIn(ctx, func(ctx context.Context) (pageState, error) { return probeTab(ctx, b, tab, closed) }, h, probeEvery)
	if err != nil {
		return nil, "", err
	}
	cs, err := readCookies(tab)
	if err != nil {
		return nil, "", err
	}
	j := jarFromCookies(cs, h, landed)
	if len(j.All()) == 0 {
		return nil, "", errors.New("browser: signed in, but the browser returned no Google cookies")
	}
	return j, landed, nil
}

// probeTab reads the page state of the login tab. It returns errGone when
// the tab or the whole browser was closed.
func probeTab(ctx context.Context, b *Browser, tab context.Context, closed <-chan struct{}) (pageState, error) {
	if err := ctx.Err(); err != nil {
		return pageState{}, err
	}
	isClosed := func() bool {
		select {
		case <-closed:
			return true
		default:
			return b.gone()
		}
	}
	if isClosed() {
		return pageState{}, errGone
	}
	ectx, cancel := context.WithTimeout(tab, 5*time.Second)
	defer cancel()
	// A closed tab or window must not wait out the timeout.
	go func() {
		select {
		case <-b.lost:
		case <-closed:
		case <-ectx.Done():
			return
		}
		cancel()
	}()
	s, err := chromedp.Run(ectx, chromedp.Evaluate[pageState](probeJS))
	if err == nil {
		return s, nil
	}
	if ctx.Err() != nil {
		return pageState{}, ctx.Err()
	}
	if isClosed() || !tabAlive(ctx, b, tab) {
		return pageState{}, errGone
	}
	return pageState{}, err
}

// watchClosed returns a channel that closes when the login tab is destroyed
// (the user closed it) or the browser connection drops. It stops when tab
// ends.
func watchClosed(b *Browser, tab context.Context) <-chan struct{} {
	ch := make(chan struct{})
	c := chromedp.FromContext(tab)
	if c == nil || c.Target == nil {
		close(ch)
		return ch
	}
	id := c.Target.TargetID
	events := chromedp.BrowserEvents(tab, target.TargetDestroyed) // subscribes now
	go func() {
		for ev, err := range events {
			if err != nil {
				if b.gone() {
					close(ch)
				}
				return
			}
			if ev.TargetID == id {
				close(ch)
				return
			}
		}
	}()
	return ch
}

// tabAlive reports whether the login tab is still listed by the browser.
func tabAlive(ctx context.Context, b *Browser, tab context.Context) bool {
	c := chromedp.FromContext(tab)
	if c == nil || c.Target == nil {
		return false
	}
	tctx, cancel := context.WithTimeout(b.tab, 3*time.Second)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	infos, err := chromedp.Targets(tctx)
	if err != nil {
		return false
	}
	return slices.ContainsFunc(infos, func(i *target.Info) bool { return i.TargetID == c.Target.TargetID })
}

// AttachCDP reads the session cookies from a browser that already runs with
// remote debugging on (url is its ws:// or http:// DevTools address). It
// opens no tab and leaves the browser running.
func AttachCDP(ctx context.Context, cdpURL string, h session.Host) (*session.Jar, error) {
	if cdpURL == "" {
		return nil, errors.New("browser: --attach needs a DevTools address such as http://127.0.0.1:9222")
	}
	if h == "" {
		h = session.HostDefault
	}
	actx, cancelAlloc := remote.NewAllocator(ctx, cdpURL)
	defer cancelAlloc()
	bctx, cancelCtx := chromedp.NewContext(actx)
	defer cancelCtx()
	cs, err := readCookies(bctx)
	if err != nil {
		return nil, fmt.Errorf("browser: attach to %s: %w", cdpURL, err)
	}
	j := jarFromCookies(cs, h, h)
	if len(j.All()) == 0 {
		return nil, errors.New("browser: the attached browser has no Google cookies; sign in to NotebookLM in it first")
	}
	return j, nil
}
