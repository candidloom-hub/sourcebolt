package distcheck

import (
	"regexp"
	"strings"
	"testing"
)

// GoReleaser expands repository tokens with ApplySingleEnvOnly
// (internal/tmpl/tmpl.go): no template functions, and the whole value must
// match envOnlyRe. A snapshot or dry run never expands them, so a wrong form
// (v0.3.0 used `envOrDefault`) only fails on a real tag. This copies the
// v2.18.3 regex.
var goreleaserEnvOnly = regexp.MustCompile(`^{{\s*\.Env\.[^.\s}]+\s*}}$`)

func TestGoReleaserTokensAreSingleEnv(t *testing.T) {
	lines := strings.Split(string(read(t, ".goreleaser.yaml")), "\n")
	n := 0
	for i, line := range lines {
		k, v, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok || k != "token" {
			continue
		}
		n++
		v = strings.Trim(strings.TrimSpace(v), `'"`)
		if !goreleaserEnvOnly.MatchString(v) {
			t.Errorf(".goreleaser.yaml:%d: token %q must be exactly {{ .Env.NAME }}", i+1, v)
		}
	}
	if n == 0 {
		t.Fatal("no token: fields found in .goreleaser.yaml")
	}
}
