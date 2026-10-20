package app

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/candidloom-hub/sourcebolt/internal/session"
)

// Sign-in without `sourcebolt login`, for containers (Docker MCP Toolkit,
// docker run): the cookie export itself, or the path of a file holding it
// (a Docker secret under /run/secrets). Same formats as `login --import`.
const (
	EnvCookies     = "NOTEBOOKLM_COOKIES"
	EnvCookiesFile = "NOTEBOOKLM_COOKIES_FILE"
)

const maxCookieExport = 4 << 20 // same limit as `login --import -`

// EnvProfile builds an in-memory profile from EnvCookies or
// EnvCookiesFile, and the name of the variable used. It returns nil, "",
// nil when neither is set (an empty value counts as unset: catalogs pass
// "" for a secret nobody filled in). Build calls it; call it directly
// only where Build is not used (status).
//
// Both variables are removed from the process environment before
// anything else happens, valid or not, so a child process (the fallback
// browser) never inherits the cookies.
//
// Errors name the variable and, for an incomplete jar, the missing cookie
// names. They never include the input: a parser error can quote it.
func EnvProfile(c Config) (*session.Profile, string, error) {
	raw, fromEnv := os.LookupEnv(EnvCookies)
	path, fromFile := os.LookupEnv(EnvCookiesFile)
	os.Unsetenv(EnvCookies)
	os.Unsetenv(EnvCookiesFile)
	fromEnv = fromEnv && strings.TrimSpace(raw) != ""
	fromFile = fromFile && strings.TrimSpace(path) != ""

	var src string
	switch {
	case fromEnv && fromFile:
		return nil, "", fmt.Errorf("both %s and %s are set; set only one", EnvCookies, EnvCookiesFile)
	case fromEnv:
		src = EnvCookies
	case fromFile:
		src = EnvCookiesFile
		f, err := os.Open(strings.TrimSpace(path))
		if err != nil {
			return nil, "", fmt.Errorf("%s: %w", EnvCookiesFile, err)
		}
		b, err := io.ReadAll(io.LimitReader(f, maxCookieExport))
		f.Close()
		if err != nil {
			return nil, "", fmt.Errorf("%s: %w", EnvCookiesFile, err)
		}
		raw = string(b)
	default:
		return nil, "", nil
	}

	h := c.Host
	if h == "" {
		h = session.HostDefault
	}
	jar, err := session.ParseExport([]byte(raw), "", h)
	if err != nil {
		return nil, "", fmt.Errorf("%s: no cookies could be read; use a Netscape cookies.txt, a JSON cookie array or a Cookie header (the parser detail is withheld because it can quote cookie values)", src)
	}
	if !jar.Valid() {
		return nil, "", fmt.Errorf("%s is missing required cookies: %s. Export every google.com cookie (sourcebolt needs SID, __Secure-1PSIDTS, and OSID or APISID+SAPISID), not only the __Secure- ones", src, strings.Join(jar.Missing(), ", "))
	}
	return &session.Profile{Name: c.Profile, Host: h, Jar: jar}, src, nil
}
