package main

import (
	"strings"
	"testing"
)

const fakeEnvHeader = "SID=fake-cli-sid-123; __Secure-1PSIDTS=fake-cli-ts-456; OSID=fake-cli-osid-789"

func TestStatusReportsEnvCookies(t *testing.T) {
	t.Setenv("NOTEBOOKLM_COOKIES", fakeEnvHeader)
	t.Setenv("NOTEBOOKLM_COOKIES_FILE", "")
	code, stdout, stderr := runCLI(t, "", "status")
	if code != 0 || !strings.Contains(stdout, "NOTEBOOKLM_COOKIES") || !strings.Contains(stdout, "in memory") || !strings.Contains(stdout, "valid_cookies=true") {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if strings.Contains(stdout+stderr, "fake-cli-") {
		t.Fatalf("status printed a cookie value: %q %q", stdout, stderr)
	}
}

func TestStatusReportsBadEnvCookies(t *testing.T) {
	t.Setenv("NOTEBOOKLM_COOKIES", "SID=fake-cli-sid-123")
	t.Setenv("NOTEBOOKLM_COOKIES_FILE", "")
	code, stdout, stderr := runCLI(t, "", "status")
	if code != 1 || !strings.Contains(stderr, "missing required cookies") {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if strings.Contains(stdout+stderr, "fake-cli-") {
		t.Fatalf("status printed a cookie value: %q %q", stdout, stderr)
	}
}

func TestDoctorReportsEnvCookies(t *testing.T) {
	t.Setenv("NOTEBOOKLM_COOKIES", fakeEnvHeader)
	t.Setenv("NOTEBOOKLM_COOKIES_FILE", "")
	t.Setenv("NOTEBOOKLM_HOST", "127.0.0.1:1") // the warm-up fails fast, offline
	_, stdout, stderr := runCLI(t, "", "doctor")
	if !strings.Contains(stdout, "cookies from NOTEBOOKLM_COOKIES") {
		t.Fatalf("stdout = %q", stdout)
	}
	if strings.Contains(stdout+stderr, "fake-cli-") {
		t.Fatalf("doctor printed a cookie value: %q %q", stdout, stderr)
	}
}
