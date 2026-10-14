package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newClock() *clock { return &clock{t: time.Unix(1_700_000_000, 0)} }

func TestSingleflight(t *testing.T) {
	c := New(newClock().now)
	var loads atomic.Int32
	release := make(chan struct{})
	load := func(context.Context) (any, error) {
		loads.Add(1)
		<-release
		return "v", nil
	}
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if v, err := c.Get(context.Background(), "p/notebooks", time.Minute, load); err != nil || v != "v" {
				t.Errorf("got %v %v", v, err)
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if n := loads.Load(); n != 1 {
		t.Fatalf("loads = %d, want 1", n)
	}
}

func TestTTL(t *testing.T) {
	clk := newClock()
	c := New(clk.now)
	n := 0
	load := func(context.Context) (any, error) { n++; return n, nil }
	c.Get(context.Background(), "k", 30*time.Second, load)
	clk.add(10 * time.Second)
	if v, _ := c.Get(context.Background(), "k", 30*time.Second, load); v != 1 {
		t.Fatalf("fresh read = %v", v)
	}
	clk.add(30 * time.Second)
	if v, _ := c.Get(context.Background(), "k", 30*time.Second, load); v != 2 {
		t.Fatalf("expired read = %v, want reload", v)
	}
}

func TestStaleWhileRevalidate(t *testing.T) {
	clk := newClock()
	c := New(clk.now)
	var loads atomic.Int32
	done := make(chan struct{}, 4)
	load := func(context.Context) (any, error) {
		n := loads.Add(1)
		if n > 1 {
			defer func() { done <- struct{}{} }()
		}
		return n, nil
	}
	c.Get(context.Background(), "k", 30*time.Second, load)
	clk.add(20 * time.Second) // past ttl/2, before ttl
	if v, _ := c.Get(context.Background(), "k", 30*time.Second, load); v != int32(1) {
		t.Fatalf("stale read must return old value at once, got %v", v)
	}
	c.Get(context.Background(), "k", 30*time.Second, load)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("background refresh never ran")
	}
	time.Sleep(20 * time.Millisecond)
	if n := loads.Load(); n != 2 {
		t.Fatalf("loads = %d, want exactly one background refresh", n)
	}
	if v, _ := c.Get(context.Background(), "k", 30*time.Second, load); v != int32(2) {
		t.Fatalf("after refresh = %v", v)
	}
}

func TestInvalidatePrefix(t *testing.T) {
	c := New(newClock().now)
	n := 0
	load := func(context.Context) (any, error) { n++; return n, nil }
	c.Get(context.Background(), "p/notebook/a", time.Minute, load)
	c.Get(context.Background(), "p/notebooks", time.Minute, load)
	c.Get(context.Background(), "q/notebook/a", time.Minute, load)
	c.Invalidate("p/notebook")
	if v, _ := c.Get(context.Background(), "p/notebook/a", time.Minute, load); v != 4 {
		t.Fatalf("p/notebook/a = %v, want reload", v)
	}
	if v, _ := c.Get(context.Background(), "q/notebook/a", time.Minute, load); v != 3 {
		t.Fatalf("other profile must stay cached, got %v", v)
	}
}

func TestLoaderErrorNotCached(t *testing.T) {
	c := New(newClock().now)
	calls := 0
	load := func(context.Context) (any, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("boom")
		}
		return "ok", nil
	}
	if _, err := c.Get(context.Background(), "k", time.Minute, load); err == nil {
		t.Fatal("want error")
	}
	if v, err := c.Get(context.Background(), "k", time.Minute, load); err != nil || v != "ok" {
		t.Fatalf("got %v %v", v, err)
	}
}

func BenchmarkHit(b *testing.B) {
	c := New(time.Now)
	load := func(context.Context) (any, error) { return 1, nil }
	ctx := context.Background()
	c.Get(ctx, "k", time.Hour, load)
	for b.Loop() {
		c.Get(ctx, "k", time.Hour, load)
	}
}

func TestCancelOneWaiterOthersSucceed(t *testing.T) {
	c := New(newClock().now)
	release := make(chan struct{})
	load := func(ctx context.Context) (any, error) {
		select {
		case <-release:
			return "v", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	ctxA, cancelA := context.WithCancel(context.Background())
	errA := make(chan error, 1)
	go func() { _, err := c.Get(ctxA, "k", time.Minute, load); errA <- err }()
	time.Sleep(20 * time.Millisecond)
	resB := make(chan any, 1)
	go func() {
		v, err := c.Get(context.Background(), "k", time.Minute, load)
		if err != nil {
			resB <- err
		} else {
			resB <- v
		}
	}()
	time.Sleep(20 * time.Millisecond)
	cancelA()
	if err := <-errA; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled caller: %v", err)
	}
	close(release)
	if v := <-resB; v != "v" {
		t.Fatalf("other waiter must still get the value, got %v", v)
	}
}

func TestLastWaiterCancelStopsLoad(t *testing.T) {
	c := New(newClock().now)
	stopped := make(chan struct{})
	load := func(ctx context.Context) (any, error) { <-ctx.Done(); close(stopped); return nil, ctx.Err() }
	ctx, cancel := context.WithCancel(context.Background())
	go c.Get(ctx, "k", time.Minute, load)
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("load must stop when its only caller cancels")
	}
}
