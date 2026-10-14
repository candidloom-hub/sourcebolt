package browser

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// ErrClosed means the browser process is gone (closed by the user, reaped
// when idle, or Close was called).
var ErrClosed = errors.New("browser: closed")

// Browser is one running browser process with one warm tab.
type Browser struct {
	tab         context.Context // chromedp context of the first tab
	cancelTab   context.CancelFunc
	cancelAlloc context.CancelFunc

	lost <-chan struct{} // closed when the connection to the browser drops

	lock chan struct{} // FIFO: blocked channel senders are served in order
	idle time.Duration

	mu    sync.Mutex
	timer *time.Timer
	once  sync.Once
}

// Launch starts the browser. ctx bounds the whole browser lifetime: when it
// ends, the process is stopped.
func Launch(ctx context.Context, o Options) (*Browser, error) {
	if o.ProfileDir == "" {
		return nil, errors.New("browser: Options.ProfileDir is required (never the default profile)")
	}
	exe, err := FindBrowser(o.ExecPath)
	if err != nil {
		return nil, err
	}
	o.ExecPath = exe
	if err := os.MkdirAll(o.ProfileDir, 0o700); err != nil {
		return nil, err
	}
	actx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocatorOptions(o)...)
	tab, cancelTab := chromedp.NewContext(actx)
	if err := chromedp.Do(tab); err != nil {
		cancelTab()
		cancelAlloc()
		return nil, fmt.Errorf("browser: start %s: %w", exe, err)
	}
	b := &Browser{tab: tab, cancelTab: cancelTab, cancelAlloc: cancelAlloc, lock: make(chan struct{}, 1), idle: o.Idle}
	if c := chromedp.FromContext(tab); c != nil && c.Browser != nil {
		b.lost = c.Browser.LostConnection
	}
	b.armIdle()
	return b, nil
}

// Done is closed when the browser is gone.
func (b *Browser) Done() <-chan struct{} { return b.tab.Done() }

// gone reports whether the browser closed or its connection dropped (the
// user closed the last window).
func (b *Browser) gone() bool {
	if b.tab.Err() != nil {
		return true
	}
	select {
	case <-b.lost:
		return true
	default:
		return false
	}
}

// Tab locks the warm tab for one caller and returns a chromedp context for
// it. The context also ends with ctx. Callers must call release.
func (b *Browser) Tab(ctx context.Context) (context.Context, func(), error) {
	select {
	case b.lock <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-b.Done():
		return nil, nil, ErrClosed
	}
	if b.gone() {
		<-b.lock
		return nil, nil, ErrClosed
	}
	b.stopIdle()
	tctx, cancel := context.WithCancel(b.tab)
	stop := context.AfterFunc(ctx, cancel)
	var once sync.Once
	release := func() {
		once.Do(func() {
			stop()
			cancel()
			<-b.lock
			b.armIdle()
		})
	}
	return tctx, release, nil
}

// Close stops the browser gracefully and waits for the process to exit.
func (b *Browser) Close() error {
	var err error
	b.once.Do(func() {
		b.stopIdle()
		if b.tab.Err() == nil {
			err = chromedp.Cancel(b.tab)
		}
		b.cancelTab()
		b.cancelAlloc()
		if errors.Is(err, context.Canceled) {
			err = nil
		}
	})
	return err
}

func (b *Browser) armIdle() {
	if b.idle <= 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.timer != nil {
		b.timer.Stop()
	}
	b.timer = time.AfterFunc(b.idle, func() { b.Close() })
}

func (b *Browser) stopIdle() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
}
