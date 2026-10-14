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
	Store     session.Store
}

// Build loads the profile (or an empty one, so tools report AUTH_REQUIRED
// instead of the server failing to start) and wires every layer.
func Build(c Config) (*Runtime, error) {
	if err := loadRPCOverrides(c); err != nil {
		return nil, err
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
	var opts []Option
	if c.BrowserFallback {
		opts = append(opts, WithBrowserFallback(c.ConfigDir))
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
			bo := browser.FetchOptions(o.browserConfigDir, p.Name)
			host := p.Host
			newBrowser = func(context.Context) (transport.Transport, error) {
				return browser.NewTransport(browser.NewLauncher(bo).Get, host), nil
			}
		}
		rt.Router = transport.NewRouter(tr, newBrowser)
		rt.Active = rt.Router
	}
	rt.Deps = mcpserver.Deps{NLM: nlm.New(rt.Active), Cache: cache.New(nil), Jobs: jobs.New(), Profile: p.Name, Host: p.Host}
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
