package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Tokens are the per-page values every RPC needs.
type Tokens struct {
	AT      string // SNlM0e, CSRF token sent as "at"
	FSID    string // FdrFJe, sent as "f.sid"
	BL      string // cfb2h, build label sent as "bl"
	HL      string // <html lang>, sent as "hl"
	Email   string // oPEP7c
	Fetched time.Time
}

var (
	ErrSignedOut      = errors.New("session: signed out")
	ErrCookieMismatch = errors.New("session: cookie mismatch (cookies bound to another client)")
	ErrPageDrift      = errors.New("session: page tokens not found (page layout changed)")
)

// HostRedirectError means the account was redirected to another notebook host.
type HostRedirectError struct{ To Host }

func (e *HostRedirectError) Error() string { return "session: account redirected to " + string(e.To) }

const maxPage = 8 << 20

// quote matches ", ', \x22 or &quot; around keys and values in WIZ_global_data.
const quote = `(?:\\x22|&quot;|["'])`

func tokenRe(key string) *regexp.Regexp {
	return regexp.MustCompile(quote + key + quote + `\s*:\s*` + quote + `(.*?)` + quote)
}

var (
	reAT    = tokenRe("SNlM0e")
	reFSID  = tokenRe("FdrFJe")
	reBL    = tokenRe("cfb2h")
	reEmail = tokenRe("oPEP7c")
	reLang  = regexp.MustCompile(`<html[^>]*\slang="([^"]+)"`)
)

// Bootstrap fetches https://<h>/ with the client's cookies and extracts tokens.
// Redirects are never followed: a redirect to accounts.google.com means the
// session is gone, and following it would hide that.
func Bootstrap(ctx context.Context, c *http.Client, h Host) (Tokens, error) {
	nc := *c
	nc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.Origin()+"/", nil)
	if err != nil {
		return Tokens{}, err
	}
	resp, err := nc.Do(req)
	if err != nil {
		return Tokens{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return Tokens{}, classifyRedirect(resp.Header.Get("Location"))
	}
	if resp.StatusCode != http.StatusOK {
		return Tokens{}, fmt.Errorf("session: GET / returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPage))
	if err != nil {
		return Tokens{}, err
	}
	page := string(body)
	tok := Tokens{
		AT:      find(reAT, page),
		FSID:    find(reFSID, page),
		BL:      find(reBL, page),
		HL:      find(reLang, page),
		Email:   find(reEmail, page),
		Fetched: time.Now(),
	}
	if tok.AT == "" {
		if tok.Email == "" {
			// No CSRF token and no account: Google served the signed-out
			// app shell with 200 instead of redirecting to sign-in.
			return tok, fmt.Errorf("%w (signed-out page)", ErrSignedOut)
		}
		return tok, ErrPageDrift
	}
	return tok, nil
}

func find(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

func classifyRedirect(loc string) error {
	u, err := url.Parse(loc)
	if err != nil {
		return ErrSignedOut
	}
	switch {
	case u.Hostname() == "accounts.google.com" && strings.Contains(u.Path, "CookieMismatch"):
		return ErrCookieMismatch
	case u.Hostname() == "accounts.google.com":
		return ErrSignedOut
	case IsNotebookHost(u):
		return &HostRedirectError{To: Host(u.Host)}
	}
	return fmt.Errorf("%w: unexpected redirect to %s", ErrSignedOut, u.Host)
}
