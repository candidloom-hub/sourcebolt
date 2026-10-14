package transport

import (
	"errors"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

func TestKindMapping(t *testing.T) {
	call := Call{RPC: "sources.add", ID: "izAoDd", PendingCodes: []int{3, 9}}
	plain := Call{RPC: "notebooks.list", ID: "wXbhsf"}
	cases := []struct {
		name      string
		err       error
		call      Call
		kind      Kind
		retriable bool
	}{
		{"invalid", &wire.Error{Code: 3}, plain, KindInvalidArgument, false},
		{"pending3", &wire.Error{Code: 3}, call, KindPending, false},
		{"pending9", &wire.Error{Code: 9}, call, KindPending, false},
		{"precondition", &wire.Error{Code: 9}, plain, KindInvalidArgument, false},
		{"deadline", &wire.Error{Code: 4}, plain, KindTransport, true},
		{"notfound", &wire.Error{Code: 5}, plain, KindNotFound, false},
		{"perm", &wire.Error{Code: 7}, plain, KindPermission, false},
		{"quota", &wire.Error{Code: 8}, plain, KindQuota, false},
		{"internal", &wire.Error{Code: 13}, plain, KindInternal, false},
		{"unavailable", &wire.Error{Code: 14}, plain, KindTransport, true},
		{"unauth", &wire.Error{Code: 16}, plain, KindAuth, false},
		{"private-auth", &wire.Error{Code: 277566}, plain, KindAuth, false},
		{"private-denied", &wire.Error{Code: 80620}, plain, KindPermission, false},
		{"private-rate", &wire.Error{Code: 324934}, plain, KindRateLimited, true},
		{"drift", &wire.Error{Drift: true}, plain, KindDrift, false},
		{"malformed", &wire.Error{Malformed: true}, plain, KindDrift, false},
		{"signedout", &wire.Error{SignedOut: true}, plain, KindAuth, false},
		{"er", &wire.Error{ServerErr: true}, plain, KindTransport, true},
		{"sess-signedout", session.ErrSignedOut, plain, KindAuth, false},
		{"sess-mismatch", session.ErrCookieMismatch, plain, KindCookieBinding, false},
		{"sess-drift", session.ErrPageDrift, plain, KindDrift, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := FromWire(c.err, c.call)
			if e.Kind != c.kind || e.Retriable != c.retriable {
				t.Fatalf("got kind=%v retriable=%v, want %v %v", e.Kind, e.Retriable, c.kind, c.retriable)
			}
			if e.RPC != c.call.RPC {
				t.Fatalf("RPC = %q", e.RPC)
			}
			if !errors.Is(e, c.err) {
				t.Fatal("cause must unwrap")
			}
		})
	}
}

func TestFallbackKinds(t *testing.T) {
	for k := KindAuth; k <= KindInternal; k++ {
		want := k == KindTransport || k == KindDrift || k == KindCookieBinding
		if got := k.AllowsFallback(); got != want {
			t.Errorf("%v.AllowsFallback() = %v", k, got)
		}
	}
}
