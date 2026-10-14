//go:build live

package nlm_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/app"
)

// TestLiveListNotebooks runs against real NotebookLM with the profile the
// owner imported (sourcebolt login --import). Run manually:
//
//	go test -tags live ./internal/nlm -run TestLiveListNotebooks -v
func TestLiveListNotebooks(t *testing.T) {
	cfg, err := app.ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	rt, err := app.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !rt.Profile.Jar.Valid() {
		t.Skip("no valid profile; run sourcebolt login --import first")
	}
	ctx := context.Background()
	if err := rt.Transport.Warm(ctx); err != nil {
		t.Fatal(err)
	}
	var rtts []time.Duration
	for range 10 {
		start := time.Now()
		nbs, err := rt.Deps.NLM.ListNotebooks(ctx)
		if err != nil {
			t.Fatal(err)
		}
		rtts = append(rtts, time.Since(start))
		if len(rtts) == 1 {
			t.Logf("%d notebooks", len(nbs))
		}
	}
	sort.Slice(rtts, func(i, j int) bool { return rtts[i] < rtts[j] })
	t.Logf("list notebooks round trip over 10 calls: p50 %v, p95 %v", rtts[4], rtts[9])
}
