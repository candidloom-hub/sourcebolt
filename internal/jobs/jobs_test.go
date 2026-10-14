package jobs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newTable() (*Table, *fakeClock) {
	c := &fakeClock{t: time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)}
	return New(WithClock(c.Now)), c
}

func waitStatus(t *testing.T, tb *Table, id, want string) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		j, ok := tb.Get(id)
		if ok && j.Status == want {
			return j
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s: status %q (ok=%v), want %q", id, j.Status, ok, want)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestStartDone(t *testing.T) {
	tb, clk := newTable()
	release := make(chan struct{})
	id := tb.Start("research", func(ctx context.Context) (any, error) {
		<-release
		return "result", nil
	})
	j, ok := tb.Get(id)
	if !ok || j.Status != StatusRunning || j.Kind != "research" || j.ID != id || !j.Started.Equal(clk.Now()) {
		t.Fatalf("running job = %+v ok=%v", j, ok)
	}
	close(release)
	j = waitStatus(t, tb, id, StatusDone)
	if j.Result != "result" || j.Err != nil {
		t.Errorf("done job = %+v", j)
	}
}

func TestStartFailed(t *testing.T) {
	tb, _ := newTable()
	boom := errors.New("boom")
	id := tb.Start("chat", func(ctx context.Context) (any, error) { return nil, boom })
	j := waitStatus(t, tb, id, StatusFailed)
	if !errors.Is(j.Err, boom) {
		t.Errorf("Err = %v", j.Err)
	}
}

func TestPanicBecomesFailed(t *testing.T) {
	tb, _ := newTable()
	id := tb.Start("x", func(ctx context.Context) (any, error) { panic("kaboom") })
	j := waitStatus(t, tb, id, StatusFailed)
	if j.Err == nil {
		t.Error("want error from panic")
	}
}

func TestCancel(t *testing.T) {
	tb, _ := newTable()
	started := make(chan struct{})
	ctxDone := make(chan struct{})
	id := tb.Start("research", func(ctx context.Context) (any, error) {
		close(started)
		<-ctx.Done()
		close(ctxDone)
		return "late", ctx.Err()
	})
	<-started
	if !tb.Cancel(id) {
		t.Fatal("Cancel on running job = false")
	}
	select {
	case <-ctxDone:
	case <-time.After(5 * time.Second):
		t.Fatal("job ctx not cancelled")
	}
	j, _ := tb.Get(id)
	if j.Status != StatusCancelled {
		t.Errorf("status = %q", j.Status)
	}
	// The function returning afterwards must not overwrite the status.
	time.Sleep(20 * time.Millisecond)
	j, _ = tb.Get(id)
	if j.Status != StatusCancelled || j.Result != nil {
		t.Errorf("after return: %+v", j)
	}
	if tb.Cancel(id) {
		t.Error("second Cancel = true")
	}
}

func TestCancelFinishedAndUnknown(t *testing.T) {
	tb, _ := newTable()
	id := tb.Start("x", func(ctx context.Context) (any, error) { return 1, nil })
	waitStatus(t, tb, id, StatusDone)
	if tb.Cancel(id) {
		t.Error("Cancel on done job = true")
	}
	if j, _ := tb.Get(id); j.Status != StatusDone {
		t.Errorf("status changed to %q", j.Status)
	}
	if tb.Cancel("nope") {
		t.Error("Cancel on unknown id = true")
	}
	if _, ok := tb.Get("nope"); ok {
		t.Error("Get on unknown id ok")
	}
}

func TestEviction(t *testing.T) {
	tb, clk := newTable()
	id := tb.Start("x", func(ctx context.Context) (any, error) { return 1, nil })
	waitStatus(t, tb, id, StatusDone)

	clk.Advance(59 * time.Minute)
	if _, ok := tb.Get(id); !ok {
		t.Fatal("evicted before 1h")
	}
	clk.Advance(2 * time.Minute)
	if _, ok := tb.Get(id); ok {
		t.Fatal("not evicted after 1h")
	}
}

func TestEvictionCountsFromFinishNotStart(t *testing.T) {
	tb, clk := newTable()
	release := make(chan struct{})
	id := tb.Start("x", func(ctx context.Context) (any, error) { <-release; return 1, nil })
	clk.Advance(3 * time.Hour) // running jobs are never evicted
	if _, ok := tb.Get(id); !ok {
		t.Fatal("running job evicted")
	}
	close(release)
	waitStatus(t, tb, id, StatusDone)
	clk.Advance(30 * time.Minute)
	if _, ok := tb.Get(id); !ok {
		t.Fatal("evicted 30m after finish")
	}
}

func TestEvictedCancelledJob(t *testing.T) {
	tb, clk := newTable()
	id := tb.Start("x", func(ctx context.Context) (any, error) { <-ctx.Done(); return nil, ctx.Err() })
	tb.Cancel(id)
	clk.Advance(61 * time.Minute)
	if _, ok := tb.Get(id); ok {
		t.Fatal("cancelled job not evicted")
	}
}

func TestIDsUnguessableAndUnique(t *testing.T) {
	tb, _ := newTable()
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		id := tb.Start("x", func(ctx context.Context) (any, error) { return nil, nil })
		if len(id) < 22 {
			t.Fatalf("id %q too short (< 128 bits of base64/hex)", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestConcurrentUse(t *testing.T) {
	tb, _ := newTable()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := tb.Start("x", func(ctx context.Context) (any, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(time.Millisecond):
					return 1, nil
				}
			})
			tb.Get(id)
			tb.Cancel(id)
			tb.Get(id)
		}()
	}
	wg.Wait()
}

func TestGetReturnsCopy(t *testing.T) {
	tb, _ := newTable()
	id := tb.Start("x", func(ctx context.Context) (any, error) { return 1, nil })
	j, _ := tb.Get(id)
	j.Status = "tampered"
	waitStatus(t, tb, id, StatusDone)
}
