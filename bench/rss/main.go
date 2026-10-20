// Command rss measures B2: peak RSS after listing 100 notebooks in RPC mode.
//
//	go run ./bench/rss [-bin bin/sourcebolt.exe]
//
// Method. The real binary cannot be pointed at a fake upstream: it speaks
// HTTPS and trusts only the system roots, and trusting a test CA would mean
// changing machine trust settings. So the 100-notebook measurement runs the
// production stack (rpc transport, nlm client, cache, MCP server) inside this
// process against a local TLS HTTP/2 fake that serves the live wXbhsf
// fixture structure expanded to 100 notebooks. The process also holds the MCP
// client and the fake server, so the figure over-states the real server.
//
// With -bin, the spawned binary is also measured idle (after initialize and
// tools/list, no upstream), which is the floor the in-process number sits on.
//
// Linux reads VmHWM from /proc/<pid>/status; Windows calls psapi
// GetProcessMemoryInfo (PeakWorkingSetSize). macOS is skipped.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/candidloom-hub/sourcebolt/bench/benchdata"
	"github.com/candidloom-hub/sourcebolt/internal/cache"
	"github.com/candidloom-hub/sourcebolt/internal/mcpserver"
	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport/rpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/time/rate"
)

const targetBytes = 25 << 20

func main() {
	bin := flag.String("bin", "", "also measure this binary idle after initialize")
	notebooks := flag.Int("notebooks", 100, "notebooks in the fake list")
	check := flag.Bool("check", false, "exit 1 unless peak RSS < 25 MB")
	flag.Parse()
	if !supported {
		fmt.Printf("B2 skipped: RSS measurement is not implemented on %s\n", runtime.GOOS)
		return
	}
	if *bin != "" {
		b, err := idleBinary(*bin)
		if err != nil {
			fmt.Fprintln(os.Stderr, "idle binary:", err)
			os.Exit(1)
		}
		fmt.Printf("binary idle (after initialize + tools/list): peak RSS %s\n", mb(b))
	}
	before, after, err := listInProcess(*notebooks)
	if err != nil {
		fmt.Fprintln(os.Stderr, "in-process:", err)
		os.Exit(1)
	}
	fmt.Printf("B2 peak RSS, production stack in-process + fake upstream + MCP client\n")
	fmt.Printf("before list (setup only): %s\n", mb(before))
	fmt.Printf("after listing %d notebooks: %s\n", *notebooks, mb(after))
	fmt.Println("target: < 25 MB")
	if *check && after >= targetBytes {
		fmt.Println("FAIL")
		os.Exit(1)
	}
}

func listInProcess(n int) (before, after int64, err error) {
	root := benchdata.Root()
	page, err := os.ReadFile(filepath.Join(root, "testdata", "fixtures", "_page", "home_dq.html"))
	if err != nil {
		return 0, 0, err
	}
	body := benchdata.ListBody(n)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Write(page)
			return
		}
		w.Write(body)
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	jar := session.NewJar()
	jar.Add(session.Cookie{Name: "SID", Value: "bench", Domain: "127.0.0.1", Path: "/"})
	p := &session.Profile{Name: "bench", Host: session.Host(srv.Listener.Addr().String()), Jar: jar}
	tr := rpc.New(rpc.Options{Profile: p, HTTP: srv.Client(), Limiter: rate.NewLimiter(rate.Inf, 1)})
	ms := mcpserver.New(mcpserver.Deps{NLM: nlm.New(tr), Cache: cache.New(nil), Profile: "bench", Host: p.Host}, "default")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ct, st := mcp.NewInMemoryTransports()
	if _, err := ms.Connect(ctx, st, nil); err != nil {
		return 0, 0, err
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "rss", Version: "v0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		return 0, 0, err
	}
	defer cs.Close()

	if before, err = peakRSS(0); err != nil {
		return 0, 0, err
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "notebook_list", Arguments: map[string]any{"limit": n}})
	if err != nil {
		return 0, 0, err
	}
	if res.IsError {
		return 0, 0, fmt.Errorf("notebook_list failed: %v", res.Content)
	}
	after, err = peakRSS(0)
	return before, after, err
}

func idleBinary(bin string) (int64, error) {
	dir, err := os.MkdirTemp("", "sb-rss-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	cmd := exec.Command(bin, "serve")
	cmd.Env = append(os.Environ(), "NOTEBOOKLM_CONFIG_DIR="+dir, "NOTEBOOKLM_HOST=127.0.0.1:1")
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	defer func() { in.Close(); cmd.Process.Kill(); cmd.Wait() }()
	r := bufio.NewReader(out)
	for _, msg := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"rss","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	} {
		if _, err := io.WriteString(in, msg+"\n"); err != nil {
			return 0, err
		}
		if msg[len(msg)-1] == '}' && !strings.Contains(msg, "notifications/") {
			if _, err := r.ReadString('\n'); err != nil {
				return 0, err
			}
		}
	}
	time.Sleep(200 * time.Millisecond) // let the background warm-up fail and settle
	if d := rssDetail(cmd.Process.Pid); d != "" {
		fmt.Println("binary idle detail:", d)
	}
	return peakRSS(cmd.Process.Pid)
}

func mb(b int64) string { return fmt.Sprintf("%.1f MB", float64(b)/(1<<20)) }
