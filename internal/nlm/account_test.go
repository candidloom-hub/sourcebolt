package nlm

import (
	"context"
	"encoding/json/v2"
	"testing"
	"time"
)

func TestQuotaLive(t *testing.T) {
	fx := loadFixture(t, "EylDcb", "live_2026-10-13_01")
	if got, _ := json.Marshal(quotaParams()); fx.Synthetic || !sameJSON(t, got, fx.Request) {
		t.Fatalf("builder %s want %s", got, fx.Request)
	}
	ft := newFake(fx)
	q, err := New(ft).Quota(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Windows) != 2 {
		t.Fatalf("windows = %+v", q.Windows)
	}
	w := q.Windows[0]
	if w.Type != "5h" || !w.ResetAt.Equal(time.Unix(1791950768, 718998000)) || w.UsedPct != 0 || w.LeftPct != 100 {
		t.Fatalf("w0 = %+v", w)
	}
	if q.Windows[1].Type != "weekly" {
		t.Fatalf("w1 = %+v", q.Windows[1])
	}
	c, _ := ft.callFor("EylDcb")
	if c.SourcePath != "/" || !c.Idempotent {
		t.Fatalf("call = %+v", c)
	}
}

func TestQuotaUsedAndUnknownType(t *testing.T) {
	ft := newFake(inlineFixture("EylDcb", `[1,[[null,null,null,null,1,[100,0],40,60],[null,null,null,null,9,[200,0],null,25]]]`))
	q, err := New(ft).Quota(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if q.Windows[0].UsedPct != 40 || q.Windows[0].LeftPct != 60 {
		t.Fatalf("w0 = %+v", q.Windows[0])
	}
	if q.Windows[1].Type != "type-9" || q.Windows[1].UsedPct != 75 {
		t.Fatalf("w1 = %+v", q.Windows[1])
	}
}

// The live capture pins the envelope; the [2,2] pair is the setting the web
// app actually sent in that capture.
func TestAccountSettingLive(t *testing.T) {
	fx := loadFixture(t, "hT54vc", "live_2026-10-13_01")
	if got, _ := json.Marshal(accountSettingParams([]any{nil, nil, nil, []any{[]any{2, 2}}})); fx.Synthetic || !sameJSON(t, got, fx.Request) {
		t.Fatalf("builder %s want %s", got, fx.Request)
	}
}

func TestSetLanguage(t *testing.T) {
	ft := newFake(loadFixture(t, "hT54vc", "live_2026-10-13_01"))
	if err := New(ft).SetLanguage(context.Background(), "fr"); err != nil {
		t.Fatal(err)
	}
	want := `[[[null,[[null,null,null,null,["fr"]]]]],` + mustJSON(requestOptions()) + `]`
	c, _ := ft.callFor("hT54vc")
	if p := paramsJSON(t, ft, "hT54vc"); !sameJSON(t, []byte(p), []byte(want)) || c.SourcePath != "/" || c.Idempotent {
		t.Fatalf("call %+v params %s", c, p)
	}
	if err := New(ft).SetLanguage(context.Background(), ""); err == nil {
		t.Fatal("empty language must be rejected")
	}
}
