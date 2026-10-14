package session

import (
	"bufio"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ImportNetscape parses a Netscape cookies.txt export.
func ImportNetscape(r io.Reader) (*Jar, error) {
	j := NewJar()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		httpOnly := false
		if rest, ok := strings.CutPrefix(line, "#HttpOnly_"); ok {
			line, httpOnly = rest, true
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 7 {
			continue
		}
		c := Cookie{Domain: f[0], Path: f[2], Secure: strings.EqualFold(f[3], "TRUE"), Name: f[5], Value: f[6], HTTPOnly: httpOnly}
		if strings.EqualFold(f[1], "TRUE") && !strings.HasPrefix(c.Domain, ".") {
			c.Domain = "." + c.Domain // include-subdomains flag
		}
		if n, err := strconv.ParseInt(f[4], 10, 64); err == nil && n > 0 {
			c.Expires = unixAuto(float64(n))
		}
		j.Add(c)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return nonEmpty(j)
}

type jsonCookie struct {
	Name           string  `json:"name"`
	Value          string  `json:"value"`
	Domain         string  `json:"domain"`
	Path           string  `json:"path"`
	Expires        float64 `json:"expires"`
	ExpirationDate float64 `json:"expirationDate"`
	Secure         bool    `json:"secure"`
	HTTPOnly       bool    `json:"httpOnly"`
}

// ImportJSON parses an array of cookie objects (browser-extension exports).
// Expiry may be in seconds or milliseconds.
func ImportJSON(r io.Reader) (*Jar, error) {
	var in []jsonCookie
	if err := json.UnmarshalRead(r, &in, json.RejectUnknownMembers(false)); err != nil {
		return nil, fmt.Errorf("session: cookie JSON: %w", err)
	}
	j := NewJar()
	for _, x := range in {
		c := Cookie{Name: x.Name, Value: x.Value, Domain: x.Domain, Path: x.Path, Secure: x.Secure, HTTPOnly: x.HTTPOnly}
		if e := max(x.Expires, x.ExpirationDate); e > 0 {
			c.Expires = unixAuto(e)
		}
		j.Add(c)
	}
	return nonEmpty(j)
}

var curlCookie = regexp.MustCompile(`(?i)(?:-H|--header)\s+['"]cookie:\s*([^'"]+)['"]|(?:-b|--cookie)\s+['"]([^'"]+)['"]`)

// ImportHeader parses a raw "Cookie:" header, a bare "a=b; c=d" list or a
// cURL command copied from browser dev tools.
func ImportHeader(s string, h Host) (*Jar, error) {
	s = strings.TrimSpace(s)
	if m := curlCookie.FindStringSubmatch(s); m != nil {
		s = m[1] + m[2]
	}
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(s, "Cookie:"), "cookie:"))
	domain := string(h)
	if strings.HasSuffix(domain, "google.com") {
		domain = ".google.com"
	}
	j := NewJar()
	for _, part := range strings.Split(s, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || name == "" {
			continue
		}
		j.Add(Cookie{Name: name, Value: value, Domain: domain, Path: "/"})
	}
	return nonEmpty(j)
}

// unixAuto converts seconds or milliseconds since the epoch.
func unixAuto(v float64) time.Time {
	if v > 1e11 {
		return time.UnixMilli(int64(v))
	}
	return time.Unix(int64(v), 0)
}

func nonEmpty(j *Jar) (*Jar, error) {
	if len(j.All()) == 0 {
		return nil, errors.New("session: no cookies found")
	}
	return j, nil
}
