// Command ttfp measures B9: streamed chat time-to-first-progress, the delay
// between Google's first answer chunk and the MCP client's first progress
// notification for chat_ask.
//
//	go run ./bench/ttfp [-runs 30] [-check]
//
// Method. As in bench/rss, the production stack (rpc transport, nlm client,
// MCP server) runs in this process against a local TLS HTTP/2 fake. The fake
// answers batchexecute reads from the live fixtures and streams the live
// GenerateFreeFormStreamed capture: it flushes everything up to and including
// the first chunk that carries answer text, notes the time, pauses, then
// sends the rest. The MCP client (in-memory transport, progress token set)
// notes when the first progress notification arrives. The difference is the
// server's own overhead; Google's time to that chunk is excluded by
// construction. The first run is dropped (token bootstrap, TLS handshake).
package main

import (
	"context"
	"encoding/json/v2"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/candidloom-hub/sourcebolt/bench/benchdata"
	"github.com/candidloom-hub/sourcebolt/internal/cache"
	"github.com/candidloom-hub/sourcebolt/internal/mcpserver"
	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/runtimecfg"
	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport/rpc"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/time/rate"
)

const (
	target   = 20 * time.Millisecond
	notebook = "00000000-0000-4000-8000-000000000001"
	conv     = "00000000-0000-4000-8000-0000000000c0"
	pause    = 100 * time.Millisecond
)

func main() {
	runs := flag.Int("runs", 30, "number of asks; the first is dropped")
	check := flag.Bool("check", false, "exit 1 unless p50 <= 20 ms")
	flag.Parse()
	if *runs < 2 {
		flag.Usage()
		os.Exit(2)
	}
	lat, err := measure(*runs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ttfp:", err)
		os.Exit(1)
	}
	lat = lat[1:]
	slices.Sort(lat)
	p50, p95 := lat[len(lat)/2], lat[min(len(lat)*95/100, len(lat)-1)]
	fmt.Printf("B9 chat time-to-first-progress after Google's first answer chunk (%d asks, first dropped)\n", len(lat))
	fmt.Printf("min=%s p50=%s p95=%s max=%s\n", ms(lat[0]), ms(p50), ms(p95), ms(lat[len(lat)-1]))
	fmt.Printf("target: p50 <= first chunk + %s\n", ms(target))
	if *check && p50 > target {
		fmt.Println("FAIL")
		os.Exit(1)
	}
}

func ms(d time.Duration) string { return fmt.Sprintf("%.2f ms", float64(d)/float64(time.Millisecond)) }

func fixture(rpcID string) ([]byte, error) {
	files, err := filepath.Glob(filepath.Join(benchdata.Root(), "testdata", "fixtures", rpcID, "live_*.json"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no live fixture for %s", rpcID)
	}
	raw, err := os.ReadFile(files[len(files)-1]) // newest capture
	if err != nil {
		return nil, err
	}
	var fx struct {
		Body string `json:"response_body"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		return nil, err
	}
	return []byte(fx.Body), nil
}

// splitFirstAnswer cuts body after the first top-level chunk whose snapshot
// carries answer text (not thinking), so that chunk is what Google sends first.
func splitFirstAnswer(body []byte) (first, rest []byte, err error) {
	depth, inStr, esc := 0, false, false
	for i, c := range body {
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = depth > 0
		case '[':
			depth++
		case ']':
			if depth--; depth == 0 && answers(body[:i+1]) {
				return body[:i+1], body[i+1:], nil
			}
		}
	}
	return nil, nil, fmt.Errorf("stream fixture has no answer text")
}

func answers(prefix []byte) bool {
	d := wire.NewStreamDecoder(strings.NewReader(string(prefix)))
	for {
		s, err := d.Next()
		if err != nil {
			return false
		}
		if !s.Thinking && s.Text != "" {
			return true
		}
	}
}

func measure(runs int) ([]time.Duration, error) {
	page, err := os.ReadFile(filepath.Join(benchdata.Root(), "testdata", "fixtures", "_page", "home_dq.html"))
	if err != nil {
		return nil, err
	}
	stream, err := fixture("GenerateFreeFormStreamed")
	if err != nil {
		return nil, err
	}
	first, rest, err := splitFirstAnswer(stream)
	if err != nil {
		return nil, err
	}
	reads := map[string][]byte{}
	for _, id := range []string{"rLM1Ne", "hPTbtc"} {
		if reads[id], err = fixture(id); err != nil {
			return nil, err
		}
	}

	var mu sync.Mutex
	var sent time.Time // just before the first answer chunk is written
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.Write(page)
		case strings.HasSuffix(r.URL.Path, "/GenerateFreeFormStreamed"):
			fl := w.(http.Flusher)
			mu.Lock()
			sent = time.Now() // conservative: the write and flush count as ours
			mu.Unlock()
			w.Write(first)
			fl.Flush()
			time.Sleep(pause)
			w.Write(rest)
		default:
			w.Write(reads[r.URL.Query().Get("rpcids")])
		}
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	jar := session.NewJar()
	jar.Add(session.Cookie{Name: "SID", Value: "bench", Domain: "127.0.0.1", Path: "/"})
	p := &session.Profile{Name: "bench", Host: session.Host(srv.Listener.Addr().String()), Jar: jar}
	tr := rpc.New(rpc.Options{Profile: p, HTTP: srv.Client(), Limiter: rate.NewLimiter(rate.Inf, 1)})
	defer runtimecfg.Apply()() // as the served binary
	ms := mcpserver.New(mcpserver.Deps{NLM: nlm.New(tr), Cache: cache.New(nil), Profile: "bench", Host: p.Host}, "default")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ct, st := mcp.NewInMemoryTransports()
	if _, err := ms.Connect(ctx, st, nil); err != nil {
		return nil, err
	}
	got := make(chan time.Time, 64)
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "ttfp", Version: "v0"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(context.Context, *mcp.ProgressNotificationClientRequest) {
			select {
			case got <- time.Now():
			default:
			}
		},
	}).Connect(ctx, ct, nil)
	if err != nil {
		return nil, err
	}
	defer cs.Close()

	var lat []time.Duration
	for i := range runs {
		for len(got) > 0 {
			<-got
		}
		params := &mcp.CallToolParams{Name: "chat_ask", Arguments: map[string]any{
			"notebook": notebook, "question": "What is this about?", "conversation_id": conv,
		}}
		params.SetProgressToken(fmt.Sprintf("ask-%d", i))
		res, err := cs.CallTool(ctx, params)
		if err != nil {
			return nil, err
		}
		if res.IsError {
			return nil, fmt.Errorf("chat_ask failed: %v", res.Content[0])
		}
		var at time.Time
		select {
		case at = <-got:
		default:
			return nil, fmt.Errorf("ask %d: no progress notification", i)
		}
		mu.Lock()
		lat = append(lat, at.Sub(sent))
		mu.Unlock()
	}
	return lat, nil
}
