package thp

import (
	"runtime"
	"testing"
)

// Importing the package turns huge pages off on Linux (B2 idle RSS).
func TestImportDisablesHugePages(t *testing.T) {
	if got, want := Disabled(), runtime.GOOS == "linux"; got != want {
		t.Fatalf("Disabled() = %v on %s, want %v", got, runtime.GOOS, want)
	}
}
