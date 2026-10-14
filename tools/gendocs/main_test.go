package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestToolsDocFresh fails when docs/tools.md no longer matches the
// registered tools. Fix: go run ./tools/gendocs > docs/tools.md
func TestToolsDocFresh(t *testing.T) {
	got, err := render(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../docs/tools.md")
	if err != nil {
		t.Fatalf("read docs/tools.md: %v (run: go run ./tools/gendocs > docs/tools.md)", err)
	}
	if strings.ReplaceAll(string(want), "\r\n", "\n") != got {
		t.Fatal("docs/tools.md is stale; run: go run ./tools/gendocs > docs/tools.md")
	}
}
