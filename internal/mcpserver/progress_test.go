package mcpserver

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestProgressFromContext(t *testing.T) {
	m := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "v"}, nil)
	s := &server{toolProfile: "all"}
	register(s, m, "core", &mcp.Tool{Name: "work"}, func(ctx context.Context, _ struct{}) (struct{}, error) {
		p := progressFrom(ctx)
		p("half", 1, 2)
		p("done", 2, 2)
		return struct{}{}, nil
	})
	var mu sync.Mutex
	var got []string
	ct, st := mcp.NewInMemoryTransports()
	if _, err := m.Connect(context.Background(), st, nil); err != nil {
		t.Fatal(err)
	}
	c := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "v"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) {
			mu.Lock()
			got = append(got, r.Params.Message)
			mu.Unlock()
		},
	})
	cs, err := c.Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	params := &mcp.CallToolParams{Name: "work", Arguments: map[string]any{}}
	params.SetProgressToken("tok-1")
	if _, err := cs.CallTool(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 || got[0] != "half" || got[1] != "done" {
		t.Fatalf("progress messages = %v", got)
	}
}

func TestProgressFromContextWithoutTokenIsNoop(t *testing.T) {
	progressFrom(context.Background())("x", 1, 1) // must not panic
}
