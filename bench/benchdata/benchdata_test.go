package benchdata

import "testing"

func TestSizes(t *testing.T) {
	b, n := ListBodyOfSize(40_000)
	t.Logf("40KB body: %d bytes, %d rows", len(b), n)
	t.Logf("100 rows: %d bytes", len(ListBody(100)))
	if len(b) < 40_000 || len(b) > 48_000 {
		t.Fatalf("size %d", len(b))
	}
}
