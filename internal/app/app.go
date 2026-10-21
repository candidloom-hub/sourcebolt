// Package app wires configuration, the profile store, transports, the
// domain client and the MCP server together for the CLI.
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Config comes from flags and environment.
type Config struct {
	ConfigDir string       // NOTEBOOKLM_CONFIG_DIR, default %USERPROFILE%\.sourcebolt (Windows) or <user config>/sourcebolt
	Profile   string       // NOTEBOOKLM_PROFILE, default "default"
	Host      session.Host // NOTEBOOKLM_HOST overrides the profile's host
	RPCIDs    string       // NOTEBOOKLM_RPC_IDS, default <config>/rpc-ids.json if present
	// BrowserFallback enables the lazy headless-browser fallback transport.
	// Default true; NOTEBOOKLM_BROWSER_FALLBACK=0 (or false/off/no) disables.
	BrowserFallback bool

	// legacyDir is the old Windows folder (%APPDATA%\sourcebolt) to
	// migrate profiles from; empty when NOTEBOOKLM_CONFIG_DIR is set.
	legacyDir string
}

func browserFallbackFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("NOTEBOOKLM_BROWSER_FALLBACK"))) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

// dirEnv resolves the default folders; tests replace it.
var dirEnv = session.OSDirEnv

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
		e := dirEnv()
		dir, err := e.ConfigDir()
		if err != nil {
			return c, err
		}
		c.ConfigDir = dir
		c.legacyDir = e.LegacyConfigDir() // only for the default folder
	}
	if c.Profile == "" {
		c.Profile = "default"
	}
	return c, nil
}

// Store opens the encrypted profile store. With the default folder on
// Windows it first copies profiles that are only in the old %APPDATA%
// folder (see session.MigrateProfiles); the old folder is left as it is.
func (c Config) Store() session.Store {
	dir := filepath.Join(c.ConfigDir, "profiles")
	if c.legacyDir != "" {
		old := filepath.Join(c.legacyDir, "profiles")
		names, _ := session.MigrateProfiles(old, dir, session.OSKeyring{})
		for _, n := range names {
			fmt.Fprintf(os.Stderr, "sourcebolt: copied profile %q from the old data folder %s to %s (the old folder was not changed)\n", n, c.legacyDir, c.ConfigDir)
		}
	}
	return session.NewFileStore(dir, session.OSKeyring{})
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
	// ProfileErr is why the saved profile could not be read (wrong or
	// missing key, corrupt file); nil otherwise. The server still starts
	// and tool calls report AUTH_REQUIRED until `sourcebolt login`
	// replaces the profile.
	ProfileErr error
	gate       *profileGate // set with ProfileErr

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

// Warm fetches page tokens ahead of the first tool call. While the saved
// profile cannot be read it sends nothing and returns the auth error.
func (rt *Runtime) Warm(ctx context.Context) error {
	if rt.gate != nil {
		if err := rt.gate.check(); err != nil {
			return err
		}
	}
	return rt.Transport.Warm(ctx)
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
	var profileErr error
	if err != nil {
		// The server starts either way, so an MCP client sees tools and an
		// AUTH_REQUIRED that says what to do instead of "connection closed".
		if !errors.Is(err, fs.ErrNotExist) || profileExists(store, c.Profile) {
			profileErr = err // saved but unreadable: wrong or missing key, damaged file, locked keyring
		}
		p = &session.Profile{Name: c.Profile, Host: session.HostDefault, Jar: session.NewJar()}
	}
	if c.Host != "" {
		p.Host = c.Host
	}
	if p.Host == "" {
		p.Host = session.HostDefault
	}
	rt := ForProfile(p, store, opts...)
	if profileErr != nil {
		rt.ProfileErr = profileErr
		g := &profileGate{rt: rt, store: store, name: c.Profile, err: profileErr, next: rt.Active}
		rt.gate, rt.Active = g, g
		dom := rt.Deps.NLM.DOM
		rt.Deps.NLM = nlm.New(g)
		rt.Deps.NLM.DOM = dom
	}
	return rt, nil
}

func profileExists(store session.Store, name string) bool {
	names, err := store.List()
	return err == nil && slices.Contains(names, name)
}

// profileGate stands in front of the transports while the saved profile
// cannot be read. Each call retries the load: while it fails the call gets
// AUTH_REQUIRED without anything being sent; once `sourcebolt login` has
// replaced the profile, the rpc transport adopts it and the gate opens for
// good.
type profileGate struct {
	rt    *Runtime
	store session.Store
	name  string

	mu   sync.Mutex
	err  error // nil once open
	next transport.Transport
}

// check returns the auth error while the profile is still unreadable.
func (g *profileGate) check() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.err == nil {
		return nil
	}
	if _, err := g.store.Load(g.name); err != nil {
		g.err = err
		return g.authErr(err)
	}
	g.err = nil
	g.rt.Transport.ReloadIfNewer()
	return nil
}

// authErr names the failure without cookie values or key material (store
// errors carry neither) and says how to fix it.
func (g *profileGate) authErr(err error) error {
	return &transport.Error{
		Kind:  transport.KindAuth,
		RPC:   "session.load",
		Cause: fmt.Errorf("the saved profile %q could not be read (%v); run: sourcebolt login to replace it", g.name, err),
	}
}

func (g *profileGate) Name() string { return g.next.Name() }

func (g *profileGate) Supports(rpc string) bool { return g.next.Supports(rpc) }

func (g *profileGate) Do(ctx context.Context, c transport.Call) (transport.Result, error) {
	if err := g.check(); err != nil {
		return transport.Result{}, err
	}
	return g.next.Do(ctx, c)
}

// Stream keeps chat working through the gate (nlm needs a Streamer).
func (g *profileGate) Stream(ctx context.Context, nb string, params any, onSnap func(wire.Snapshot)) (wire.Snapshot, error) {
	if err := g.check(); err != nil {
		return wire.Snapshot{}, err
	}
	st, ok := g.next.(nlm.Streamer)
	if !ok {
		return wire.Snapshot{}, &transport.Error{Kind: transport.KindUnsupported, RPC: "chat.ask", Transport: g.next.Name(), Message: "transport cannot stream chat answers"}
	}
	return st.Stream(ctx, nb, params, onSnap)
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
