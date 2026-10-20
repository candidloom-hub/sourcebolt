package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport/browser"
	"github.com/zalando/go-keyring"
)

// fakePlain stands in for the browser process; nothing starts.
type fakePlain struct {
	mu     sync.Mutex
	done   chan struct{}
	closed int
}

func (f *fakePlain) DevToolsURL() string   { return "http://127.0.0.1:4242" }
func (f *fakePlain) Done() <-chan struct{} { return f.done }
func (f *fakePlain) Close(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed++
	return nil
}

type autoStub struct {
	plain      *fakePlain
	started    *browser.PlainOptions
	findArg    string
	attachURLs []string
	deadline   time.Time
	checks     int
}

// stubAuto replaces every outside effect of the no-flag login: browser
// discovery, the port, the process, the CDP reads and the validating call.
// jars are returned one per poll (the last one repeats).
func stubAuto(t *testing.T, findErr error, jars []*session.Jar, checkErr error) *autoStub {
	t.Helper()
	keyring.MockInit()
	s := &autoStub{plain: &fakePlain{done: make(chan struct{})}}
	oldFind, oldPort, oldStart, oldAttach, oldCheck, oldEvery := findBrowser, freePort, startPlain, attachBrowser, checkSession, pollEvery
	findBrowser = func(explicit string) (string, error) {
		s.findArg = explicit
		if findErr != nil {
			return "", findErr
		}
		return "fake-browser.exe", nil
	}
	freePort = func() (int, error) { return 4242, nil }
	startPlain = func(o browser.PlainOptions) (plainBrowser, error) {
		s.started = &o
		return s.plain, nil
	}
	var mu sync.Mutex
	attachBrowser = func(ctx context.Context, url string, h session.Host) (*session.Jar, error) {
		mu.Lock()
		defer mu.Unlock()
		s.attachURLs = append(s.attachURLs, url)
		if dl, ok := ctx.Deadline(); ok {
			s.deadline = dl
		}
		if len(jars) == 0 {
			return nil, errors.New("connection refused")
		}
		j := jars[0]
		if len(jars) > 1 {
			jars = jars[1:]
		}
		return j, nil
	}
	checkSession = func(ctx context.Context, p *session.Profile) (int, error) {
		s.checks++
		return 7, checkErr
	}
	pollEvery = time.Millisecond
	t.Cleanup(func() {
		findBrowser, freePort, startPlain, attachBrowser, checkSession, pollEvery = oldFind, oldPort, oldStart, oldAttach, oldCheck, oldEvery
	})
	return s
}

func TestLoginNoFlagsSignsInWithPlainBrowser(t *testing.T) {
	s := stubAuto(t, nil, []*session.Jar{session.NewJar(), fakeJar("SID"), fakeJar(validNames...)}, nil)
	code, dir, out, errOut := runLogin(t, "--browser-path", `D:\b\brave.exe`)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if s.findArg != `D:\b\brave.exe` {
		t.Errorf("browser discovery got %q", s.findArg)
	}
	o := s.started
	if o == nil {
		t.Fatal("no browser started")
	}
	if want := filepath.Join(dir, "browser", "default"); o.ProfileDir != want {
		t.Errorf("ProfileDir = %q, want %q (the fallback's profile)", o.ProfileDir, want)
	}
	if o.ExecPath != "fake-browser.exe" || o.Port != 4242 || o.StartURL != "https://notebook.google.com/" {
		t.Errorf("start options = %+v", *o)
	}
	for _, u := range s.attachURLs {
		if u != "http://127.0.0.1:4242" {
			t.Fatalf("attached to %q", u)
		}
	}
	if len(s.attachURLs) < 3 {
		t.Errorf("polled %d times, want until the cookies were complete", len(s.attachURLs))
	}
	if s.checks != 1 {
		t.Errorf("validated %d times, want once (only a complete jar)", s.checks)
	}
	if s.plain.closed == 0 {
		t.Error("the browser must be closed after sign-in")
	}
	if !strings.Contains(errOut, "A browser window opened") || !strings.Contains(errOut, "finish by itself") {
		t.Errorf("instructions missing: %q", errOut)
	}
	if !strings.Contains(out, "7 notebooks") {
		t.Errorf("stdout = %q", out)
	}
	assertNoCookieValues(t, out+errOut)
	if files, _ := filepath.Glob(filepath.Join(dir, "profiles", "default*")); len(files) == 0 {
		t.Fatal("profile was not saved")
	}
}

func TestLoginNoFlagsWindowClosedSavesNothing(t *testing.T) {
	s := stubAuto(t, nil, nil, nil)
	close(s.plain.done)
	code, dir, _, errOut := runLogin(t)
	if code != 1 || !strings.Contains(errOut, "cancelled") || !strings.Contains(errOut, "nothing saved") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	assertNothingSaved(t, dir)
}

func TestLoginNoFlagsTimeout(t *testing.T) {
	s := stubAuto(t, nil, []*session.Jar{fakeJar("SID")}, nil)
	code, dir, _, errOut := runLogin(t, "--timeout", "30ms")
	if code != 1 || !strings.Contains(errOut, "timed out") || !strings.Contains(errOut, "nothing saved") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if !strings.Contains(errOut, "login --attach") || !strings.Contains(errOut, "login --import") {
		t.Errorf("timeout must name the fallbacks: %q", errOut)
	}
	if s.plain.closed == 0 {
		t.Error("the browser must be closed on timeout")
	}
	assertNothingSaved(t, dir)
}

func TestLoginNoFlagsDefaultTimeoutTenMinutes(t *testing.T) {
	s := stubAuto(t, nil, []*session.Jar{fakeJar(validNames...)}, nil)
	start := time.Now()
	if code, _, _, errOut := runLogin(t); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if d := s.deadline.Sub(start); d <= 9*time.Minute || d > 10*time.Minute+time.Second {
		t.Errorf("deadline in %v, want about 10m", d)
	}
}

func TestLoginNoFlagsValidationFailureKeepsWaiting(t *testing.T) {
	s := stubAuto(t, nil, []*session.Jar{fakeJar(validNames...)}, errors.New("HTTP 401 from notebook host"))
	code, dir, _, errOut := runLogin(t, "--timeout", "30ms")
	if code != 1 || !strings.Contains(errOut, "HTTP 401") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if s.checks < 2 {
		t.Errorf("checked %d times; a failed check must keep waiting", s.checks)
	}
	assertNothingSaved(t, dir)
}

func TestLoginNoFlagsBrowserNotFound(t *testing.T) {
	s := stubAuto(t, browser.ErrNoBrowser, nil, nil)
	code, dir, _, errOut := runLogin(t)
	if code != 1 || !strings.Contains(errOut, "login --import") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if s.started != nil {
		t.Error("nothing may start without a browser")
	}
	assertNothingSaved(t, dir)
}
