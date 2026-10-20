package nlm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

func TestDeleteNotebook(t *testing.T) {
	fx := loadFixture(t, "WWINqb", "delete")
	ft := newFake(fx)
	res, err := New(ft).Delete(context.Background(), "notebook", "", []string{nb1})
	if err != nil || len(res) != 1 || res[0].ID != nb1 || res[0].Status != "deleted" {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if p := paramsJSON(t, ft, "WWINqb"); !sameJSON(t, []byte(p), fx.Request) {
		t.Fatalf("params %s", p)
	}
	if c, _ := ft.callFor("WWINqb"); c.Idempotent {
		t.Fatalf("delete must not be retried: %+v", c)
	}
}

func TestDeleteNotebookOneCallPerID(t *testing.T) {
	ft := newFake(loadFixture(t, "WWINqb", "delete"))
	res, err := New(ft).Delete(context.Background(), "notebook", "", []string{"a", "b", "a"})
	if err != nil || len(res) != 2 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if len(ft.calls) != 2 {
		t.Fatalf("calls = %d, want 2 (duplicates dropped)", len(ft.calls))
	}
}

func TestDeleteNotebookContinuesAfterFailure(t *testing.T) {
	ft := newFake()
	ft.errs["WWINqb"] = &transport.Error{Kind: transport.KindPermission, RPC: "notebooks.delete"}
	res, err := New(ft).Delete(context.Background(), "notebook", "", []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ft.calls) != 2 || len(res) != 2 {
		t.Fatalf("calls = %d, res = %+v; every id must be tried", len(ft.calls), res)
	}
	for _, r := range res {
		if r.Status != "failed" || r.Err == nil {
			t.Fatalf("res = %+v", res)
		}
	}
	ft.errs["WWINqb"] = &transport.Error{Kind: transport.KindUnsupported, RPC: "notebooks.delete"}
	res, _ = New(ft).Delete(context.Background(), "notebook", "", []string{"a"})
	if len(res) != 1 || res[0].Status != "unsupported" {
		t.Fatalf("res = %+v", res)
	}
}

// TestDeleteLive2 pins the three deletes done in the UI during live capture
// #2 (2026-10-20): tGMBJ deletes a source, AH0mwd a note and J7Gthc a
// conversation's chat history.
func TestDeleteLive2(t *testing.T) {
	for _, tc := range []struct {
		kind, rpc, fixture, nb, id string
	}{
		{"source", "tGMBJ", "live_2026-10-20_01", fakeNB2, "00000000-0000-4000-8000-000000000006"},
		{"note", "AH0mwd", "live_2026-10-20_01", fakeNB2, "00000000-0000-4000-8000-000000000016"},
		{"note", "AH0mwd", "live_2026-10-20_02", "00000000-0000-4000-8000-000000000011", "00000000-0000-4000-8000-000000000015"},
		{"chat_history", "J7Gthc", "live_2026-10-20_01", fakeNB2, "00000000-0000-4000-8000-000000000017"},
	} {
		fx := loadFixture(t, tc.rpc, tc.fixture)
		ft := newFake(fx)
		res, err := New(ft).Delete(context.Background(), tc.kind, tc.nb, []string{tc.id})
		if err != nil || len(res) != 1 || res[0].ID != tc.id || res[0].Status != "deleted" {
			t.Fatalf("%s/%s: res = %+v, err = %v", tc.rpc, tc.fixture, res, err)
		}
		if p := paramsJSON(t, ft, tc.rpc); !sameJSON(t, []byte(p), fx.Request) {
			t.Fatalf("%s/%s params\n got  %s\n want %s", tc.rpc, tc.fixture, p, fx.Request)
		}
		c, _ := ft.callFor(tc.rpc)
		if c.Idempotent || c.SourcePath != "/notebook/"+tc.nb {
			t.Fatalf("%s: call = %+v", tc.rpc, c)
		}
	}
}

func TestDeleteNotebookScopedKindsNeedNotebook(t *testing.T) {
	for _, kind := range []string{"source", "note", "chat_history"} {
		ft := newFake()
		_, err := New(ft).Delete(context.Background(), kind, "", []string{"x"})
		var te *transport.Error
		if !errors.As(err, &te) || te.Kind != transport.KindInvalidArgument || len(ft.calls) != 0 {
			t.Fatalf("%s: err = %v, calls = %d", kind, err, len(ft.calls))
		}
	}
}

func TestDeleteBlockedKinds(t *testing.T) {
	for kind, rpc := range map[string]string{"artifact": "V5N4be"} {
		ft := newFake()
		_, err := New(ft).Delete(context.Background(), kind, nb1, []string{"x"})
		var te *transport.Error
		if !errors.As(err, &te) || te.Kind != transport.KindUnsupported || !strings.Contains(te.Message, rpc) || !strings.Contains(te.Message, "live capture") {
			t.Fatalf("%s: err = %v", kind, err)
		}
		if len(ft.calls) != 0 {
			t.Fatalf("%s: blocked kind must not call", kind)
		}
	}
}

func TestDeleteInvalid(t *testing.T) {
	for _, tc := range []struct {
		kind string
		ids  []string
	}{{"notebook", nil}, {"notebook", []string{""}}, {"bogus", []string{"x"}}} {
		_, err := New(newFake()).Delete(context.Background(), tc.kind, nb1, tc.ids)
		var te *transport.Error
		if !errors.As(err, &te) || te.Kind != transport.KindInvalidArgument {
			t.Fatalf("%+v: err = %v", tc, err)
		}
	}
}

func TestCancelChat(t *testing.T) {
	fx := loadFixture(t, "XgrPMd", "cancel")
	ft := newFake(fx)
	if err := New(ft).CancelChat(context.Background(), "conv-0001"); err != nil {
		t.Fatal(err)
	}
	if p := paramsJSON(t, ft, "XgrPMd"); !sameJSON(t, []byte(p), fx.Request) {
		t.Fatalf("params %s", p)
	}
	var te *transport.Error
	if err := New(newFake()).CancelChat(context.Background(), ""); !errors.As(err, &te) || te.Kind != transport.KindInvalidArgument {
		t.Fatalf("err = %v", err)
	}
}
