package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"
)

// refusedDial returns a real connection-refused error from this OS: it
// binds a port, closes it, then dials it.
func refusedDial(t *testing.T) error {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	c, err := net.Dial("tcp", addr)
	if err == nil {
		c.Close()
		t.Skip("closed port accepted a connection")
	}
	return err
}

// A refused connection means the host is unreachable from this machine;
// the browser would hit the same wall after seconds of launch cost.
func TestRouterNoFallbackOnConnectionRefused(t *testing.T) {
	dial := refusedDial(t)
	if !ConnRefused(dial) {
		t.Fatalf("ConnRefused(%v) = false", dial)
	}
	cause := &url.Error{Op: "Post", URL: "https://notebooklm.google.com/x", Err: dial}
	pe := &Error{Kind: KindTransport, RPC: "notebooks.list", Transport: "rpc", Cause: cause, Unreachable: true}
	built := 0
	r := NewRouter(&fakeTransport{name: "rpc", err: pe}, func(context.Context) (Transport, error) {
		built++
		return &fakeTransport{name: "browser"}, nil
	})
	_, err := r.Do(t.Context(), Call{RPC: "notebooks.list", Idempotent: true})
	if !errors.Is(err, pe) || built != 0 {
		t.Fatalf("err = %v, fallback built %d times", err, built)
	}
}

func TestConnRefusedOnlyMatchesRefusal(t *testing.T) {
	for _, err := range []error{
		errors.New("connection refused"), // text alone is not enough
		&net.DNSError{Err: "no such host", Name: "x", IsNotFound: true},
		context.DeadlineExceeded,
		fmt.Errorf("HTTP 503"),
	} {
		if ConnRefused(err) {
			t.Errorf("ConnRefused(%v) = true", err)
		}
	}
}

// A refusal behind a proxy is not Unreachable: the browser may route
// differently, so the fallback still runs.
func TestRouterFallsBackOnRefusedProxy(t *testing.T) {
	pe := &Error{Kind: KindTransport, RPC: "notebooks.list", Transport: "rpc", Cause: refusedDial(t)}
	r := NewRouter(&fakeTransport{name: "rpc", err: pe}, func(context.Context) (Transport, error) {
		return &fakeTransport{name: "browser"}, nil
	})
	res, err := r.Do(t.Context(), Call{RPC: "notebooks.list", Idempotent: true})
	if err != nil || res.Transport != "browser" {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
}
