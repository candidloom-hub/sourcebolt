package safety

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
)

// Resolver is the part of *net.Resolver that CheckPublicURL needs.
// *net.Resolver satisfies it; tests inject a fake. A nil Resolver means
// net.DefaultResolver.
type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// ErrBlockedURL wraps every URL rejection so callers can map it to
// INVALID_ARGUMENT with errors.Is.
var ErrBlockedURL = errors.New("url not allowed")

func blocked(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrBlockedURL, fmt.Sprintf(format, a...))
}

// Extra ranges that netip's predicates do not cover.
var blockedPrefixes = mustPrefixes(
	"0.0.0.0/8",     // "this network"
	"100.64.0.0/10", // CGNAT
	"192.0.0.0/24",  // IETF protocol assignments
	"198.18.0.0/15", // benchmarking
	"240.0.0.0/4",   // reserved + broadcast
	"64:ff9b::/96",  // NAT64 (embedded v4 checked separately)
	"100::/64",      // discard-only
	"2001:db8::/32", // documentation
	"2002::/16",     // 6to4 (embeds arbitrary v4)
)

func mustPrefixes(ss ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(ss))
	for i, s := range ss {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}

// numericLabel matches decimal, hex or octal-looking labels that legacy
// inet_aton parsers turn into IPv4 addresses (2130706433, 0x7f000001, 127.1).
var numericLabel = regexp.MustCompile(`^(0[xX][0-9a-fA-F]*|[0-9]+)$`)

// ipBlocked reports whether a is not a public unicast address.
func ipBlocked(a netip.Addr) bool {
	a = a.Unmap().WithZone("")
	if a.Is6() {
		// NAT64: judge the embedded IPv4 address.
		if netip.MustParsePrefix("64:ff9b::/96").Contains(a) {
			b := a.As16()
			if ipBlocked(netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]})) {
				return true
			}
		}
	}
	if !a.IsValid() || a.IsLoopback() || a.IsPrivate() || a.IsUnspecified() ||
		a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() ||
		a.IsInterfaceLocalMulticast() || a.IsMulticast() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// CheckPublicURL parses raw and returns it only when it is an http(s) URL
// without credentials whose host is, or resolves entirely to, public
// addresses. Every resolved address must be public. The check is a
// pre-flight filter: it does not pin the connection to the checked IPs, so
// callers that fetch the URL themselves must also guard the dial.
func CheckPublicURL(ctx context.Context, raw string, r Resolver) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, blocked("unparseable: %v", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return nil, blocked("scheme %q (only http and https)", u.Scheme)
	}
	if u.User != nil {
		return nil, blocked("credentials in URL")
	}
	host := strings.TrimSuffix(u.Hostname(), ".")
	if host == "" {
		return nil, blocked("missing host")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return nil, blocked("host %q", host)
	}

	if a, err := netip.ParseAddr(host); err == nil {
		if ipBlocked(a) {
			return nil, blocked("address %s is not public", a)
		}
		return u, nil
	}
	// Legacy numeric forms never reach the resolver.
	numeric := true
	for _, l := range strings.Split(host, ".") {
		if !numericLabel.MatchString(l) {
			numeric = false
			break
		}
	}
	if numeric {
		return nil, blocked("numeric host %q", host)
	}

	if r == nil {
		r = net.DefaultResolver
	}
	addrs, err := r.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, blocked("resolve %q: %v", host, err)
	}
	if len(addrs) == 0 {
		return nil, blocked("host %q resolves to nothing", host)
	}
	for _, ia := range addrs {
		a, ok := netip.AddrFromSlice(ia.IP)
		if !ok || ipBlocked(a) {
			return nil, blocked("host %q resolves to non-public address %s", host, ia.IP)
		}
	}
	return u, nil
}
