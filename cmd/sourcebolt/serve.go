package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/app"
	"github.com/candidloom-hub/sourcebolt/internal/mcpserver"
	"github.com/candidloom-hub/sourcebolt/internal/runtimecfg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func cmdServe(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	profile := fs.String("profile", "", "account profile (default $NOTEBOOKLM_PROFILE or \"default\")")
	tools := fs.String("tools", "default", "tool profile: default, minimal or all")
	httpAddr := fs.String("http", "", "serve streamable HTTP on this loopback address instead of stdio")
	tokenFile := fs.String("token-file", "", "file holding the bearer token required in HTTP mode")
	noBrowser := fs.Bool("no-browser-fallback", false, "do not fall back to a headless browser when the rpc transport fails (also NOTEBOOKLM_BROWSER_FALLBACK=0)")
	if code, ok := parseFlags(fs, args); !ok {
		return code
	}
	defer runtimecfg.Apply()() // the server's GC setting; benchmarks apply the same
	cfg, err := app.ConfigFromEnv()
	if err != nil {
		return fail(stderr, err)
	}
	if *noBrowser {
		cfg.BrowserFallback = false
	}
	if *profile != "" {
		cfg.Profile = *profile
	}
	rt, err := app.Build(cfg)
	if err != nil {
		return fail(stderr, err)
	}
	if rt.ProfileErr != nil {
		// Serve anyway: an MCP client shows tool errors, not stderr.
		fmt.Fprintf(stderr, "sourcebolt: profile %q could not be read (%v); tools report AUTH_REQUIRED until you run: sourcebolt login\n", cfg.Profile, rt.ProfileErr)
	}
	// Stops the fallback browser and detached jobs on every exit path
	// (stdio and HTTP).
	defer rt.Close()
	srv := mcpserver.New(rt.Deps, *tools)

	// Warm tokens and the TLS connection in the background so the first
	// tool call skips the bootstrap; never block initialize on it.
	go func() {
		wctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		_ = rt.Warm(wctx)
	}()

	if *httpAddr != "" {
		return serveHTTP(ctx, srv, *httpAddr, *tokenFile, stderr)
	}
	t := &mcp.IOTransport{Reader: io.NopCloser(stdin), Writer: nopWriteCloser{stdout}}
	if err := srv.Run(ctx, t); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
		return fail(stderr, err)
	}
	return 0
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// serveHTTP serves streamable HTTP on a loopback address only, requiring a
// bearer token and rejecting cross-origin browser requests.
func serveHTTP(ctx context.Context, srv *mcp.Server, addr, tokenFile string, stderr io.Writer) int {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fail(stderr, err)
	}
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fail(stderr, fmt.Errorf("--http must be a loopback address, got %s", addr))
	}
	if tokenFile == "" {
		return fail(stderr, errors.New("--http requires --token-file"))
	}
	raw, err := os.ReadFile(tokenFile)
	if err != nil {
		return fail(stderr, err)
	}
	token := strings.TrimSpace(string(raw))
	if len(token) < 16 {
		return fail(stderr, errors.New("token must be at least 16 characters"))
	}
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	guard := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" && !loopbackOrigin(o) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
	hs := &http.Server{Addr: addr, Handler: guard, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hs.Shutdown(sctx)
	}()
	fmt.Fprintf(stderr, "sourcebolt: streamable HTTP on http://%s\n", addr)
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fail(stderr, err)
	}
	return 0
}

// loopbackOrigin reports whether a browser Origin header names this
// machine: http or https with host localhost or a loopback IP. It parses
// the URL, so localhost.evil.com and localhost@evil.com do not pass.
func loopbackOrigin(o string) bool {
	u, err := url.Parse(o)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return false
	}
	h := u.Hostname()
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
