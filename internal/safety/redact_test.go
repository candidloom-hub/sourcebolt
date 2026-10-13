package safety

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	in := "POST f.req=%5B%5B%5B%22x%22&at=AJpMv9x:1760&f.sid=-77 Cookie: SID=abc; __Secure-1PSIDTS=def; NID=n; theme=dark Authorization: SAPISIDHASH 123_abcdef"
	out := Redact(in)
	for _, secret := range []string{"%5B%5B", "AJpMv9x", "-77", "abc", "def", "123_abcdef"} {
		if strings.Contains(out, secret) {
			t.Errorf("leaked %q in %q", secret, out)
		}
	}
	for _, keep := range []string{"SID=***", "at=***", "theme=dark"} {
		if !strings.Contains(out, keep) {
			t.Errorf("missing %q in %q", keep, out)
		}
	}
}
