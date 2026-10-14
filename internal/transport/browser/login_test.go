package browser

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/chromedp/cdproto/network"
)

func TestSignedInHost(t *testing.T) {
	const def = session.HostDefault
	tests := []struct {
		name string
		s    pageState
		want session.Host
		ok   bool
	}{
		{"notebook host with both tokens", pageState{Href: "https://notebook.google.com/", AT: true, Email: true}, def, true},
		{"landed on legacy host", pageState{Href: "https://notebooklm.google.com/notebook/x", AT: true, Email: true}, "notebooklm.google.com", true},
		{"landed on cloud host", pageState{Href: "https://notebook.cloud.google.com/", AT: true, Email: true}, "notebook.cloud.google.com", true},
		{"no email yet", pageState{Href: "https://notebook.google.com/", AT: true}, "", false},
		{"no CSRF token yet", pageState{Href: "https://notebook.google.com/", Email: true}, "", false},
		{"sign-in page", pageState{Href: "https://accounts.google.com/v3/signin/identifier", AT: true, Email: true}, "", false},
		{"other site mimicking tokens", pageState{Href: "https://evil.example/", AT: true, Email: true}, "", false},
		{"garbage url", pageState{Href: "::", AT: true, Email: true}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := signedInHost(tt.s, def)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("got %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestSignedInHostAcceptsRequestedHost(t *testing.T) {
	// A host the caller asked for (NOTEBOOKLM_HOST, local test pages) counts
	// as a notebook host.
	got, ok := signedInHost(pageState{Href: "http://127.0.0.1:5555/app", AT: true, Email: true}, "127.0.0.1:5555")
	if !ok || got != "127.0.0.1:5555" {
		t.Fatalf("got %q, %v", got, ok)
	}
}

func TestWaitSignedInRecordsLandedHost(t *testing.T) {
	states := []pageState{
		{Href: "https://accounts.google.com/signin"},
		{Href: "https://notebooklm.google.com/", AT: false, Email: true},
		{Href: "https://notebooklm.google.com/", AT: true, Email: true},
	}
	i := 0
	probe := func(context.Context) (pageState, error) {
		s := states[min(i, len(states)-1)]
		i++
		return s, nil
	}
	got, err := waitSignedIn(t.Context(), probe, session.HostDefault, time.Millisecond)
	if err != nil || got != "notebooklm.google.com" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestWaitSignedInWindowClosed(t *testing.T) {
	probe := func(context.Context) (pageState, error) { return pageState{}, errGone }
	_, err := waitSignedIn(t.Context(), probe, session.HostDefault, time.Millisecond)
	if !errors.Is(err, ErrCancelled) || !strings.Contains(err.Error(), "CANCELLED") {
		t.Fatalf("err = %v, want CANCELLED", err)
	}
}

func TestWaitSignedInRetriesTransientErrors(t *testing.T) {
	n := 0
	probe := func(context.Context) (pageState, error) {
		n++
		if n < 3 {
			return pageState{}, errors.New("Execution context was destroyed")
		}
		return pageState{Href: "https://notebook.google.com/", AT: true, Email: true}, nil
	}
	if _, err := waitSignedIn(t.Context(), probe, session.HostDefault, time.Millisecond); err != nil {
		t.Fatal(err)
	}
}

func TestWaitSignedInTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	probe := func(context.Context) (pageState, error) { return pageState{Href: "https://accounts.google.com/"}, nil }
	_, err := waitSignedIn(ctx, probe, session.HostDefault, time.Millisecond)
	if !errors.Is(err, ErrTimeout) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want ErrTimeout wrapping DeadlineExceeded", err)
	}
}

func TestWaitSignedInInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	probe := func(context.Context) (pageState, error) { return pageState{}, nil }
	_, err := waitSignedIn(ctx, probe, session.HostDefault, time.Millisecond)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("err = %v, want CANCELLED on interrupt", err)
	}
}

func TestToSessionCookies(t *testing.T) {
	exp := 1893456000.5 // 2030-01-01T00:00:00.5Z
	in := []*network.Cookie{
		{Name: "SID", Value: "v1", Domain: ".google.com", Path: "/", Expires: exp, Secure: true},
		{Name: "HSID", Value: "v2", Domain: ".google.com", Path: "/", Expires: exp, HTTPOnly: true},
		{Name: "OSID", Value: "v3", Domain: "notebook.google.com", Path: "/", Expires: exp, Secure: true, HTTPOnly: true},
		{Name: "LSID", Value: "v4", Domain: "accounts.google.com", Path: "/", Expires: exp, Secure: true, HTTPOnly: true},
		{Name: "NID", Value: "v5", Domain: ".google.com", Path: "/", Expires: -1, Session: true},
		{Name: "TRACK", Value: "x", Domain: ".youtube.com", Path: "/"},
		{Name: "OTHER", Value: "x", Domain: "example.com", Path: "/"},
		{Name: "MAIL", Value: "x", Domain: "mail.google.com", Path: "/"},
		nil,
	}
	got := toSessionCookies(in, cookieHosts(session.HostDefault, session.HostDefault))
	byName := map[string]session.Cookie{}
	for _, c := range got {
		byName[c.Name] = c
	}
	for _, n := range []string{"TRACK", "OTHER", "MAIL"} {
		if _, ok := byName[n]; ok {
			t.Errorf("%s must be filtered out (not sent to a NotebookLM or Google sign-in host)", n)
		}
	}
	want := map[string]session.Cookie{
		"SID":  {Name: "SID", Value: "v1", Domain: ".google.com", Path: "/", Expires: time.Unix(1893456000, 5e8), Secure: true},
		"HSID": {Name: "HSID", Value: "v2", Domain: ".google.com", Path: "/", Expires: time.Unix(1893456000, 5e8), HTTPOnly: true},
		"OSID": {Name: "OSID", Value: "v3", Domain: "notebook.google.com", Path: "/", Expires: time.Unix(1893456000, 5e8), Secure: true, HTTPOnly: true},
		"LSID": {Name: "LSID", Value: "v4", Domain: "accounts.google.com", Path: "/", Expires: time.Unix(1893456000, 5e8), Secure: true, HTTPOnly: true},
		"NID":  {Name: "NID", Value: "v5", Domain: ".google.com", Path: "/"},
	}
	for n, w := range want {
		g, ok := byName[n]
		if !ok {
			t.Errorf("%s missing", n)
			continue
		}
		if g.Name != w.Name || g.Value != w.Value || g.Domain != w.Domain || g.Path != w.Path ||
			g.Secure != w.Secure || g.HTTPOnly != w.HTTPOnly || !g.Expires.Equal(w.Expires) {
			// Never print values: only the metadata.
			t.Errorf("%s: domain=%q path=%q secure=%v httpOnly=%v expires=%v; want domain=%q path=%q secure=%v httpOnly=%v expires=%v",
				n, g.Domain, g.Path, g.Secure, g.HTTPOnly, g.Expires, w.Domain, w.Path, w.Secure, w.HTTPOnly, w.Expires)
		}
	}
}

func TestCookieHosts(t *testing.T) {
	got := cookieHosts("127.0.0.1:5555", "notebooklm.google.com")
	for _, h := range []string{"notebook.google.com", "127.0.0.1", "notebooklm.google.com", "accounts.google.com", "www.google.com"} {
		if !slices.Contains(got, h) {
			t.Errorf("cookieHosts missing %s: %v", h, got)
		}
	}
}

func TestJarFromCookiesValidity(t *testing.T) {
	exp := float64(time.Now().Add(24 * time.Hour).Unix())
	in := []*network.Cookie{
		{Name: "SID", Value: "a", Domain: ".google.com", Path: "/", Expires: exp},
		{Name: "__Secure-1PSIDTS", Value: "b", Domain: ".google.com", Path: "/", Expires: exp, Secure: true, HTTPOnly: true},
		{Name: "APISID", Value: "c", Domain: ".google.com", Path: "/", Expires: exp},
		{Name: "SAPISID", Value: "d", Domain: ".google.com", Path: "/", Expires: exp, Secure: true},
	}
	j := jarFromCookies(in, session.HostDefault, session.HostDefault)
	if !j.Valid() {
		t.Fatalf("jar should be valid, missing %v", j.Missing())
	}
	j = jarFromCookies(in[:1], session.HostDefault, session.HostDefault)
	if j.Valid() || len(j.Missing()) == 0 {
		t.Fatal("a jar with only SID must not be valid")
	}
}
