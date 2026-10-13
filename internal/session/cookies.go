package session

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

// Cookie is a stored browser cookie. Domain starting with "." matches subdomains;
// otherwise it is host-only.
type Cookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitzero"`
	Secure   bool      `json:"secure,omitempty"`
	HTTPOnly bool      `json:"httpOnly,omitempty"`
}

func (c Cookie) expired(now time.Time) bool { return !c.Expires.IsZero() && c.Expires.Before(now) }

func (c Cookie) matches(host string) bool {
	d := strings.ToLower(c.Domain)
	host = strings.ToLower(host)
	if strings.HasPrefix(d, ".") {
		return host == d[1:] || strings.HasSuffix(host, d)
	}
	return host == d
}

// Jar is a concurrency-safe cookie jar that implements http.CookieJar and
// merges Set-Cookie responses so rotated cookies are never lost.
type Jar struct {
	mu      sync.Mutex
	cookies []Cookie
}

func NewJar() *Jar { return &Jar{} }

// Add inserts or replaces the cookie with the same name, domain and path.
func (j *Jar) Add(c Cookie) {
	if c.Path == "" {
		c.Path = "/"
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	for i, x := range j.cookies {
		if x.Name == c.Name && strings.EqualFold(x.Domain, c.Domain) && x.Path == c.Path {
			j.cookies[i] = c
			return
		}
	}
	j.cookies = append(j.cookies, c)
}

// All returns a copy of every stored cookie.
func (j *Jar) All() []Cookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	return slices.Clone(j.cookies)
}

// forHost returns one live cookie per name for host. On duplicate names a
// ".google.com" cookie wins (Google answers CookieMismatch otherwise),
// then the longer, more specific domain.
func (j *Jar) forHost(host string) []Cookie {
	now := time.Now()
	best := map[string]Cookie{}
	var order []string
	for _, c := range j.All() {
		if c.expired(now) || !c.matches(host) {
			continue
		}
		cur, seen := best[c.Name]
		if !seen {
			order = append(order, c.Name)
			best[c.Name] = c
			continue
		}
		if cur.Domain == ".google.com" {
			continue
		}
		if c.Domain == ".google.com" || len(c.Domain) > len(cur.Domain) {
			best[c.Name] = c
		}
	}
	slices.Sort(order)
	out := make([]Cookie, 0, len(order))
	for _, n := range order {
		out = append(out, best[n])
	}
	return out
}

// HeaderFor builds the Cookie header value for requests to host h.
func (j *Jar) HeaderFor(h Host) string {
	u := url.URL{Host: string(h)}
	cs := j.forHost(u.Hostname())
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = c.Name + "=" + c.Value
	}
	return strings.Join(parts, "; ")
}

// Has reports whether a live cookie exists for every name.
func (j *Jar) Has(names ...string) bool {
	now := time.Now()
	have := map[string]bool{}
	for _, c := range j.All() {
		if !c.expired(now) {
			have[c.Name] = true
		}
	}
	for _, n := range names {
		if !have[n] {
			return false
		}
	}
	return true
}

// Valid applies the minimum cookie set NotebookLM accepts:
// SID + __Secure-1PSIDTS, plus OSID or APISID+SAPISID.
func (j *Jar) Valid() bool {
	return j.Has("SID", "__Secure-1PSIDTS") && (j.Has("OSID") || j.Has("APISID", "SAPISID"))
}

// Cookies implements http.CookieJar.
func (j *Jar) Cookies(u *url.URL) []*http.Cookie {
	cs := j.forHost(u.Hostname())
	out := make([]*http.Cookie, len(cs))
	for i, c := range cs {
		out[i] = &http.Cookie{Name: c.Name, Value: c.Value}
	}
	return out
}

// SetCookies implements http.CookieJar.
func (j *Jar) SetCookies(u *url.URL, cs []*http.Cookie) {
	for _, hc := range cs {
		c := Cookie{Name: hc.Name, Value: hc.Value, Path: hc.Path, Secure: hc.Secure, HTTPOnly: hc.HttpOnly, Expires: hc.Expires}
		if hc.Domain != "" {
			c.Domain = "." + strings.TrimPrefix(hc.Domain, ".")
		} else {
			c.Domain = u.Hostname()
		}
		if hc.MaxAge > 0 {
			c.Expires = time.Now().Add(time.Duration(hc.MaxAge) * time.Second)
		} else if hc.MaxAge < 0 {
			c.Expires = time.Unix(1, 0)
		}
		j.Add(c)
	}
}

// Merge stores every Set-Cookie of resp.
func (j *Jar) Merge(resp *http.Response) {
	if resp == nil || resp.Request == nil {
		return
	}
	j.SetCookies(resp.Request.URL, resp.Cookies())
}
