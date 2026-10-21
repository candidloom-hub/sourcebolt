package app

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/mcpserver"
	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/zalando/go-keyring"
)

// unreadableEnv points the config at a temp dir, the keyring at the mock,
// and the host at a TLS server that counts requests (none may be sent
// while the profile cannot be read).
func unreadableEnv(t *testing.T) (dir string, hits *atomic.Int64) {
	t.Helper()
	keyring.MockInit()
	dir = t.TempDir()
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", dir)
	t.Setenv("NOTEBOOKLM_BROWSER_FALLBACK", "0")
	t.Setenv("NOTEBOOKLM_PROFILE", "")
	t.Setenv("NOTEBOOKLM_RPC_IDS", "")
	t.Setenv(EnvCookies, "")
	t.Setenv(EnvCookiesFile, "")
	hits = new(atomic.Int64)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	t.Setenv("NOTEBOOKLM_HOST", u.Host)
	return dir, hits
}

func profilesDir(dir string) string { return filepath.Join(dir, "profiles") }

func saveDefault(t *testing.T, st session.Store, cookie string) {
	t.Helper()
	j := session.NewJar()
	j.Add(session.Cookie{Name: "SID", Value: cookie, Domain: ".google.com", Path: "/"})
	p := &session.Profile{Name: "default", Host: session.HostDefault, Jar: j}
	if old, err := st.Load("default"); err == nil {
		p.Rev = old.Rev
	}
	if err := st.Save(p); err != nil {
		t.Fatal(err)
	}
}

// wrongKeyProfile leaves a profile file encrypted with key A while the
// keyring holds key B for it.
func wrongKeyProfile(t *testing.T, dir string) {
	t.Helper()
	st := session.NewFileStore(profilesDir(dir), session.OSKeyring{})
	saveDefault(t, st, "secret-cookie-value-A")
	path := filepath.Join(profilesDir(dir), "default.profile")
	blobA, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Delete("default"); err != nil {
		t.Fatal(err)
	}
	saveDefault(t, st, "secret-cookie-value-B") // mints key B
	if err := os.WriteFile(path, blobA, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Load("default"); err == nil || !strings.Contains(err.Error(), "message authentication failed") {
		t.Fatalf("setup: Load err = %v, want a decrypt failure", err)
	}
}

func corruptProfile(t *testing.T, dir string) {
	t.Helper()
	st := session.NewFileStore(profilesDir(dir), session.OSKeyring{})
	saveDefault(t, st, "secret-cookie-value-A")
	if err := os.WriteFile(filepath.Join(profilesDir(dir), "default.profile"), []byte("not a profile"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func missingKeyProfile(t *testing.T, dir string) {
	t.Helper()
	st := session.NewFileStore(profilesDir(dir), session.OSKeyring{})
	saveDefault(t, st, "secret-cookie-value-A")
	if err := keyring.DeleteAll("sourcebolt"); err != nil {
		t.Fatal(err)
	}
}

func connectRuntime(t *testing.T, rt *Runtime) *mcp.ClientSession {
	t.Helper()
	srv := mcpserver.New(rt.Deps, "default")
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func toolEnvelope(t *testing.T, cs *mcp.ClientSession, name string) mcpserver.Envelope {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if !res.IsError || len(res.Content) == 0 {
		t.Fatalf("%s: want an error result, got %+v", name, res)
	}
	var e mcpserver.Envelope
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &e); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	return e
}

// A saved profile that cannot be read must not stop the server: it starts,
// answers initialize and tools/list, and tool calls report AUTH_REQUIRED
// saying the profile could not be read and to run sourcebolt login, the
// same way a server with no profile does. Nothing is sent to Google.
func TestBuildUnreadableProfileStillServes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{"wrong key", wrongKeyProfile},
		{"corrupt file", corruptProfile},
		{"missing key", missingKeyProfile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, hits := unreadableEnv(t)
			tc.setup(t, dir)
			cfg, err := ConfigFromEnv()
			if err != nil {
				t.Fatal(err)
			}
			rt, err := Build(cfg)
			if err != nil {
				t.Fatalf("Build failed instead of serving: %v", err)
			}
			defer rt.Close()
			if rt.ProfileErr == nil {
				t.Fatal("ProfileErr not set for an unreadable profile")
			}
			cs := connectRuntime(t, rt)
			tools, err := cs.ListTools(context.Background(), nil)
			if err != nil || len(tools.Tools) == 0 {
				t.Fatalf("tools/list: %d tools, err = %v", len(tools.Tools), err)
			}
			e := toolEnvelope(t, cs, "notebook_list")
			if e.Code != "AUTH_REQUIRED" {
				t.Fatalf("code = %q, want AUTH_REQUIRED (%+v)", e.Code, e)
			}
			if !strings.Contains(e.Error, "could not be read") || !strings.Contains(e.Error, "sourcebolt login") {
				t.Fatalf("error = %q, want it to say the profile could not be read and to run sourcebolt login", e.Error)
			}
			if !strings.Contains(e.Guidance, "sourcebolt login") {
				t.Fatalf("guidance = %q", e.Guidance)
			}
			if strings.Contains(e.Error, "secret-cookie-value") {
				t.Fatalf("error leaks a cookie value: %q", e.Error)
			}
			var te *transport.Error
			if werr := rt.Warm(context.Background()); !errors.As(werr, &te) || te.Kind != transport.KindAuth {
				t.Fatalf("Warm = %v, want the auth error", werr)
			}
			if n := hits.Load(); n != 0 {
				t.Fatalf("%d requests sent with an unreadable profile", n)
			}
		})
	}
}

// After `sourcebolt login` replaces the unreadable profile, the running
// server picks it up on the next call without a restart.
func TestUnreadableProfileRecoversAfterLogin(t *testing.T) {
	dir, _ := unreadableEnv(t)
	wrongKeyProfile(t, dir)
	cfg, _ := ConfigFromEnv()
	rt, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer rt.Close()
	cs := connectRuntime(t, rt)
	if e := toolEnvelope(t, cs, "notebook_list"); !strings.Contains(e.Error, "could not be read") {
		t.Fatalf("first call: %+v", e)
	}
	// What login does: delete the unreadable profile and save a new one.
	st := session.NewFileStore(profilesDir(dir), session.OSKeyring{})
	if err := st.Delete("default"); err != nil {
		t.Fatal(err)
	}
	saveDefault(t, st, "fresh-cookie")
	if e := toolEnvelope(t, cs, "notebook_list"); strings.Contains(e.Error, "could not be read") {
		t.Fatalf("still unreadable after login: %+v", e)
	}
	if rt.Profile.Rev == 0 || len(rt.Profile.Jar.All()) == 0 {
		t.Fatalf("new login not adopted: rev = %d, cookies = %d", rt.Profile.Rev, len(rt.Profile.Jar.All()))
	}
}
