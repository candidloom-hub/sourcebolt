package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/zalando/go-keyring"
)

// syncBuffer is a goroutine-safe stderr.
type syncBuffer struct {
	ch chan struct{}
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.ch <- struct{}{}
	defer func() { <-s.ch }()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.ch <- struct{}{}
	defer func() { <-s.ch }()
	return s.b.String()
}

// `serve` with a saved profile it cannot read must still complete the MCP
// handshake (clients otherwise only show "connection closed") and answer
// tool calls with AUTH_REQUIRED and the login hint.
func TestServeStartsWithUnreadableProfile(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", dir)
	t.Setenv("NOTEBOOKLM_BROWSER_FALLBACK", "0")
	t.Setenv("NOTEBOOKLM_PROFILE", "")
	t.Setenv("NOTEBOOKLM_COOKIES", "")
	t.Setenv("NOTEBOOKLM_COOKIES_FILE", "")
	t.Setenv("NOTEBOOKLM_HOST", "127.0.0.1:1") // nothing may be sent anyway
	pdir := filepath.Join(dir, "profiles")
	if err := session.NewFileStore(pdir, session.OSKeyring{}).Save(&session.Profile{Name: "default", Jar: session.NewJar()}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "default.profile"), []byte("damaged"), 0o600); err != nil {
		t.Fatal(err)
	}

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	stderr := &syncBuffer{ch: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() {
		code := cmdServe(ctx, []string{"--tools", "minimal"}, inR, outW, stderr)
		outW.Close()
		done <- code
	}()

	cctx, ccancel := context.WithTimeout(ctx, 20*time.Second)
	defer ccancel()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil).
		Connect(cctx, &mcp.IOTransport{Reader: outR, Writer: inW}, nil)
	if err != nil {
		select {
		case code := <-done:
			t.Fatalf("serve exited %d before the handshake: %s", code, stderr.String())
		default:
		}
		t.Fatalf("initialize: %v", err)
	}
	tools, err := cs.ListTools(cctx, nil)
	if err != nil || len(tools.Tools) == 0 {
		t.Fatalf("tools/list: err = %v", err)
	}
	res, err := cs.CallTool(cctx, &mcp.CallToolParams{Name: "notebook_list"})
	if err != nil {
		t.Fatal(err)
	}
	body := res.Content[0].(*mcp.TextContent).Text
	if !res.IsError || !strings.Contains(body, `"code":"AUTH_REQUIRED"`) ||
		!strings.Contains(body, "could not be read") || !strings.Contains(body, "sourcebolt login") {
		t.Fatalf("notebook_list = %s", body)
	}
	if !strings.Contains(stderr.String(), "could not be read") {
		t.Fatalf("stderr = %q, want a note that the profile could not be read", stderr.String())
	}
	cs.Close()
	inW.Close()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("serve exit = %d: %s", code, stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not exit after stdin closed")
	}
}

// serve starts with an unreadable profile, but status and doctor still
// report it as unreadable.
func TestStatusAndDoctorReportUnreadableProfile(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", dir)
	t.Setenv("NOTEBOOKLM_BROWSER_FALLBACK", "0")
	t.Setenv("NOTEBOOKLM_PROFILE", "")
	t.Setenv("NOTEBOOKLM_COOKIES", "")
	t.Setenv("NOTEBOOKLM_COOKIES_FILE", "")
	t.Setenv("NOTEBOOKLM_HOST", "127.0.0.1:1")
	pdir := filepath.Join(dir, "profiles")
	if err := session.NewFileStore(pdir, session.OSKeyring{}).Save(&session.Profile{Name: "default", Jar: session.NewJar()}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "default.profile"), []byte("damaged"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	cmdStatus(nil, &out, &errb)
	if !strings.Contains(out.String(), "unreadable") {
		t.Fatalf("status = %q", out.String())
	}
	out.Reset()
	code := cmdDoctor(context.Background(), nil, &out, &errb)
	if code != 1 || !strings.Contains(out.String(), "FAIL  profile default") || !strings.Contains(out.String(), "unsupported profile file") {
		t.Fatalf("doctor = %d: %q", code, out.String())
	}
	if strings.Contains(out.String(), "token bootstrap") {
		t.Fatalf("doctor tried to bootstrap an unreadable profile: %q", out.String())
	}
}
