// Package app wires configuration, the profile store, transports, the
// domain client and the MCP server together for the CLI.
package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/candidloom-hub/sourcebolt/internal/cache"
	"github.com/candidloom-hub/sourcebolt/internal/jobs"
	"github.com/candidloom-hub/sourcebolt/internal/mcpserver"
	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/rpcids"
	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport/rpc"
)

// Config comes from flags and environment.
type Config struct {
	ConfigDir string       // NOTEBOOKLM_CONFIG_DIR, default <user config>/sourcebolt
	Profile   string       // NOTEBOOKLM_PROFILE, default "default"
	Host      session.Host // NOTEBOOKLM_HOST overrides the profile's host
	RPCIDs    string       // NOTEBOOKLM_RPC_IDS, default <config>/rpc-ids.json if present
}

// ConfigFromEnv reads the NOTEBOOKLM_* variables.
func ConfigFromEnv() (Config, error) {
	c := Config{
		ConfigDir: os.Getenv("NOTEBOOKLM_CONFIG_DIR"),
		Profile:   os.Getenv("NOTEBOOKLM_PROFILE"),
		Host:      session.Host(os.Getenv("NOTEBOOKLM_HOST")),
		RPCIDs:    os.Getenv("NOTEBOOKLM_RPC_IDS"),
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
	Transport *rpc.Transport
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
	return ForProfile(p, store), nil
}

// ForProfile wires a runtime around an in-memory profile.
func ForProfile(p *session.Profile, store session.Store) *Runtime {
	tr := rpc.New(rpc.Options{Profile: p, Store: store, MaxRetries: 2})
	return &Runtime{
		Deps:      mcpserver.Deps{NLM: nlm.New(tr), Cache: cache.New(nil), Jobs: jobs.New(), Profile: p.Name, Host: p.Host},
		Transport: tr,
		Profile:   p,
		Store:     store,
	}
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
