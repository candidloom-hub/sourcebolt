// Package app wires configuration, the profile store, transports, the
// domain client and the MCP server together for the CLI.
package app

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/candidloom-hub/sourcebolt/internal/cache"
	"github.com/candidloom-hub/sourcebolt/internal/jobs"
	"github.com/candidloom-hub/sourcebolt/internal/mcpserver"
	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/rpcids"
	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/transport/browser"
	"github.com/candidloom-hub/sourcebolt/internal/transport/rpc"
)

// Config comes from flags and environment.
type Config struct {
	ConfigDir string       // NOTEBOOKLM_CONFIG_DIR, default <user config>/sourcebolt
	Profile   string       // NOTEBOOKLM_PROFILE, default "default"
	Host      session.Host // NOTEBOOKLM_HOST overrides the profile's host
	RPCIDs    string       // NOTEBOOKLM_RPC_IDS, default <config>/rpc-ids.json if present
	// BrowserFallback enables the lazy headless-browser fallback transport.
	// Default true; NOTEBOOKLM_BROWSER_FALLBACK=0 (or false/off/no) disables.
	BrowserFallback bool
}

func browserFallbackFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("NOTEBOOKLM_BROWSER_FALLBACK"))) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

// ConfigFromEnv reads the NOTEBOOKLM_* variables.
func ConfigFromEnv() (Config, error) {
	c := Config{
		ConfigDir: os.Getenv("NOTEBOOKLM_CONFIG_DIR"),
		Profile:   os.Getenv("NOTEBOOKLM_PROFILE"),
		Host:      session.Host(os.Getenv("NOTEBOOKLM_HOST")),
		RPCIDs:    os.Getenv("NOTEBOOKLM_RPC_IDS"),

		BrowserFallback: browserFallbackFromEnv(),
	}
	if c.ConfigDir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return c, err
		}
		c.ConfigDir = filepath.Join(base, "sourcebolt")
	}
	if c.Profile == "" {
		c.Profile = "default"
	}
	return c, nil
}

// Store opens the encrypted profile store.
func (c Config) Store() session.Store {
	return session.NewFileStore(filepath.Join(c.ConfigDir, "profiles"), session.OSKeyring{})
}

// Runtime is a fully wired server for one profile.
type Runtime struct {
	Deps      mcpserver.Deps
	Transport *rpc.Transport      // the rpc transport (Warm, doctor)
	Active    transport.Transport // what the nlm client calls: Router, or Transport when fallback is off
	Router    *transport.Router   // nil when the browser fallback is disabled
	Profile   *session.Profile
	Store     session.Store // nil for env cookies: nothing is written to disk
	// CookieSource names the environment variable the cookies came from
	// (EnvCookies or EnvCookiesFile); empty for a saved profile.
	CookieSource string

	// The fallback browser launcher is created on first use but owned
	// here, so Close can stop a headless browser that would otherwise
	// outlive the process and keep the profile dir locked.
	mu          sync.Mutex
	browserOpts browser.Options
	launcher    *browser.Launcher
	closed      bool
}

// newBrowser builds the fallback transport around the runtime's launcher.
// The launcher starts no browser until the first call.
func (rt *Runtime) newBrowser(context.Context) (transport.Transport, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	l, err := rt.sharedLauncher()
	if err != nil {
		return nil, err
	}
	return browser.NewTransport(l.Get, rt.Profile.Host), nil
}

// sharedLauncher returns the runtime's launcher, creating it on first use.
// Caller holds rt.mu.
func (rt *Runtime) sharedLauncher() (*browser.Launcher, error) {
	if rt.closed {
		return nil, browser.ErrClosed
	}
	if rt.launcher == nil {
		rt.launcher = browser.NewLauncher(rt.browserOpts)
	}
	return rt.launcher, nil
}

// lazyDOM is the nlm DOM fallback. It builds browser.DOM on first use, on
// the runtime's launcher (shared with the browser transport), so nothing
// starts before an RPC drift asks for it.
type lazyDOM struct {
	rt  *Runtime
	sel browser.Selectors

	mu sync.Mutex
	d  *browser.DOM
}

func (l *lazyDOM) get() (*browser.DOM, error) {
	l.rt.mu.Lock()
	launcher, err := l.rt.sharedLauncher()
	l.rt.mu.Unlock()
	if err != nil {
		// Typed like the DOM's own errors; nothing was sent.
		return nil, &transport.Error{Kind: transport.KindTransport, Transport: nlm.DOMTransport, Cause: err}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.d == nil {
		l.d = browser.NewDOM(launcher.Get, l.rt.Profile.Host, l.sel)
	}
	return l.d, nil
}

func (l *lazyDOM) Ask(ctx context.Context, nb, question string, progress func(string)) (string, error) {
	d, err := l.get()
	if err != nil {
		return "", err
	}
	return d.Ask(ctx, nb, question, progress)
}

func (l *lazyDOM) AddURL(ctx context.Context, nb, url string) error {
	d, err := l.get()
	if err != nil {
		return err
	}
	return d.AddURL(ctx, nb, url)
}

func (l *lazyDOM) AddText(ctx context.Context, nb, title, content string) error {
	d, err := l.get()
	if err != nil {
		return err
	}
	return d.AddText(ctx, nb, title, content)
}

// Close cancels running detached jobs and stops the fallback browser (if
// one was ever launched). It is idempotent and safe on a runtime that
// never used the browser.
func (rt *Runtime) Close() error {
	rt.mu.Lock()
	if rt.closed {
		rt.mu.Unlock()
		return nil
	}
	rt.closed = true
	l := rt.launcher
	rt.mu.Unlock()
	if rt.Deps.Jobs != nil {
		rt.Deps.Jobs.Close()
	}
	if l != nil {
		return l.Close()
	}
	return nil
}

// Build loads the profile (or an empty one, so tools report AUTH_REQUIRED
// instead of the server failing to start) and wires every layer.
func Build(c Config) (*Runtime, error) {
	if err := loadRPCOverrides(c); err != nil {
		return nil, err
	}
	var opts []Option
	if c.BrowserFallback {
		opts = append(opts, WithBrowserFallback(c.ConfigDir))
	}

	// Env cookies win over a saved profile whenever they are set. Setting
	// them is an explicit choice for this one process, while a saved
	// profile is ambient state that may be older than the export; a
	// container has no saved profile anyway. The saved profile is never
	// read or written in this mode (no store: rotated cookies stay in
	// memory and the secret is never persisted), so unsetting the
	// variable brings the saved profile back unchanged.
	envP, src, err := EnvProfile(c)
	if err != nil {
		return nil, err
	}
	if envP != nil {
		rt := ForProfile(envP, nil, opts...)
		rt.CookieSource = src
		return rt, nil
	}

	store := c.Store()
	p, err := store.Load(c.Profile)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		p = &session.Profile{Name: c.Profile, Host: session.HostDefault, Jar: session.NewJar()}
	case err != nil:
		return nil, err
	}
	if c.Host != "" {
		p.Host = c.Host
	}
	if p.Host == "" {
		p.Host = session.HostDefault
	}
	return ForProfile(p, store, opts...), nil
}

// Option customizes ForProfile.
type Option func(*options)

type options struct {
	browserConfigDir string
	browserFallback  bool
	newBrowser       func(ctx context.Context) (transport.Transport, error) // tests
}

// WithBrowserFallback adds the lazy browser fallback; its profile dir is
// <configDir>/browser/<profile>. No browser starts before the first call
// that the fallback policy allows.
func WithBrowserFallback(configDir string) Option {
	return func(o *options) { o.browserConfigDir, o.browserFallback = configDir, true }
}

// ForProfile wires a runtime around an in-memory profile. Without options
// the nlm client uses the bare rpc transport.
func ForProfile(p *session.Profile, store session.Store, opts ...Option) *Runtime {
	var o options
	for _, f := range opts {
		f(&o)
	}
	tr := rpc.New(rpc.Options{Profile: p, Store: store, MaxRetries: 2})
	rt := &Runtime{Transport: tr, Active: tr, Profile: p, Store: store}
	if o.browserFallback {
		newBrowser := o.newBrowser
		if newBrowser == nil {
			rt.browserOpts = browser.FetchOptions(o.browserConfigDir, p.Name)
			newBrowser = rt.newBrowser
		}
		rt.Router = transport.NewRouter(tr, newBrowser)
		rt.Active = rt.Router
	}
	rt.Deps = mcpserver.Deps{NLM: nlm.New(rt.Active), Cache: cache.New(nil), Jobs: jobs.New(), Profile: p.Name, Host: p.Host}
	if o.browserFallback && o.newBrowser == nil {
		// Chat and add-source fall back to driving the page on RPC drift.
		// A bad <config>/selectors.json logs one line and keeps the defaults.
		rt.Deps.NLM.DOM = &lazyDOM{rt: rt, sel: browser.LoadSelectors(o.browserConfigDir, os.Stderr)}
	}
	return rt
}

func loadRPCOverrides(c Config) error {
	path := c.RPCIDs
	if path == "" {
		def := filepath.Join(c.ConfigDir, "rpc-ids.json")
		if _, err := os.Stat(def); err != nil {
			return nil
		}
		path = def
	}
	return rpcids.LoadOverrides(path)
}
