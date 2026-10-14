package wire_test

import (
	"testing"

	"github.com/candidloom-hub/sourcebolt/bench/benchdata"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// BenchmarkDecode40KB is B4: decode a ~40 KB batchexecute body built from
// the live wXbhsf row structure (target: p50 < 1 ms).
func BenchmarkDecode40KB(b *testing.B) {
	body, rows := benchdata.ListBodyOfSize(40_000)
	b.Logf("body=%d bytes, %d notebook rows", len(body), rows)
	b.ReportAllocs()
	for b.Loop() {
		frames, _, err := wire.Decode(body)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := wire.Pick(frames, "wXbhsf", wire.NullAllowed); err != nil {
			b.Fatal(err)
		}
	}
}
