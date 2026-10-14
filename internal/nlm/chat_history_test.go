package nlm

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

const fakeConv = "00000000-0000-4000-8000-000000000002"

// synthRow builds an in-test synthetic fixture whose single wrb.fr frame
// carries payload (shape: catalog consensus, not live-verified).
func synthRow(t *testing.T, rpc string, payload any) Fixture {
	t.Helper()
	inner, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	row, err := json.Marshal([]any{[]any{"wrb.fr", rpc, string(inner), nil, nil, nil, "generic"}})
	if err != nil {
		t.Fatal(err)
	}
	return Fixture{RPC: rpc, Synthetic: true, ResponseBody: fmt.Sprintf(")]}'\n\n%d\n%s\n", len(row)+1, row)}
}

// TestLiveChatShapes pins hPTbtc and khqZz builders to the live capture.
func TestLiveChatShapes(t *testing.T) {
	cases := []struct {
		rpc    string
		params any
	}{
		{"hPTbtc", chatSessionsParams(fakeNB, 20)},
		{"khqZz", chatTurnsParams(fakeConv, 20, "")},
	}
	for _, c := range cases {
		fx := loadFixture(t, c.rpc, "live_2026-10-13_01")
		if fx.Synthetic {
			t.Fatalf("%s must be a live capture", c.rpc)
		}
		got, _ := json.Marshal(c.params)
		if !sameJSON(t, got, fx.Request) {
			t.Errorf("%s builder\n got  %s\n want %s", c.rpc, got, fx.Request)
		}
	}
}

func TestChatSessionsLive(t *testing.T) {
	ft := newFake(loadFixture(t, "hPTbtc", "live_2026-10-13_01"))
	got, err := New(ft).ChatSessions(t.Context(), fakeNB)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != fakeConv || got[0].Created.IsZero() || got[0].Updated.Before(got[0].Created) {
		t.Fatalf("sessions = %+v", got)
	}
	c, _ := ft.callFor("hPTbtc")
	if c.RPC != "chat.sessions" || c.SourcePath != "/notebook/"+fakeNB || !c.Idempotent {
		t.Fatalf("call = %+v", c)
	}
}

func TestHistoryEmptyLive(t *testing.T) {
	ft := newFake(loadFixture(t, "khqZz", "live_2026-10-13_01"))
	turns, cur, err := New(ft).History(t.Context(), fakeNB, fakeConv, 20, "")
	if err != nil || len(turns) != 0 || cur != "" {
		t.Fatalf("turns=%v cur=%q err=%v", turns, cur, err)
	}
}

// Synthetic: three turns served newest first over two pages.
func TestHistoryPaging(t *testing.T) {
	p1 := newFake(loadFixture(t, "khqZz", "synthetic_page1"))
	turns, cur, err := New(p1).History(t.Context(), fakeNB, fakeConv, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if cur != "cursor-2" || len(turns) != 2 {
		t.Fatalf("page1 cur=%q turns=%+v", cur, turns)
	}
	// oldest first within the page
	if turns[0].Role != "assistant" || turns[0].Text != "text-a1" || turns[1].Role != "user" || turns[1].Text != "text-q2" {
		t.Fatalf("page1 order = %+v", turns)
	}
	if !turns[0].Created.Equal(time.Unix(1760000200, 0)) {
		t.Fatalf("created = %v", turns[0].Created)
	}
	c, _ := p1.callFor("khqZz")
	got, _ := json.Marshal(c.Params)
	want := loadFixture(t, "khqZz", "synthetic_page1").Request
	if !sameJSON(t, got, want) {
		t.Fatalf("page1 params %s want %s", got, want)
	}

	p2 := newFake(loadFixture(t, "khqZz", "synthetic_page2"))
	turns2, cur2, err := New(p2).History(t.Context(), fakeNB, fakeConv, 2, cur)
	if err != nil {
		t.Fatal(err)
	}
	if cur2 != "" || len(turns2) != 1 || turns2[0].Role != "user" || turns2[0].Text != "text-q1" {
		t.Fatalf("page2 cur=%q turns=%+v", cur2, turns2)
	}
	c2, _ := p2.callFor("khqZz")
	got2, _ := json.Marshal(c2.Params)
	if !sameJSON(t, got2, loadFixture(t, "khqZz", "synthetic_page2").Request) {
		t.Fatalf("page2 params %s", got2)
	}
}

func configNB(t *testing.T, settings any) Fixture {
	row := []any{"", []any{}, fakeNB, "", nil, nil, nil, settings}
	return synthRow(t, "rLM1Ne", []any{row})
}

func TestConfigureMergesPartial(t *testing.T) {
	// current: custom goal with prompt, default length. Only length changes.
	ft := newFake(configNB(t, []any{[]any{2, "text-prompt"}, []any{1}}), loadFixture(t, "s0tc2d", "chat_config"))
	if err := New(ft).Configure(t.Context(), fakeNB, ChatConfig{Length: "longer"}); err != nil {
		t.Fatal(err)
	}
	c, ok := ft.callFor("s0tc2d")
	if !ok {
		t.Fatal("no s0tc2d call")
	}
	if c.RPC != "notebooks.mutate" || c.SourcePath != "/notebook/"+fakeNB || c.Idempotent {
		t.Fatalf("call = %+v", c)
	}
	got, _ := json.Marshal(c.Params)
	if want := loadFixture(t, "s0tc2d", "chat_config").Request; !sameJSON(t, got, want) {
		t.Fatalf("params %s want %s", got, want)
	}
	if _, ok := ft.callFor("rLM1Ne"); !ok {
		t.Fatal("did not read current settings first")
	}
}

func TestConfigureFromNullSettings(t *testing.T) {
	ft := newFake(loadFixture(t, "rLM1Ne", "get_empty"), loadFixture(t, "s0tc2d", "chat_config"))
	if err := New(ft).Configure(t.Context(), fakeNB, ChatConfig{Goal: "learning_guide"}); err != nil {
		t.Fatal(err)
	}
	c, _ := ft.callFor("s0tc2d")
	got, _ := json.Marshal(c.Params)
	want := `["` + fakeNB + `",[[null,null,null,null,null,null,null,[[3],[1]]]]]`
	if !sameJSON(t, got, []byte(want)) {
		t.Fatalf("params %s want %s", got, want)
	}
}

func TestConfigureKeepsGoalWhenOnlyPromptMissing(t *testing.T) {
	ft := newFake(configNB(t, []any{[]any{3}, []any{5}}), loadFixture(t, "s0tc2d", "chat_config"))
	if err := New(ft).Configure(t.Context(), fakeNB, ChatConfig{Goal: "custom", Prompt: "p"}); err != nil {
		t.Fatal(err)
	}
	c, _ := ft.callFor("s0tc2d")
	got, _ := json.Marshal(c.Params)
	want := `["` + fakeNB + `",[[null,null,null,null,null,null,null,[[2,"p"],[5]]]]]`
	if !sameJSON(t, got, []byte(want)) {
		t.Fatalf("params %s want %s", got, want)
	}
}

func TestConfigureRejectsBadInput(t *testing.T) {
	cur := configNB(t, []any{[]any{1}, []any{1}})
	cases := map[string]ChatConfig{
		"unknown goal":      {Goal: "pirate"},
		"unknown length":    {Length: "huge"},
		"custom no prompt":  {Goal: "custom"},
		"prompt too long":   {Goal: "custom", Prompt: strings.Repeat("x", 10001)},
		"prompt non-custom": {Goal: "learning_guide", Prompt: "p"},
		"empty":             {},
	}
	for name, cfg := range cases {
		ft := newFake(cur, loadFixture(t, "s0tc2d", "chat_config"))
		err := New(ft).Configure(t.Context(), fakeNB, cfg)
		var te *transport.Error
		if !errors.As(err, &te) || te.Kind != transport.KindInvalidArgument {
			t.Errorf("%s: err = %v", name, err)
		}
		if _, wrote := ft.callFor("s0tc2d"); wrote {
			t.Errorf("%s: wrote settings despite invalid input", name)
		}
	}
}
