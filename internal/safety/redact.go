// Package safety holds input checks and log redaction shared by all layers.
package safety

import "regexp"

var (
	reForm   = regexp.MustCompile(`\b(f\.req|at|f\.sid)=[^&\s]*`)
	reCookie = regexp.MustCompile(`\b(SID|HSID|SSID|APISID|SAPISID|OSID|LSID|NID|SIDCC|__Secure-[\w-]+)=[^;\s]*`)
	reHash   = regexp.MustCompile(`SAPISIDHASH\s+\S+`)
)

// Redact hides request bodies, CSRF tokens and Google session cookies.
// Every log line that may carry request data goes through it.
func Redact(s string) string {
	s = reForm.ReplaceAllString(s, "$1=***")
	s = reCookie.ReplaceAllString(s, "$1=***")
	return reHash.ReplaceAllString(s, "SAPISIDHASH ***")
}
