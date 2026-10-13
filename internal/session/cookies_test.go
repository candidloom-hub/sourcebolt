package session

import (
	"strings"
	"testing"
	"time"
)

func jarOf(cs ...Cookie) *Jar {
	j := NewJar()
	for _, c := range cs {
		j.Add(c)
	}
	return j
}

func ck(name, value, domain string) Cookie {
	return Cookie{Name: name, Value: value, Domain: domain, Path: "/", Expires: time.Now().Add(time.Hour)}
}

func TestRequiredCookies(t *testing.T) {
	full := jarOf(ck("SID", "s", ".google.com"), ck("__Secure-1PSIDTS", "t", ".google.com"), ck("OSID", "o", "notebook.google.com"))
	if !full.Valid() {
		t.Fatal("SID + 1PSIDTS + OSID must be valid")
	}
	alt := jarOf(ck("SID", "s", ".google.com"), ck("__Secure-1PSIDTS", "t", ".google.com"), ck("APISID", "a", ".google.com"), ck("SAPISID", "b", ".google.com"))
	if !alt.Valid() {
		t.Fatal("SID + 1PSIDTS + APISID + SAPISID must be valid")
	}
	if jarOf(ck("SID", "s", ".google.com")).Valid() {
		t.Fatal("SID-only jar must be invalid")
	}
}

func TestDomainPrecedence(t *testing.T) {
	j := jarOf(ck("SID", "host", "notebook.google.com"), ck("SID", "wide", ".google.com"))
	h := j.HeaderFor("notebook.google.com")
	if strings.Count(h, "SID=") != 1 || !strings.Contains(h, "SID=wide") {
		t.Fatalf("header = %q", h)
	}
}

func TestHostFilter(t *testing.T) {
	j := jarOf(ck("SID", "s", ".google.com"), ck("YSC", "y", ".youtube.com"), ck("OTHER", "x", "notebooklm.google.com"))
	h := j.HeaderFor("notebook.google.com")
	if strings.Contains(h, "YSC") || strings.Contains(h, "OTHER") || !strings.Contains(h, "SID=s") {
		t.Fatalf("header = %q", h)
	}
}

func TestExpiredExcluded(t *testing.T) {
	old := ck("OLD", "x", ".google.com")
	old.Expires = time.Now().Add(-time.Minute)
	if h := jarOf(old).HeaderFor("notebook.google.com"); h != "" {
		t.Fatalf("header = %q", h)
	}
}

func TestImportNetscape(t *testing.T) {
	in := "# Netscape HTTP Cookie File\n" +
		".google.com\tTRUE\t/\tTRUE\t4102444800\tSID\tsidval\n" +
		"#HttpOnly_.google.com\tTRUE\t/\tTRUE\t4102444800\t__Secure-1PSIDTS\tts\n" +
		"notebook.google.com\tFALSE\t/\tTRUE\t4102444800\tOSID\tos\n"
	j, err := ImportNetscape(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if !j.Valid() {
		t.Fatalf("jar invalid: %q", j.HeaderFor("notebook.google.com"))
	}
}

func TestImportJSONExpiryMsAndS(t *testing.T) {
	in := `[{"name":"SID","value":"a","domain":".google.com","path":"/","expirationDate":4102444800},
	        {"name":"HSID","value":"b","domain":".google.com","path":"/","expires":4102444800000}]`
	j, err := ImportJSON(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range j.All() {
		if c.Expires.Year() != 2100 {
			t.Fatalf("%s expires %v, want year 2100", c.Name, c.Expires)
		}
	}
}

func TestImportCurl(t *testing.T) {
	in := `curl 'https://notebook.google.com/' -H 'accept: */*' -H 'cookie: SID=a; __Secure-1PSIDTS=b; OSID=c'`
	j, err := ImportHeader(in, "notebook.google.com")
	if err != nil {
		t.Fatal(err)
	}
	if !j.Valid() {
		t.Fatalf("jar invalid: %q", j.HeaderFor("notebook.google.com"))
	}
	raw, err := ImportHeader("Cookie: SID=a; HSID=b", "notebook.google.com")
	if err != nil || !raw.Has("SID", "HSID") {
		t.Fatalf("raw header import failed: %v", err)
	}
}
