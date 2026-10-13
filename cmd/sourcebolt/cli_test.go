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
)

func runCLI(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", t.TempDir())
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
