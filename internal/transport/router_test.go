package transport

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type fakeTransport struct {
	name  string
	err   error
	calls atomic.Int32
}

func (f *fakeTransport) Name() string         { return f.name }
func (f *fakeTransport) Supports(string) bool { return true }
func (f *fakeTransport) Do(ctx context.Context, c Call) (Result, error) {
	f.calls.Add(1)
	if f.err != nil {
		return Result{}, f.err
	}
	return Result{Payload: jsontext.Value(`["` + f.name + `"]`), Transport: f.name}, nil
}

func allKinds() []Kind {
	var ks []Kind
	for k := KindAuth; k <= KindInternal; k++ {
		ks = append(ks, k)
	}
	return ks
}

func TestRouterFallbackPolicyAllKinds(t *testing.T) {
	if n := len(allKinds()); n != 12 {
		t.Fatalf("kinds = %d, want 12", n)
	}
	for _, k := range allKinds() {
		t.Run(k.Code(), func(t *testing.T) {
			perr := &Error{Kind: k, RPC: "notebooks.list", Transport: "rpc", Message: "primary failed"}
			primary := &fakeTransport{name: "rpc", err: perr}
			fb := &fakeTransport{name: "browser"}
			var built atomic.Int32
			r := NewRouter(primary, func(context.Context) (Transport, error) {
				built.Add(1)
				return fb, nil
			})
			res, err := r.Do(t.Context(), Call{RPC: "notebooks.list", Idempotent: true})
			want := k == KindTransport || k == KindDrift || k == KindCookieBinding
			if want {
				if err != nil {
					t.Fatalf("Do: %v", err)
				}
				if res.Transport != "browser" || r.Last() != "browser" {
					t.Errorf("transport = %q, Last = %q, want browser", res.Transport, r.Last())
				}
				if built.Load() != 1 || fb.calls.Load() != 1 {
					t.Errorf("constructor calls = %d, fallback calls = %d, want 1, 1", built.Load(), fb.calls.Load())
				}
				return
			}
			if !errors.Is(err, perr) {
				t.Fatalf("err = %v, want the primary error", err)
			}
			if built.Load() != 0 || fb.calls.Load() != 0 {
				t.Errorf("fallback touched for %s: constructor %d, calls %d", k, built.Load(), fb.calls.Load())
			}
			if r.Last() != "" {
				t.Errorf("Last = %q after failure, want empty", r.Last())
			}
		})
	}
}

func TestRouterPrimarySuccess(t *testing.T) {
	r := NewRouter(&fakeTransport{name: "rpc"}, func(context.Context) (Transport, error) {
		t.Fatal("fallback constructed on success")
		return nil, nil
	})
	res, err := r.Do(t.Context(), Call{RPC: "x"})
	if err != nil || res.Transport != "rpc" || r.Last() != "rpc" {
		t.Fatalf("res = %+v, Last = %q, err = %v", res, r.Last(), err)
	}
	if r.Name() != "router" {
		t.Errorf("Name = %q", r.Name())
	}
}

func TestRouterBothFailWrapsBothKeepsPrimaryKind(t *testing.T) {
	perr := &Error{Kind: KindDrift, RPC: "notebooks.list", Transport: "rpc", Message: "layout changed"}
	ferr := &Error{Kind: KindAuth, RPC: "notebooks.list", Transport: "browser", Message: "signed out in browser"}
	r := NewRouter(&fakeTransport{name: "rpc", err: perr}, func(context.Context) (Transport, error) {
		return &fakeTransport{name: "browser", err: ferr}, nil
	})
	_, err := r.Do(t.Context(), Call{RPC: "notebooks.list", Idempotent: true})
	if !errors.Is(err, perr) || !errors.Is(err, ferr) {
		t.Fatalf("err = %v, want both wrapped", err)
	}
	var te *Error
	if !errors.As(err, &te) || te.Kind != KindDrift {
		t.Fatalf("outer kind = %v, want DRIFT", te)
	}
	for _, s := range []string{"layout changed", "signed out in browser", "rpc", "browser"} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("message %q lacks %q", err.Error(), s)
		}
	}
	if r.Last() != "" {
		t.Errorf("Last = %q, want empty", r.Last())
	}
}

func TestRouterConstructorErrorKeepsPrimary(t *testing.T) {
	perr := &Error{Kind: KindTransport, RPC: "x", Transport: "rpc", Message: "connection reset"}
	cerr := &Error{Kind: KindUnsupported, Transport: "browser", Message: "no Chromium-based browser found"}
	var built atomic.Int32
	r := NewRouter(&fakeTransport{name: "rpc", err: perr}, func(context.Context) (Transport, error) {
		built.Add(1)
		return nil, cerr
	})
	for range 3 {
		_, err := r.Do(t.Context(), Call{RPC: "x", Idempotent: true})
		if !errors.Is(err, perr) || !errors.Is(err, cerr) {
			t.Fatalf("err = %v, want primary and constructor errors", err)
		}
		var te *Error
		if !errors.As(err, &te) || te.Kind != KindTransport {
			t.Fatalf("kind = %v, want TRANSPORT", te)
		}
		if !strings.Contains(err.Error(), "connection reset") || !strings.Contains(err.Error(), "no Chromium-based browser") {
			t.Errorf("message %q hides an error", err.Error())
		}
	}
	if built.Load() != 1 {
		t.Errorf("constructor calls = %d, want at most once", built.Load())
	}
}

func TestRouterSentNonIdempotentBlocksFallback(t *testing.T) {
	for _, tc := range []struct {
		name       string
		sent, idem bool
		fallback   bool
	}{
		{"sent non-idempotent", true, false, false},
		{"sent idempotent", true, true, true},
		{"unsent non-idempotent", false, false, true},
		{"unsent idempotent", false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			perr := &Error{Kind: KindTransport, RPC: "sources.add", Transport: "rpc", Sent: tc.sent}
			fb := &fakeTransport{name: "browser"}
			var built atomic.Int32
			r := NewRouter(&fakeTransport{name: "rpc", err: perr}, func(context.Context) (Transport, error) {
				built.Add(1)
				return fb, nil
			})
			_, err := r.Do(t.Context(), Call{RPC: "sources.add", Idempotent: tc.idem})
			if got := built.Load() == 1 && fb.calls.Load() == 1; got != tc.fallback {
				t.Fatalf("fell back = %v, want %v (err %v)", got, tc.fallback, err)
			}
			if !tc.fallback && !errors.Is(err, perr) {
				t.Fatalf("err = %v, want primary", err)
			}
		})
	}
}

func TestRouterNoFallbackOnPlainErrorOrCancelledContext(t *testing.T) {
	plain := errors.New("not a transport error")
	r := NewRouter(&fakeTransport{name: "rpc", err: plain}, func(context.Context) (Transport, error) {
		t.Fatal("constructed for a plain error")
		return nil, nil
	})
	if _, err := r.Do(t.Context(), Call{RPC: "x"}); !errors.Is(err, plain) {
		t.Fatalf("err = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	r = NewRouter(&fakeTransport{name: "rpc", err: &Error{Kind: KindTransport, Cause: context.Canceled}}, func(context.Context) (Transport, error) {
		t.Fatal("constructed after cancel")
		return nil, nil
	})
	if _, err := r.Do(ctx, Call{RPC: "x"}); err == nil {
		t.Fatal("want error")
	}
}

func TestRouterConcurrentConstructsOnce(t *testing.T) {
	var built atomic.Int32
	fb := &fakeTransport{name: "browser"}
	r := NewRouter(&fakeTransport{name: "rpc", err: &Error{Kind: KindCookieBinding}}, func(context.Context) (Transport, error) {
		built.Add(1)
		return fb, nil
	})
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			if _, err := r.Do(t.Context(), Call{RPC: "x", Idempotent: true}); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if built.Load() != 1 || fb.calls.Load() != 16 {
		t.Errorf("constructor = %d, fallback calls = %d", built.Load(), fb.calls.Load())
	}
}
