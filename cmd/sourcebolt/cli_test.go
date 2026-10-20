package main

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/transport/browser"
)

func runCLI(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", t.TempDir())
	t.Setenv("NOTEBOOKLM_BROWSER_FALLBACK", "0") // tests must never launch a real browser
	var out, errb bytes.Buffer
	code := run(context.Background(), args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func TestVersion(t *testing.T) {
	code, out, _ := runCLI(t, "", "--version")
	if code != 0 || strings.TrimSpace(out) != "sourcebolt dev" {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestUnknownCommand(t *testing.T) {
	if code, _, _ := runCLI(t, "", "frobnicate"); code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
}

func TestCallUnknownTool(t *testing.T) {
	code, _, errOut := runCLI(t, "", "call", "no_such_tool", "{}")
	if code != 2 || !strings.Contains(errOut, "no_such_tool") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
}

func TestCallServerInfo(t *testing.T) {
	code, out, errOut := runCLI(t, "", "call", "server_info", "{}")
	if code != 0 || !strings.Contains(out, `"name":"sourcebolt"`) {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
}

func TestCallToolErrorExitsOne(t *testing.T) {
	// No profile exists, so the first Google call must fail with an auth envelope.
	t.Setenv("NOTEBOOKLM_HOST", "127.0.0.1:1")
	code, out, _ := runCLI(t, "", "call", "notebook_list", "{}")
	if code != 1 || !strings.Contains(out, `"status":"error"`) {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestLoginImportRejectsInvalidJar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", dir)
	var out, errb bytes.Buffer
	code := run(context.Background(), []string{"login", "--import", "-", "--format", "header"}, strings.NewReader("Cookie: SID=only"), &out, &errb)
	if code != 1 || !strings.Contains(errb.String(), "missing required cookies") {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
	if files, _ := filepath.Glob(filepath.Join(dir, "profiles", "*")); len(files) != 0 {
		t.Fatalf("nothing may be saved, found %v", files)
	}
}

func TestServeInitializeFast(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := filepath.Join(t.TempDir(), "sourcebolt")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, "serve")
	cmd.Env = append(os.Environ(), "NOTEBOOKLM_CONFIG_DIR="+t.TempDir(), "NOTEBOOKLM_HOST=127.0.0.1:1")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stdin.Close(); cmd.Process.Kill(); cmd.Wait() })

	start := time.Now()
	stdin.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}` + "\n"))
	line := make(chan string, 1)
	go func() {
		s := bufio.NewScanner(stdout)
		if s.Scan() {
			line <- s.Text()
		}
	}()
	select {
	case l := <-line:
		if !strings.Contains(l, `"serverInfo"`) || !strings.Contains(l, `"sourcebolt"`) {
			t.Fatalf("unexpected response %q", l)
		}
		t.Logf("initialize answered in %v", time.Since(start))
		if d := time.Since(start); d > 500*time.Millisecond {
			t.Fatalf("initialize took %v", d)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no initialize response")
	}
}

// fakeJar has the cookie names a valid session needs, with made-up values.
func fakeJar(names ...string) *session.Jar {
	j := session.NewJar()
	for _, n := range names {
		j.Add(session.Cookie{Name: n, Value: "fake-value-" + n, Domain: ".google.com", Path: "/", Expires: time.Now().Add(time.Hour)})
	}
	return j
}

var validNames = []string{"SID", "__Secure-1PSIDTS", "APISID", "SAPISID"}

type loginCall struct {
	o       browser.Options
	h       session.Host
	timeout time.Duration
	url     string
}

// stubBrowser replaces the browser functions for one test.
func stubBrowser(t *testing.T, jar *session.Jar, landed session.Host, err error) *loginCall {
	t.Helper()
	got := &loginCall{}
	oldLogin, oldAttach := loginBrowser, attachBrowser
	loginBrowser = func(ctx context.Context, o browser.Options, h session.Host) (*session.Jar, session.Host, error) {
		got.o, got.h = o, h
		if dl, ok := ctx.Deadline(); ok {
			got.timeout = time.Until(dl)
		}
		return jar, landed, err
	}
	attachBrowser = func(ctx context.Context, url string, h session.Host) (*session.Jar, error) {
		got.url, got.h = url, h
		return jar, err
	}
	t.Cleanup(func() { loginBrowser, attachBrowser = oldLogin, oldAttach })
	return got
}

func runLogin(t *testing.T, args ...string) (code int, dir, stdout, stderr string) {
	t.Helper()
	dir = t.TempDir()
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", dir)
	t.Setenv("NOTEBOOKLM_HOST", "")
	var out, errb bytes.Buffer
	code = run(t.Context(), append([]string{"login"}, args...), strings.NewReader(""), &out, &errb)
	return code, dir, out.String(), errb.String()
}

func assertNothingSaved(t *testing.T, dir string) {
	t.Helper()
	if files, _ := filepath.Glob(filepath.Join(dir, "profiles", "*")); len(files) != 0 {
		t.Fatalf("nothing may be saved, found %v", files)
	}
}

func assertNoCookieValues(t *testing.T, s string) {
	t.Helper()
	if strings.Contains(s, "fake-value-") {
		t.Fatalf("output leaks a cookie value: %q", s)
	}
}

func TestLoginAtMostOneMethod(t *testing.T) {
	stubBrowser(t, nil, "", nil)
	if code, _, _, errOut := runLogin(t, "--attach", "x", "--import", "-"); code != 2 || !strings.Contains(errOut, "--browser") {
		t.Fatalf("two methods: code=%d stderr=%q", code, errOut)
	}
	if code, _, _, _ := runLogin(t, "--browser", "--attach", "http://127.0.0.1:9222"); code != 2 {
		t.Fatalf("two methods: code=%d, want 2", code)
	}
	if code, _, _, _ := runLogin(t, "--browser", "--import", "-"); code != 2 {
		t.Fatalf("two methods: code=%d, want 2", code)
	}
}

func TestLoginBrowserPassesOptions(t *testing.T) {
	got := stubBrowser(t, nil, "", browser.ErrCancelled)
	code, dir, _, _ := runLogin(t, "--browser", "--profile", "work", "--host", "notebooklm.google.com",
		"--browser-path", `D:\b\brave.exe`, "--timeout", "90s")
	if code != 1 {
		t.Fatalf("code = %d", code)
	}
	if want := filepath.Join(dir, "browser", "work"); got.o.ProfileDir != want {
		t.Errorf("ProfileDir = %q, want %q", got.o.ProfileDir, want)
	}
	if got.o.Headless {
		t.Error("login must open a visible window")
	}
	if got.o.ExecPath != `D:\b\brave.exe` {
		t.Errorf("ExecPath = %q", got.o.ExecPath)
	}
	if got.h != "notebooklm.google.com" {
		t.Errorf("host = %q", got.h)
	}
	if got.timeout <= 80*time.Second || got.timeout > 90*time.Second {
		t.Errorf("timeout = %v, want about 90s", got.timeout)
	}
}

func TestLoginBrowserDefaultTimeoutAndHost(t *testing.T) {
	got := stubBrowser(t, nil, "", browser.ErrCancelled)
	runLogin(t, "--browser")
	if got.h != session.HostDefault {
		t.Errorf("host = %q, want default", got.h)
	}
	if got.timeout <= 290*time.Second || got.timeout > 300*time.Second {
		t.Errorf("timeout = %v, want about 300s", got.timeout)
	}
	if !strings.HasSuffix(got.o.ProfileDir, filepath.Join("browser", "default")) {
		t.Errorf("ProfileDir = %q", got.o.ProfileDir)
	}
}

func TestLoginBrowserCancelledSavesNothing(t *testing.T) {
	stubBrowser(t, nil, "", browser.ErrCancelled)
	code, dir, _, errOut := runLogin(t, "--browser")
	if code != 1 || !strings.Contains(errOut, "CANCELLED") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	assertNothingSaved(t, dir)
}

func TestLoginBrowserMissingCookiesSavesNothing(t *testing.T) {
	stubBrowser(t, fakeJar("SID"), session.HostDefault, nil)
	code, dir, _, errOut := runLogin(t, "--browser")
	if code != 1 || !strings.Contains(errOut, "missing required cookies") || !strings.Contains(errOut, "__Secure-1PSIDTS") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	assertNoCookieValues(t, errOut)
	assertNothingSaved(t, dir)
}

func TestLoginBrowserValidationFailureSavesNothing(t *testing.T) {
	// The account "landed" on an unreachable host, so the validating
	// ListNotebooks call fails. It must target the landed host.
	stubBrowser(t, fakeJar(validNames...), "127.0.0.1:1", nil)
	code, dir, out, errOut := runLogin(t, "--browser")
	if code != 1 || !strings.Contains(errOut, "127.0.0.1:1") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	assertNoCookieValues(t, out+errOut)
	assertNothingSaved(t, dir)
}

func TestLoginAttachValidationFailureSavesNothing(t *testing.T) {
	got := stubBrowser(t, fakeJar(validNames...), "", nil)
	code, dir, out, errOut := runLogin(t, "--attach", "http://127.0.0.1:9222", "--host", "127.0.0.1:1")
	if code != 1 || !strings.Contains(errOut, "127.0.0.1:1") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if got.url != "http://127.0.0.1:9222" || got.h != "127.0.0.1:1" {
		t.Errorf("attach got url=%q host=%q", got.url, got.h)
	}
	assertNoCookieValues(t, out+errOut)
	assertNothingSaved(t, dir)
}

func TestLoginBrowserRejectsUnsafeProfileName(t *testing.T) {
	got := stubBrowser(t, nil, "", browser.ErrCancelled)
	code, _, _, _ := runLogin(t, "--browser", "--profile", `..\x`)
	if code == 0 || got.o.ProfileDir != "" {
		t.Fatalf("code=%d, browser launched with %q", code, got.o.ProfileDir)
	}
}

// The advice must name exactly the cookies Jar.Valid checks: not HSID
// (never checked), and the SAPISID/OSID alternatives that are.
func TestLoginImportHintMatchesCheck(t *testing.T) {
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", t.TempDir())
	var out, errb bytes.Buffer
	run(context.Background(), []string{"login", "--import", "-", "--format", "header"}, strings.NewReader("Cookie: SID=only"), &out, &errb)
	msg := errb.String()
	for _, want := range []string{"__Secure-1PSIDTS", "OSID", "APISID", "SAPISID"} {
		if !strings.Contains(msg, want) {
			t.Errorf("hint lacks %s: %q", want, msg)
		}
	}
	if strings.Contains(msg, "HSID") {
		t.Errorf("hint names HSID, which is not required: %q", msg)
	}
}
