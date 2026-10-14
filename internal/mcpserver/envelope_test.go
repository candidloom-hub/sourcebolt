package mcpserver

import (
	"encoding/json/v2"
	"errors"
	"strings"
	"testing"
	"time"

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

// Google's Retry-After reaches the agent as whole seconds, rounded up so
// an agent that waits exactly that long is not early.
func TestRetryAfterInEnvelope(t *testing.T) {
	var ee *envelopeError
	err := toEnvelope(&transport.Error{Kind: transport.KindRateLimited, Transport: "rpc", Retriable: true, RetryAfter: 2500 * time.Millisecond})
	if !errors.As(err, &ee) || ee.env.RetryAfterSeconds != 3 {
		t.Fatalf("env = %+v", ee.env)
	}
	if b, _ := json.Marshal(ee.env); !strings.Contains(string(b), `"retry_after_seconds":3`) {
		t.Fatalf("json = %s", b)
	}
	none := toEnvelope(&transport.Error{Kind: transport.KindTransport, Transport: "rpc"})
	if b, _ := json.Marshal(none.(*envelopeError).env); strings.Contains(string(b), "retry_after") {
		t.Fatalf("json without RetryAfter = %s", b)
	}
}
