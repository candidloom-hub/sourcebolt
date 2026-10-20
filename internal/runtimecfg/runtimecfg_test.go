package runtimecfg

import (
	"runtime/debug"
	"testing"
)

func TestApplySetsGCPercentUnlessGOGCIsSet(t *testing.T) {
	t.Setenv("GOGC", "")
	restore := Apply()
	if got := debug.SetGCPercent(-1); got != GCPercent {
		t.Fatalf("GC percent = %d, want %d", got, GCPercent)
	}
	debug.SetGCPercent(GCPercent)
	restore()
	if got := debug.SetGCPercent(100); got != 100 {
		t.Fatalf("after restore GC percent = %d, want 100", got)
	}

	t.Setenv("GOGC", "50")
	debug.SetGCPercent(50)
	Apply()()
	if got := debug.SetGCPercent(100); got != 50 {
		t.Fatalf("GOGC=50 set: GC percent = %d, want 50 (left alone)", got)
	}
}
