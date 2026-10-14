package mcpserver

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/bench/benchdata"
	"github.com/candidloom-hub/sourcebolt/internal/cache"
	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// benchTransport answers every call with one fixed payload and records
// nothing, so it adds no overhead of its own.
type benchTransport struct{ payload []byte }

func (benchTransport) Name() string         { return "rpc" }
func (benchTransport) Supports(string) bool { return true }
func (t benchTransport) Do(context.Context, transport.Call) (transport.Result, error) {
	return transport.Result{Payload: t.payload, Transport: "rpc"}, nil
}

func benchSession(b *testing.B) *mcp.ClientSession {
	b.Helper()
	// 100 notebooks built from the live wXbhsf row structure (synthetic ids).
	tr := benchTransport{payload: benchdata.ListPayload(100)}
	srv := New(Deps{NLM: nlm.New(tr), Cache: cache.New(nil), Profile: "default", Host: "notebook.google.com"}, "default")
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		b.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "bench", Version: "v0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { cs.Close() })
	return cs
}

// BenchmarkToolCallNotebookList is B3: tool-call overhead excluding Google
// (in-memory MCP transport, stub upstream, 100 notebooks). "refresh" bypasses
// the cache so the whole path runs: schema validation, handler, nlm decode,
// filter, JSON result. "cached" is the steady state inside the 30 s TTL.
// p50-ns and p95-ns are per-call latencies (targets: 2 ms and 5 ms).
func BenchmarkToolCallNotebookList(b *testing.B) {
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"refresh", map[string]any{"refresh": true}},
		{"cached", nil},
	} {
		b.Run(tc.name, func(b *testing.B) {
			cs := benchSession(b)
			ctx := context.Background()
			p := &mcp.CallToolParams{Name: "notebook_list", Arguments: tc.args}
			if res, err := cs.CallTool(ctx, p); err != nil || res.IsError { // warm up, check
				b.Fatalf("warm-up: %v %v", err, res)
			}
			lat := make([]time.Duration, 0, 1024)
			b.ReportAllocs()
			for b.Loop() {
				t0 := time.Now()
				res, err := cs.CallTool(ctx, p)
				lat = append(lat, time.Since(t0))
				if err != nil || res.IsError {
					b.Fatalf("call: %v %v", err, res)
				}
			}
			slices.Sort(lat)
			b.ReportMetric(float64(lat[len(lat)/2].Nanoseconds()), "p50-ns")
			b.ReportMetric(float64(lat[min(len(lat)*95/100, len(lat)-1)].Nanoseconds()), "p95-ns")
		})
	}
}
