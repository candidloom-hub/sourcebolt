// Package runtimecfg holds the Go runtime settings the server runs with, so
// the binary and the benchmarks that stand in for it use the same ones.
package runtimecfg

import (
	"os"
	"runtime/debug"
)

// GCPercent is the server's GOGC. A tool call allocates about 0.5 MB, most
// of it short-lived go-sdk decode buffers, over a live heap of 1-2 MB, so
// the default 100 ran a collection every few calls. 200 cut the cached
// notebook_list call by about 23% (B3, interleaved, p = 0.002) and did not
// raise idle RSS (bench/rss).
const GCPercent = 200

// Apply sets GCPercent unless the operator set GOGC, and returns a func that
// restores the previous value (for tests and benchmarks).
func Apply() (restore func()) {
	if os.Getenv("GOGC") != "" {
		return func() {}
	}
	prev := debug.SetGCPercent(GCPercent)
	return func() { debug.SetGCPercent(prev) }
}
