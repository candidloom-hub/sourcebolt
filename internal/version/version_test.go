package version

import "testing"

func TestString(t *testing.T) {
	if got := String(); got != "sourcebolt dev" {
		t.Fatalf("String() = %q, want %q", got, "sourcebolt dev")
	}
}
