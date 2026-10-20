package browser

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

// PlainOptions configures a browser started as a normal process for
// sign-in: the way a person would start it, plus a debugging port.
type PlainOptions struct {
	ExecPath   string // resolved browser executable (see FindBrowser)
	ProfileDir string // dedicated user-data-dir, required: <config>/browser/<profile>
	Port       int    // remote debugging port on 127.0.0.1
	StartURL   string // page to open
}

// PlainArgs is the browser command line: only the debugging port, the
// dedicated profile and the start page. No chromedp defaults, no
// --enable-automation, no headless. Google refuses sign-in ("this browser
// may not be secure") in browsers that look automated.
func PlainArgs(o PlainOptions) []string {
	return []string{
		"--remote-debugging-port=" + strconv.Itoa(o.Port),
		"--user-data-dir=" + o.ProfileDir,
		o.StartURL,
	}
}

// FreePort returns a TCP port on 127.0.0.1 that was free a moment ago.
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// Plain is a browser process started by StartPlain.
type Plain struct {
	cmd  *exec.Cmd
	port int
	done chan struct{}
}

// StartPlain starts the browser as a normal process. It does not wait for
// the debugging port; callers poll DevToolsURL.
func StartPlain(o PlainOptions) (*Plain, error) {
	if o.ProfileDir == "" {
		return nil, errors.New("browser: PlainOptions.ProfileDir is required (never the default profile)")
	}
	if err := os.MkdirAll(o.ProfileDir, 0o700); err != nil {
		return nil, err
	}
	cmd := exec.Command(o.ExecPath, PlainArgs(o)...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("browser: start %s: %w", o.ExecPath, err)
	}
	p := &Plain{cmd: cmd, port: o.Port, done: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		close(p.done)
	}()
	return p, nil
}

// DevToolsURL is the address AttachCDP reads cookies from.
func (p *Plain) DevToolsURL() string { return "http://127.0.0.1:" + strconv.Itoa(p.port) }

// Done is closed when the browser process has exited (for example, the user
// closed the window).
func (p *Plain) Done() <-chan struct{} { return p.done }

// Close asks the browser to quit over the debugging port, so it writes its
// cookies to the profile, and kills it if it has not exited when ctx ends.
func (p *Plain) Close(ctx context.Context) error {
	select {
	case <-p.done:
		return nil
	default:
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	_ = closeCDP(cctx, p.DevToolsURL())
	cancel()
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
	}
	if err := p.cmd.Process.Kill(); err != nil {
		return err
	}
	<-p.done
	return nil
}

// closeCDP sends Browser.close to the browser at cdpURL.
func closeCDP(ctx context.Context, cdpURL string) error {
	actx, cancelAlloc := remote.NewAllocator(ctx, cdpURL)
	defer cancelAlloc()
	bctx, cancelCtx := chromedp.NewContext(actx)
	defer cancelCtx()
	_, err := chromedp.CallBrowser(bctx, cdpbrowser.Close, cdp.Empty{})
	return err
}
