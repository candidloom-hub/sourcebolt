package nlm

import (
	"context"
	"encoding/json/jsontext"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// threadServer models the server side of chat threads: a stream sent with
// a null conversation id creates a new thread, and hPTbtc lists a notebook's
// threads newest first. Each stream holds until hold(nb) returns, so tests
// can force two asks to overlap.
type threadServer struct {
	mu       sync.Mutex
	n        int
	threads  map[string][]string // nb -> thread ids, newest first
	inflight map[string]int      // nb -> streams running now
	peak     map[string]int      // nb -> most streams running at once
	started  int                 // streams started so far, all notebooks
	hold     func(s *threadServer, nb string)
}

func newThreadServer(hold func(s *threadServer, nb string)) *threadServer {
	return &threadServer{threads: map[string][]string{}, inflight: map[string]int{}, peak: map[string]int{}, hold: hold}
}

func (s *threadServer) Name() string         { return "fake" }
func (s *threadServer) Supports(string) bool { return true }

func (s *threadServer) Do(ctx context.Context, c transport.Call) (transport.Result, error) {
	if c.ID != "hPTbtc" {
		return transport.Result{}, fmt.Errorf("unexpected call %s", c.ID)
	}
	nb := c.Params.([]any)[2].(string)
	s.mu.Lock()
	rows := make([]string, 0, len(s.threads[nb]))
	for _, id := range s.threads[nb] {
		rows = append(rows, `["`+id+`"]`)
	}
	s.mu.Unlock()
	return transport.Result{Payload: jsontext.Value("[[" + strings.Join(rows, ",") + "]]"), Transport: "fake"}, nil
}

func (s *threadServer) Stream(ctx context.Context, nb string, params any, onSnap func(wire.Snapshot)) (wire.Snapshot, error) {
	s.mu.Lock()
	if params.([]any)[4] == nil {
		s.n++
		s.threads[nb] = append([]string{fmt.Sprintf("t%d", s.n)}, s.threads[nb]...)
	}
	s.inflight[nb]++
	s.started++
	s.peak[nb] = max(s.peak[nb], s.inflight[nb])
	s.mu.Unlock()
	s.hold(s, nb)
	s.mu.Lock()
	s.inflight[nb]--
	s.mu.Unlock()
	return wire.Snapshot{Text: "answer"}, nil
}

// waitFor polls cond (under s.mu) until it holds or d passes.
func (s *threadServer) waitFor(d time.Duration, cond func() bool) bool {
	for end := time.Now().Add(d); time.Now().Before(end); time.Sleep(time.Millisecond) {
		s.mu.Lock()
		ok := cond()
		s.mu.Unlock()
		if ok {
			return true
		}
	}
	return false
}

// askBoth runs two asks at once and returns their answers.
func askBoth(t *testing.T, c *Client, nbs [2]string, o [2]AskOptions) [2]Answer {
	t.Helper()
	var out [2]Answer
	var errs [2]error
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Go(func() {
			out[i], errs[i] = c.Ask(context.Background(), nbs[i], "q", o[i], nil)
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// Two new-conversation asks on one notebook: without serialization both
// streams run, both threads exist, and both read back the newest one.
func TestAskNewConversationConcurrentSameNotebook(t *testing.T) {
	srv := newThreadServer(func(s *threadServer, nb string) {
		// Give a second stream on this notebook the chance to start.
		s.waitFor(150*time.Millisecond, func() bool { return s.inflight[nb] >= 2 })
	})
	o := AskOptions{SourceIDs: []string{"s1"}, NewConversation: true}
	got := askBoth(t, New(srv), [2]string{liveNB, liveNB}, [2]AskOptions{o, o})
	if got[0].ConversationID == "" || got[0].ConversationID == got[1].ConversationID {
		t.Fatalf("threads = %q, %q; each ask must get its own", got[0].ConversationID, got[1].ConversationID)
	}
	if srv.peak[liveNB] != 1 {
		t.Fatalf("%d new-conversation streams ran at once on one notebook", srv.peak[liveNB])
	}
}

// New-conversation asks on different notebooks, and asks with an explicit
// conversation id, must still run at the same time: each stream waits until
// the other one has started.
func TestAskConcurrencyStaysWhereSafe(t *testing.T) {
	const otherNB = "00000000-0000-4000-8000-0000000000ff"
	for name, tc := range map[string]struct {
		nbs [2]string
		o   [2]AskOptions
	}{
		"new conversation, two notebooks": {
			[2]string{liveNB, otherNB},
			[2]AskOptions{{SourceIDs: []string{"s1"}, NewConversation: true}, {SourceIDs: []string{"s1"}, NewConversation: true}},
		},
		"explicit conversation, one notebook": {
			[2]string{liveNB, liveNB},
			[2]AskOptions{{SourceIDs: []string{"s1"}, ConversationID: "c1"}, {SourceIDs: []string{"s1"}, ConversationID: "c2"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			overlapped := 0
			srv := newThreadServer(func(s *threadServer, nb string) {
				if s.waitFor(2*time.Second, func() bool { return s.started >= 2 }) {
					mu.Lock()
					overlapped++
					mu.Unlock()
				}
			})
			askBoth(t, New(srv), tc.nbs, tc.o)
			if overlapped != 2 {
				t.Fatalf("only %d of 2 streams saw the other running; asks were serialized", overlapped)
			}
		})
	}
}

// Two default-thread asks on a notebook with no thread yet: the second must
// join the thread the first started, not start another one.
func TestAskDefaultThreadConcurrentFirstAsk(t *testing.T) {
	srv := newThreadServer(func(s *threadServer, nb string) {
		s.waitFor(150*time.Millisecond, func() bool { return s.inflight[nb] >= 2 })
	})
	o := AskOptions{SourceIDs: []string{"s1"}}
	got := askBoth(t, New(srv), [2]string{liveNB, liveNB}, [2]AskOptions{o, o})
	if srv.n != 1 || got[0].ConversationID != "t1" || got[1].ConversationID != "t1" {
		t.Fatalf("threads started = %d, answers in %q and %q", srv.n, got[0].ConversationID, got[1].ConversationID)
	}
}

func TestKeyedLockCancelAndCleanup(t *testing.T) {
	var k keyedLock
	unlock, err := k.Lock(context.Background(), "a")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := k.Lock(ctx, "a"); err != context.DeadlineExceeded {
		t.Fatalf("waiting on a held key: err = %v", err)
	}
	other, err := k.Lock(context.Background(), "b") // other keys are free
	if err != nil {
		t.Fatal(err)
	}
	other()
	unlock()
	if len(k.m) != 0 {
		t.Fatalf("entries left after unlock: %d", len(k.m))
	}
}
