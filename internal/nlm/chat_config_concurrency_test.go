package nlm

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

// settingsServer stores one notebook's chat settings: rLM1Ne reads them,
// s0tc2d replaces them. A read holds briefly so that a concurrent
// Configure can slip in between another's read and write unless they are
// serialized. inflight counts Configure calls between read and write.
type settingsServer struct {
	mu       sync.Mutex
	settings any // [[goal(,prompt)],[length]]
	inflight int
	peak     int
}

func (s *settingsServer) Name() string         { return "fake" }
func (s *settingsServer) Supports(string) bool { return true }

func (s *settingsServer) Do(ctx context.Context, c transport.Call) (transport.Result, error) {
	switch c.ID {
	case "rLM1Ne":
		s.mu.Lock()
		s.inflight++
		s.peak = max(s.peak, s.inflight)
		row := []any{"", []any{}, c.Params.([]any)[0], "", nil, nil, nil, s.settings}
		s.mu.Unlock()
		time.Sleep(30 * time.Millisecond)
		b, _ := json.Marshal([]any{row})
		return transport.Result{Payload: jsontext.Value(b), Transport: "fake"}, nil
	case "s0tc2d":
		s.mu.Lock()
		s.settings = c.Params.([]any)[1].([]any)[0].([]any)[7]
		s.inflight--
		s.mu.Unlock()
		return transport.Result{Payload: jsontext.Value(`[]`), Transport: "fake"}, nil
	}
	return transport.Result{}, fmt.Errorf("unexpected call %s", c.ID)
}

// Two partial changes on one notebook at once must both survive: the
// read-modify-write of Configure runs one at a time per notebook.
func TestConfigureSerializedPerNotebook(t *testing.T) {
	srv := &settingsServer{settings: []any{[]any{2, "p"}, []any{4}}}
	c := New(srv)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, cfg := range []ChatConfig{{Goal: "learning_guide"}, {Length: "shorter"}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = c.Configure(context.Background(), fakeNB, cfg)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, _ := json.Marshal(srv.settings)
	if string(got) != `[[3],[5]]` || srv.peak != 1 {
		t.Fatalf("settings %s, peak overlapping read-modify-writes %d", got, srv.peak)
	}
}

// The lock honors the caller's context while waiting.
func TestConfigureLockHonorsContext(t *testing.T) {
	c := New(&settingsServer{})
	unlock, err := c.configLock.Lock(context.Background(), fakeNB)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := c.Configure(ctx, fakeNB, ChatConfig{Goal: "default"}); err == nil {
		t.Fatal("Configure ran while another held the notebook's settings lock")
	}
}
