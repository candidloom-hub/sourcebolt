package main

import "testing"

func TestLoopbackOrigin(t *testing.T) {
	for o, want := range map[string]bool{
		"http://127.0.0.1":            true,
		"http://127.0.0.1:8080":       true,
		"http://localhost":            true,
		"http://localhost:3000":       true,
		"https://localhost:3000":      true,
		"http://[::1]:9000":           true,
		"http://localhost.evil.com":   false,
		"http://127.0.0.1.evil.com":   false,
		"http://localhost@evil.com":   false,
		"http://evil.com/?localhost":  false,
		"https://evil.com":            false,
		"null":                        false,
		"file://localhost":            false,
		"http://localhostevil.com:80": false,
	} {
		if got := loopbackOrigin(o); got != want {
			t.Errorf("loopbackOrigin(%q) = %v, want %v", o, got, want)
		}
	}
}
