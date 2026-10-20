package nlm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

func TestRefreshSourceBlocked(t *testing.T) {
	ft := newFake()
	err := New(ft).RefreshSource(context.Background(), readNB, readSID)
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindUnsupported {
		t.Fatalf("err = %v, want KindUnsupported", err)
	}
	if !strings.Contains(te.Message, "rpc-catalog") || !strings.Contains(te.Message, "item 9") {
		t.Fatalf("message %q should name rpc-catalog item 9", te.Message)
	}
	if len(ft.calls) != 0 {
		t.Fatal("blocked RPC reached the transport")
	}
}

// TestSourceGuideLive pins tr032e to live capture #2 (2026-10-20):
// request [[[[sid]]], HDR], summary at [0][0][1][0], keywords [0][0][2][0].
func TestSourceGuideLive(t *testing.T) {
	fx := loadFixture(t, "tr032e", "live_2026-10-20_01")
	ft := newFake(fx)
	sum, kws, err := New(ft).SourceGuide(context.Background(), fakeNB2, fakeSrc1)
	if err != nil {
		t.Fatal(err)
	}
	if p := paramsJSON(t, ft, "tr032e"); !sameJSON(t, []byte(p), fx.Request) {
		t.Fatalf("params %s\nwant   %s", p, fx.Request)
	}
	if sum != "text-28" || strings.Join(kws, ",") != "text-29,text-30,text-31,text-32,text-33" {
		t.Fatalf("summary %q keywords %q", sum, kws)
	}
	c, _ := ft.callFor("tr032e")
	if !c.Idempotent || c.SourcePath != "/notebook/"+fakeNB2 {
		t.Fatalf("call = %+v", c)
	}
	if _, _, err := New(newFake(inlineFixture("tr032e", `[]`))).SourceGuide(context.Background(), fakeNB2, fakeSrc1); err == nil {
		t.Fatal("a guide without a summary must be an error")
	}
	if _, _, err := New(newFake()).SourceGuide(context.Background(), fakeNB2, " "); err == nil {
		t.Fatal("empty source id must be rejected")
	}
}
