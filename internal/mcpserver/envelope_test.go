package mcpserver

import (
	"encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

// A rate limit raised locally (no transport answered, e.g. the detached-job
// cap) must not blame Google.
func TestLocalRateLimitGuidance(t *testing.T) {
	var ee *envelopeError
	local := toEnvelope(&transport.Error{Kind: transport.KindRateLimited, RPC: "chat.ask", Message: "16 detached jobs running"})
	if !errors.As(local, &ee) || strings.Contains(ee.env.Guidance, "Google") || !strings.Contains(ee.env.Guidance, "job") {
		b, _ := json.Marshal(ee.env)
		t.Fatalf("local guidance = %s", b)
	}
	remote := toEnvelope(&transport.Error{Kind: transport.KindRateLimited, RPC: "notebooks.list", Transport: "rpc"})
	if !errors.As(remote, &ee) || !strings.Contains(ee.env.Guidance, "Google") {
		t.Fatalf("remote guidance = %q", ee.env.Guidance)
	}
}
