package main

import (
	"strings"
	"testing"
)

func TestDoctorDOMCheckNeedsNotebook(t *testing.T) {
	code, _, stderr := runCLI(t, "", "doctor", "--dom-check")
	if code != 2 || !strings.Contains(stderr, "dom-check") {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
}

// With the browser fallback off there is no DOM path to check; the doctor
// says so instead of launching anything.
func TestDoctorDOMCheckFallbackDisabled(t *testing.T) {
	code, stdout, _ := runCLI(t, "", "doctor", "--dom-check", "nb1")
	if code != 1 || !strings.Contains(stdout, "dom check") || !strings.Contains(stdout, "fallback is disabled") {
		t.Fatalf("code = %d, stdout = %q", code, stdout)
	}
}
