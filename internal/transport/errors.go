package transport

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/session"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Kind classifies a failure. The router and the MCP error envelope use it.
type Kind int

const (
	KindAuth Kind = iota
	KindCookieBinding
	KindDrift
	KindTransport
	KindRateLimited
	KindQuota
	KindPermission
	KindNotFound
	KindInvalidArgument
	KindPending
	KindUnsupported
	KindInternal
)

var kindCodes = [...]string{
	KindAuth:            "AUTH_REQUIRED",
	KindCookieBinding:   "COOKIE_BINDING",
	KindDrift:           "DRIFT",
	KindTransport:       "TRANSPORT",
	KindRateLimited:     "RATE_LIMITED",
	KindQuota:           "QUOTA",
	KindPermission:      "PERMISSION_DENIED",
	KindNotFound:        "NOT_FOUND",
	KindInvalidArgument: "INVALID_ARGUMENT",
	KindPending:         "PENDING",
	KindUnsupported:     "UNSUPPORTED_BY_TRANSPORT",
	KindInternal:        "INTERNAL",
}

// Code is the stable string used in MCP error envelopes.
func (k Kind) Code() string {
	if int(k) < len(kindCodes) {
		return kindCodes[k]
	}
	return "INTERNAL"
}

func (k Kind) String() string { return k.Code() }

// AllowsFallback is the fallback policy: only failures of the transport
// itself may switch to another transport. Quota, auth and permission
// failures follow the account, so another transport cannot fix them.
func (k Kind) AllowsFallback() bool {
	return k == KindTransport || k == KindDrift || k == KindCookieBinding
}

// Error is the one error type transports return.
type Error struct {
	Kind       Kind
	RPC        string
	GRPCCode   int
	Message    string
	Retriable  bool
	RetryAfter time.Duration
	Transport  string
	Cause      error

	// Sent reports that the failure happened after the request body was
	// written (or a response arrived), so the server may have applied the
	// call. Default false: the request never left. Ruling (P4 Task 1): the
	// Router never falls back for a non-idempotent call whose primary error
	// has Sent set, because replaying it on another transport could apply a
	// mutation twice (duplicate source, duplicate notebook).
	Sent bool

	// Unreachable reports that this machine could not reach the host at
	// all: a direct (not proxied) connection the host refused. The Router
	// does not fall back then, since the browser would fail the same way.
	// A refused proxy is not Unreachable: the browser may use another route.
	Unreachable bool
}

func (e *Error) Error() string {
	msg := e.Message
	if msg == "" && e.Cause != nil {
		msg = e.Cause.Error()
	}
	return fmt.Sprintf("%s: %s: %s", e.Kind.Code(), e.RPC, msg)
}

func (e *Error) Unwrap() error { return e.Cause }

// FromWire classifies wire and session errors for call c.
func FromWire(err error, c Call) *Error {
	e := &Error{RPC: c.RPC, Cause: err, Kind: KindInternal}
	var we *wire.Error
	var hr *session.HostRedirectError
	switch {
	case errors.As(err, &we):
		e.GRPCCode = we.Code
		switch {
		case we.Drift, we.Malformed:
			e.Kind = KindDrift
		case we.SignedOut:
			e.Kind = KindAuth
		case we.ServerErr:
			e.Kind, e.Retriable = KindTransport, true
		default:
			e.Kind, e.Retriable = fromCode(we.Code, c.PendingCodes)
		}
		if we.Status != nil {
			e.Message = we.Status.Message
		}
	case errors.Is(err, session.ErrSignedOut):
		e.Kind = KindAuth
	case errors.Is(err, session.ErrCookieMismatch):
		e.Kind = KindCookieBinding
	case errors.Is(err, session.ErrPageDrift):
		e.Kind = KindDrift
	case errors.As(err, &hr):
		e.Kind = KindAuth
	}
	return e
}

func fromCode(code int, pending []int) (Kind, bool) {
	if slices.Contains(pending, code) {
		return KindPending, false
	}
	switch code {
	case 3, 9:
		return KindInvalidArgument, false
	case 4, 14:
		return KindTransport, true
	case 5, 143:
		return KindNotFound, false
	case 7, 80620:
		return KindPermission, false
	case 8:
		return KindQuota, false
	case 16, 277566, 277567:
		return KindAuth, false
	case 324934:
		return KindRateLimited, true
	}
	return KindInternal, false
}
