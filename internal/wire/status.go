package wire

import (
	"encoding/json/jsontext"
	"fmt"
)

// Status is a google.rpc.Status found at position 5 of a wrb.fr frame.
// Code is a gRPC code: 3 INVALID_ARGUMENT, 5 NOT_FOUND, 7 PERMISSION_DENIED,
// 8 RESOURCE_EXHAUSTED, 9 FAILED_PRECONDITION, 16 UNAUTHENTICATED, etc.
type Status struct {
	Code    int
	Message string
	Details []jsontext.Value
}

// Error is a wire-level failure. The transport layer maps it to error kinds.
type Error struct {
	Code      int    // gRPC or numeric body code; 0 if none
	ID        string // RPC id involved
	Msg       string
	Status    *Status
	Drift     bool // response had frames, but not for this id
	SignedOut bool // no frames at all, or an HTML page
	ServerErr bool // ["er", id, code] frame
	Malformed bool
}

func (e *Error) Error() string {
	switch {
	case e.Drift:
		return fmt.Sprintf("wire: rpc %s missing from response (drift)", e.ID)
	case e.SignedOut:
		return "wire: no RPC frames (signed out or blocked)"
	case e.ServerErr:
		return fmt.Sprintf("wire: server error frame for %s (code %d)", e.ID, e.Code)
	case e.Malformed:
		return "wire: malformed response: " + e.Msg
	case e.Code != 0:
		return fmt.Sprintf("wire: rpc %s rejected with code %d %s", e.ID, e.Code, e.Msg)
	}
	return "wire: " + e.Msg
}

func parseStatus(v jsontext.Value) *Status {
	n, ok := Int(v, 0)
	if !ok {
		return nil
	}
	s := &Status{Code: int(n), Message: Str(v, 1)}
	if d, ok := At(v, 2); ok {
		s.Details, _ = elems(d)
	}
	return s
}
