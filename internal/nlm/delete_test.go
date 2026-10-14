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
	if err := New(ft).Delete(context.Background(), "notebook", "", []string{nb1}); err != nil {
		t.Fatal(err)
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
	if err := New(ft).Delete(context.Background(), "notebook", "", []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if len(ft.calls) != 2 {
		t.Fatalf("calls = %d", len(ft.calls))
	}
}

func TestDeleteBlockedKinds(t *testing.T) {
	for kind, rpc := range map[string]string{"source": "tGMBJ", "artifact": "V5N4be", "chat_turn": "J7Gthc", "note": "AH0mwd"} {
		ft := newFake()
		err := New(ft).Delete(context.Background(), kind, nb1, []string{"x"})
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
		err := New(newFake()).Delete(context.Background(), tc.kind, nb1, tc.ids)
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
