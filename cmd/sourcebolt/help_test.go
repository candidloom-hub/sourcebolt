package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// The installers detect `setup` with `sourcebolt setup --help`, so asking a subcommand for help
// must succeed (exit 0) and print its flags; a wrong flag is still a usage error (exit 2).
// status takes no arguments: a stray word is a usage error, not a status run.
func TestStatusRejectsArguments(t *testing.T) {
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", t.TempDir())
	t.Setenv("NOTEBOOKLM_BROWSER_FALLBACK", "0")
	var out, errOut bytes.Buffer
	if code := run(context.Background(), []string{"status", "extra"}, strings.NewReader(""), &out, &errOut); code != 2 {
		t.Fatalf("status extra: exit %d, want 2 (stdout %q)", code, out.String())
	}
	if out.Len() != 0 {
		t.Fatalf("status extra printed a status: %q", out.String())
	}
}

func TestSubcommandHelpExitsZero(t *testing.T) {
	t.Setenv("NOTEBOOKLM_CONFIG_DIR", t.TempDir())
	t.Setenv("NOTEBOOKLM_BROWSER_FALLBACK", "0")
	for _, cmd := range []string{"setup", "serve", "login", "logout", "doctor", "status"} {
		for _, flag := range []string{"--help", "-h"} {
			var out, errOut bytes.Buffer
			code := run(context.Background(), []string{cmd, flag}, strings.NewReader(""), &out, &errOut)
			if code != 0 {
				t.Errorf("%s %s: exit %d, want 0 (stderr: %q)", cmd, flag, code, errOut.String())
			}
			if !strings.Contains(out.String()+errOut.String(), "Usage of "+cmd) {
				t.Errorf("%s %s: no usage text printed", cmd, flag)
			}
		}
		var out, errOut bytes.Buffer
		if code := run(context.Background(), []string{cmd, "--no-such-flag"}, strings.NewReader(""), &out, &errOut); code != 2 {
			t.Errorf("%s --no-such-flag: exit %d, want 2", cmd, code)
		}
	}
}
