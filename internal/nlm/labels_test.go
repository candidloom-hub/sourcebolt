package nlm

import (
	"context"
	"encoding/json/v2"
	"strings"
	"testing"
)

func TestLabels(t *testing.T) {
	live := loadFixture(t, "I3xc3c", "live_2026-10-13_01")
	if got, _ := json.Marshal(labelsParams(fakeNB)); live.Synthetic || !sameJSON(t, got, live.Request) {
		t.Fatalf("builder %s want %s", got, live.Request)
	}
	ls, err := New(newFake(live)).Labels(context.Background(), fakeNB)
	if err != nil || len(ls) != 0 {
		t.Fatalf("live empty = %v %v", ls, err)
	}
	ft := newFake(inlineFixture("I3xc3c", `[[["Plans",[["s1"],["s2"]],"lbl-1","📌"],["Misc",[],"lbl-2",null]]]`))
	ls, err = New(ft).Labels(context.Background(), nb1)
	if err != nil || len(ls) != 2 {
		t.Fatalf("labels = %+v %v", ls, err)
	}
	if ls[0].ID != "lbl-1" || ls[0].Name != "Plans" || ls[0].Emoji != "📌" || len(ls[0].SourceIDs) != 2 || ls[0].SourceIDs[1] != "s2" || ls[1].Emoji != "" {
		t.Fatalf("labels = %+v", ls)
	}
	c, _ := ft.callFor("I3xc3c")
	if !c.Idempotent || c.SourcePath != "/notebook/"+nb1 {
		t.Fatalf("call = %+v", c)
	}
}

func TestMutateLabels(t *testing.T) {
	rn, as := loadFixture(t, "le8sX", "rename"), loadFixture(t, "le8sX", "add_source")
	ft := newFake(rn)
	if err := New(ft).MutateLabels(context.Background(), nb1, []LabelMutation{{ID: "lbl-1", Name: "Plans"}}); err != nil {
		t.Fatal(err)
	}
	if p := paramsJSON(t, ft, "le8sX"); !sameJSON(t, []byte(p), rn.Request) {
		t.Fatalf("rename %s", p)
	}
	c, _ := ft.callFor("le8sX")
	if c.Idempotent || c.SourcePath != "/notebook/"+nb1 {
		t.Fatalf("call = %+v", c)
	}
	ft = newFake(as)
	if err := New(ft).MutateLabels(context.Background(), nb1, []LabelMutation{{ID: "lbl-1", AddSources: []string{"src-0001"}}}); err != nil {
		t.Fatal(err)
	}
	if p := paramsJSON(t, ft, "le8sX"); !sameJSON(t, []byte(p), as.Request) {
		t.Fatalf("add %s", p)
	}
}

// Only the first id is honored per call, so each source id gets its own call.
func TestMutateLabelsOneSourcePerCall(t *testing.T) {
	ft := newFake(loadFixture(t, "le8sX", "rename"))
	err := New(ft).MutateLabels(context.Background(), nb1, []LabelMutation{
		{ID: "lbl-1", Emoji: "📌", AddSources: []string{"a", "b"}, RemoveSources: []string{"c"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range ft.calls {
		got = append(got, mustJSON(c.Params.([]any)[3]))
	}
	want := []string{`[[[null,"📌"]]]`, `[[null,[["a"]]]]`, `[[null,[["b"]]]]`, `[[null,null,[["c"]]]]`}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v", got)
	}
}

func TestMutateLabelsRejectsBad(t *testing.T) {
	ft := newFake()
	for _, m := range []LabelMutation{{ID: "x"}, {Name: "n"}} {
		if err := New(ft).MutateLabels(context.Background(), nb1, []LabelMutation{m}); err == nil {
			t.Fatalf("%+v must be rejected", m)
		}
	}
	if len(ft.calls) != 0 {
		t.Fatal("no call expected")
	}
}
