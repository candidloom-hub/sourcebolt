package nlm

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Fixture is the on-disk golden capture format (testdata/fixtures/<rpc>/<name>.json).
type Fixture struct {
	RPC          string         `json:"rpc"`
	Synthetic    bool           `json:"synthetic"`
	Source       string         `json:"source"`
	Request      jsontext.Value `json:"request"`
	ResponseBody string         `json:"response_body"`
	Captured     string         `json:"captured"`
	Host         string         `json:"host"`
}

func loadFixture(t *testing.T, rpc, name string) Fixture {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", rpc, name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var f Fixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// fakeTransport replays fixtures by wire id and records every call.
type fakeTransport struct {
	mu       sync.Mutex
	fixtures map[string]Fixture
	errs     map[string]error
	calls    []transport.Call
	ctxs     []context.Context
}

func newFake(fs ...Fixture) *fakeTransport {
	ft := &fakeTransport{fixtures: map[string]Fixture{}, errs: map[string]error{}}
	for _, f := range fs {
		ft.fixtures[f.RPC] = f
	}
	return ft
}

func (f *fakeTransport) Name() string         { return "fake" }
func (f *fakeTransport) Supports(string) bool { return true }

func (f *fakeTransport) Do(ctx context.Context, c transport.Call) (transport.Result, error) {
	f.mu.Lock()
	f.calls = append(f.calls, c)
	f.ctxs = append(f.ctxs, ctx)
	err, fx := f.errs[c.ID], f.fixtures[c.ID]
	f.mu.Unlock()
	if err != nil {
		return transport.Result{}, err
	}
	frames, _, err := wire.Decode([]byte(fx.ResponseBody))
	if err != nil {
		return transport.Result{}, transport.FromWire(err, c)
	}
	p, err := wire.Pick(frames, c.ID, c.Null)
	if err != nil {
		return transport.Result{}, transport.FromWire(err, c)
	}
	return transport.Result{Payload: p, Transport: "fake"}, nil
}

func (f *fakeTransport) callFor(id string) (transport.Call, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if c.ID == id {
			return c, true
		}
	}
	return transport.Call{}, false
}

// sameJSON compares two JSON texts after canonicalization.
func sameJSON(t *testing.T, a, b []byte) bool {
	t.Helper()
	ca, cb := jsontext.Value(append([]byte(nil), a...)), jsontext.Value(append([]byte(nil), b...))
	if err := ca.Canonicalize(); err != nil {
		t.Fatalf("canonicalize %s: %v", a, err)
	}
	if err := cb.Canonicalize(); err != nil {
		t.Fatalf("canonicalize %s: %v", b, err)
	}
	return string(ca) == string(cb)
}
