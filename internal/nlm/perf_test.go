package nlm

import (
	"context"
	"testing"

	"github.com/candidloom-hub/sourcebolt/bench/benchdata"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

type payloadTransport struct{ p []byte }

func (p payloadTransport) Name() string         { return "fake" }
func (p payloadTransport) Supports(string) bool { return true }
func (p payloadTransport) Do(context.Context, transport.Call) (transport.Result, error) {
	return transport.Result{Payload: p.p, Transport: "fake"}, nil
}

// B3 budget: decoding 100 notebooks must not re-parse each row per field.
func TestListNotebooksAllocBudget(t *testing.T) {
	c := New(payloadTransport{benchdata.ListPayload(100)})
	allocs := testing.AllocsPerRun(20, func() {
		if nbs, err := c.ListNotebooks(context.Background()); err != nil || len(nbs) != 100 {
			t.Fatalf("got %d notebooks, err %v", len(nbs), err)
		}
	})
	if allocs > 1000 {
		t.Fatalf("ListNotebooks(100) = %.0f allocs/op, budget 1000", allocs)
	}
	t.Logf("ListNotebooks(100) = %.0f allocs/op", allocs)
}
