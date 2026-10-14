package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func pageServer(t *testing.T, h http.HandlerFunc) (*httptest.Server, Host) {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	return srv, Host(srv.Listener.Addr().String())
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "_page", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBootstrapParsesTokens(t *testing.T) {
	cases := []struct{ file, at, fsid, bl, email, hl string }{
		{"home_dq.html", "AJpMv9x:1760400000000", "-7712345", "boq_labs-tailwind-frontend_20261008.06_p0", "user@example.com", "en-GB"},
		{"home_sq.html", "AJsq:1", "-55", "boq_sq_1", "sq@example.com", "de"},
		{"home_escaped.html", "AJesc:2", "-99", "boq_esc_2", "esc@example.com", "ja"},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			page := fixture(t, c.file)
			srv, h := pageServer(t, func(w http.ResponseWriter, r *http.Request) { w.Write(page) })
			tok, err := Bootstrap(context.Background(), srv.Client(), h)
			if err != nil {
				t.Fatal(err)
			}
			if tok.AT != c.at || tok.FSID != c.fsid || tok.BL != c.bl || tok.Email != c.email || tok.HL != c.hl {
				t.Fatalf("got %+v", tok)
			}
		})
	}
}

func redirectTo(loc string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, loc, http.StatusFound) }
}

func TestBootstrapRedirectSignedOut(t *testing.T) {
	srv, h := pageServer(t, redirectTo("https://accounts.google.com/ServiceLogin?continue=x"))
	if _, err := Bootstrap(context.Background(), srv.Client(), h); !errors.Is(err, ErrSignedOut) {
		t.Fatalf("got %v", err)
	}
}

func TestBootstrapCookieMismatch(t *testing.T) {
	srv, h := pageServer(t, redirectTo("https://accounts.google.com/CookieMismatch"))
	if _, err := Bootstrap(context.Background(), srv.Client(), h); !errors.Is(err, ErrCookieMismatch) {
		t.Fatalf("got %v", err)
	}
}

func TestBootstrapHostRedirect(t *testing.T) {
	srv, h := pageServer(t, redirectTo("https://notebook.google.com/"))
	_, err := Bootstrap(context.Background(), srv.Client(), h)
	var hr *HostRedirectError
	if !errors.As(err, &hr) || hr.To != "notebook.google.com" {
		t.Fatalf("got %v", err)
	}
}

func TestBootstrapNoTokens(t *testing.T) {
	page := fixture(t, "no_tokens.html")
	srv, h := pageServer(t, func(w http.ResponseWriter, r *http.Request) { w.Write(page) })
	tok, err := Bootstrap(context.Background(), srv.Client(), h)
	if !errors.Is(err, ErrPageDrift) {
		t.Fatalf("got %v", err)
	}
	if tok.AT != "" {
		t.Fatalf("AT must stay empty, never fall back to FdrFJe: %q", tok.AT)
	}
}

func TestIsNotebookHost(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://notebook.google.com/notebook/x":   true,
		"https://notebooklm.google.com/":           true,
		"https://notebook.cloud.google.com/":       true,
		"https://notebooklm.google.com.evil.com/":  false,
		"https://evil.com/notebook.google.com":     false,
		"https://accounts.google.com/ServiceLogin": false,
	} {
		u, _ := url.Parse(raw)
		if got := IsNotebookHost(u); got != want {
			t.Errorf("%s: got %v want %v", raw, got, want)
		}
	}
}

// Live behavior 2026-10-14: with incomplete cookies Google answers 200 with
// the signed-out app shell (WIZ_global_data present, no SNlM0e, no email).
func TestBootstrapSignedOutShell(t *testing.T) {
	page := fixture(t, "signed_out_shell.html")
	srv, h := pageServer(t, func(w http.ResponseWriter, r *http.Request) { w.Write(page) })
	if _, err := Bootstrap(context.Background(), srv.Client(), h); !errors.Is(err, ErrSignedOut) {
		t.Fatalf("got %v, want ErrSignedOut", err)
	}
}
