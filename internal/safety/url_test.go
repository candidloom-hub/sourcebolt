package safety

import (
	"context"
	"errors"
	"net"
	"testing"
)

type fakeResolver map[string][]string

func (f fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	ips, ok := f[host]
	if !ok {
		return nil, errors.New("no such host")
	}
	var out []net.IPAddr
	for _, s := range ips {
		out = append(out, net.IPAddr{IP: net.ParseIP(s)})
	}
	return out, nil
}

func TestCheckPublicURL(t *testing.T) {
	res := fakeResolver{
		"example.com":   {"93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"},
		"internal.corp": {"192.168.1.5"},
		"mixed.test":    {"93.184.216.34", "10.0.0.1"},
		"meta.test":     {"169.254.169.254"},
		"v6loop.test":   {"::1"},
		"mapped.test":   {"::ffff:127.0.0.1"},
		"empty.test":    {},
	}
	cases := []struct {
		name, raw string
		ok        bool
	}{
		{"https public", "https://example.com/a", true},
		{"http public", "http://example.com/a?b=1", true},
		{"public ipv4 literal", "https://93.184.216.34/", true},
		{"public ipv6 literal", "https://[2606:2800:220:1:248:1893:25c8:1946]/", true},
		{"file", "file:///etc/passwd", false},
		{"javascript", "javascript:alert(1)", false},
		{"ftp", "ftp://example.com/x", false},
		{"data", "data:text/plain,hi", false},
		{"empty", "", false},
		{"no host", "http:///x", false},
		{"relative", "example.com/x", false},
		{"loopback v4", "http://127.0.0.1/", false},
		{"loopback v4 other", "http://127.5.5.5:8080/", false},
		{"loopback v6", "http://[::1]/", false},
		{"localhost", "http://localhost/", false},
		{"rfc1918 10", "http://10.0.0.1/", false},
		{"rfc1918 172", "http://172.16.0.1/", false},
		{"rfc1918 172 top", "http://172.31.255.255/", false},
		{"rfc1918 192", "http://192.168.0.1/", false},
		{"ula v6", "http://[fc00::1]/", false},
		{"ula v6 fd", "http://[fd12:3456::1]/", false},
		{"metadata", "http://169.254.169.254/latest/meta-data", false},
		{"link-local v4", "http://169.254.1.1/", false},
		{"link-local v6", "http://[fe80::1]/", false},
		{"unspecified v4", "http://0.0.0.0/", false},
		{"unspecified v6", "http://[::]/", false},
		{"multicast v4", "http://224.0.0.1/", false},
		{"multicast v6", "http://[ff02::1]/", false},
		{"cgnat", "http://100.64.0.1/", false},
		{"cgnat top", "http://100.127.255.255/", false},
		{"just outside cgnat", "http://100.128.0.1/", true},
		{"mapped loopback", "http://[::ffff:127.0.0.1]/", false},
		{"mapped private", "http://[::ffff:10.0.0.1]/", false},
		{"mapped metadata", "http://[::ffff:169.254.169.254]/", false},
		{"nat64 loopback", "http://[64:ff9b::7f00:1]/", false},
		{"decimal ip", "http://2130706433/", false},
		{"hex ip", "http://0x7f000001/", false},
		{"octal-ish ip", "http://0177.0.0.1/", false},
		{"short ip", "http://127.1/", false},
		{"userinfo", "https://user:pw@example.com/", false},
		{"userinfo only user", "https://user@example.com/", false},
		{"resolves private", "https://internal.corp/", false},
		{"resolves mixed", "https://mixed.test/", false},
		{"resolves metadata", "https://meta.test/", false},
		{"resolves v6 loopback", "https://v6loop.test/", false},
		{"resolves mapped", "https://mapped.test/", false},
		{"resolves nothing", "https://empty.test/", false},
		{"resolve failure", "https://nonexistent.invalid/", false},
		{"trailing dot localhost", "http://localhost./", false},
		{"uppercase scheme", "HTTPS://example.com/", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u, err := CheckPublicURL(context.Background(), c.raw, res)
			if c.ok {
				if err != nil || u == nil {
					t.Fatalf("want ok, got u=%v err=%v", u, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want rejection, got %v", u)
			}
			if u != nil {
				t.Errorf("want nil url on error, got %v", u)
			}
		})
	}
}

func TestCheckPublicURLNilResolverLiteralOnly(t *testing.T) {
	// IP literals never touch the resolver, so a nil one must be fine.
	if _, err := CheckPublicURL(context.Background(), "https://93.184.216.34/", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckPublicURL(context.Background(), "http://10.0.0.1/", nil); err == nil {
		t.Fatal("want rejection")
	}
}

func TestCheckPublicURLAcceptsNetResolverType(t *testing.T) {
	var r *net.Resolver = net.DefaultResolver
	if _, err := CheckPublicURL(context.Background(), "http://127.0.0.1/", r); err == nil {
		t.Fatal("want rejection")
	}
}
