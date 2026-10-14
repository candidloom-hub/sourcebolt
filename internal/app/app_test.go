package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/transport/browser"
)

func TestRuntimeCloseShutsDownBrowserLauncher(t *testing.T) {
	rt := ForProfile(testProfile(), nil, WithBrowserFallback(t.TempDir()))
	// Safe before anything launched.
	if err := rt.Close(); err != nil {
		t.Fatalf("Close before launch: %v", err)
	}
	rt = ForProfile(testProfile(), nil, WithBrowserFallback(t.TempDir()))
	if _, err := rt.newBrowser(context.Background()); err != nil {
		t.Fatalf("newBrowser: %v", err)
	}
	l := rt.launcher
	if l == nil {
		t.Fatal("runtime does not own the launcher")
	}
	if err := rt.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := rt.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := l.Get(context.Background()); !errors.Is(err, browser.ErrClosed) {
		t.Fatalf("launcher after Close: err = %v, want ErrClosed", err)
	}
	if _, err := rt.newBrowser(context.Background()); err == nil {
		t.Fatal("newBrowser after Close succeeded")
	}
}

func TestRuntimeCloseCancelsDetachedJobs(t *testing.T) {
	rt := ForProfile(testProfile(), nil)
	stopped := make(chan struct{})
	rt.Deps.Jobs.Start("chat_ask", func(ctx context.Context) (any, error) {
		<-ctx.Done()
		close(stopped)
		return nil, ctx.Err()
	})
	if err := rt.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Runtime.Close left a detached job running")
	}
}

func TestRuntimeCloseWithoutFallback(t *testing.T) {
	rt := ForProfile(testProfile(), nil)
	if err := rt.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// The router must keep chat streaming available to nlm.Client.
var _ nlm.Streamer = (*transport.Router)(nil)

func testProfile() *session.Profile {
	return &session.Profile{Name: "p", Host: session.HostDefault, Jar: session.NewJar()}
}

func TestForProfileWithFallbackUsesRouterOverRPC(t *testing.T) {
	rt := ForProfile(testProfile(), nil, WithBrowserFallback(t.TempDir()))
	if rt.Router == nil || rt.Active != transport.Transport(rt.Router) {
		t.Fatalf("Active = %T, Router = %v", rt.Active, rt.Router)
	}
	if rt.Transport == nil || rt.Router.Name() != "rpc" {
		t.Errorf("primary = %q, want rpc", rt.Router.Name())
	}
	// Nothing launched: the fallback is lazy and the router never built it.
	if rt.Router.Supports("definitely-unknown") && rt.Router.Last() != "" {
		t.Errorf("unexpected activity: last = %q", rt.Router.Last())
	}
}

func TestForProfileWithoutFallbackIsBareRPC(t *testing.T) {
	rt := ForProfile(testProfile(), nil)
	if rt.Router != nil {
		t.Fatal("Router set without fallback")
	}
	if rt.Active != transport.Transport(rt.Transport) {
		t.Fatalf("Active = %T, want the rpc transport", rt.Active)
	}
}

func TestBuildHonoursBrowserFallbackConfig(t *testing.T) {
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", t.TempDir())
	t.Setenv("NOTEBOOKLM_BROWSER_FALLBACK", "")
	cfg, err := ConfigFromEnv()
	if err != nil || !cfg.BrowserFallback {
		t.Fatalf("default BrowserFallback = %v, err = %v", cfg.BrowserFallback, err)
	}
	rt, err := Build(cfg)
	if err != nil || rt.Router == nil {
		t.Fatalf("Build default: Router = %v, err = %v", rt.Router, err)
	}
	t.Setenv("NOTEBOOKLM_BROWSER_FALLBACK", "0")
	cfg, _ = ConfigFromEnv()
	if cfg.BrowserFallback {
		t.Fatal("env 0 did not disable")
	}
	rt, err = Build(cfg)
	if err != nil || rt.Router != nil {
		t.Fatalf("Build disabled: Router = %v, err = %v", rt.Router, err)
	}
}
