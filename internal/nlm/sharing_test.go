package nlm

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"strings"
	"testing"
)

// inlineFixture builds a synthetic fixture whose wrb.fr payload is payload.
func inlineFixture(rpc, payload string) Fixture {
	q, _ := json.Marshal(payload)
	row := fmt.Sprintf(`[["wrb.fr","%s",%s,null,null,null,"generic"],["di",1]]`, rpc, q)
	return Fixture{RPC: rpc, Synthetic: true, ResponseBody: ")]}'\n\n" + fmt.Sprint(len(row)+1) + "\n" + row + "\n"}
}

func paramsJSON(t *testing.T, ft *fakeTransport, id string) string {
	t.Helper()
	c, ok := ft.callFor(id)
	if !ok {
		t.Fatalf("no call for %s", id)
	}
	b, _ := json.Marshal(c.Params)
	return string(b)
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestGetShareLive(t *testing.T) {
	fx := loadFixture(t, "JFMDGd", "live_2026-10-13_01")
	if got, _ := json.Marshal(getShareParams(fakeNB)); fx.Synthetic || !sameJSON(t, got, fx.Request) {
		t.Fatalf("builder %s want %s", got, fx.Request)
	}
	ft := newFake(fx)
	s, err := New(ft).GetShare(context.Background(), fakeNB)
	if err != nil {
		t.Fatal(err)
	}
	if s.Public || len(s.Users) != 1 || s.Users[0].Email != "user@example.com" || s.Users[0].Name != "text-42" || s.Users[0].Role != "owner" {
		t.Fatalf("share = %+v", s)
	}
	c, _ := ft.callFor("JFMDGd")
	if c.SourcePath != "/notebook/"+fakeNB || !c.Idempotent {
		t.Fatalf("call = %+v", c)
	}
}

func TestGetSharePublicRoles(t *testing.T) {
	ft := newFake(inlineFixture("JFMDGd", `[[["o@x.com",1,[],["O",""]],["e@x.com",2,[],["E",""]],["v@x.com",3,[],["V",""]]],[true],1000]`))
	s, err := New(ft).GetShare(context.Background(), nb1)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Public || len(s.Users) != 3 || s.Users[1].Role != "editor" || s.Users[2].Role != "viewer" {
		t.Fatalf("share = %+v", s)
	}
}

func TestUpdateSharePublic(t *testing.T) {
	fx := loadFixture(t, "QDyure", "public_link")
	ft := newFake(fx)
	pub := true
	if err := New(ft).UpdateShare(context.Background(), nb1, ShareUpdate{Public: &pub}); err != nil {
		t.Fatal(err)
	}
	c, _ := ft.callFor("QDyure")
	if !sameJSON(t, []byte(paramsJSON(t, ft, "QDyure")), fx.Request) || c.Idempotent || c.SourcePath != "/notebook/"+nb1 {
		t.Fatalf("call %+v params %s", c, paramsJSON(t, ft, "QDyure"))
	}
	priv := false
	ft = newFake(fx)
	if err := New(ft).UpdateShare(context.Background(), nb1, ShareUpdate{Public: &priv}); err != nil {
		t.Fatal(err)
	}
	want := `[[["` + nb1 + `",null,[0],[0,""]]],1,null,[2]]`
	if p := paramsJSON(t, ft, "QDyure"); !sameJSON(t, []byte(p), []byte(want)) {
		t.Fatalf("restricted params %s", p)
	}
}

func TestUpdateShareUsers(t *testing.T) {
	fx := loadFixture(t, "QDyure", "users")
	ft := newFake(fx)
	err := New(ft).UpdateShare(context.Background(), nb1, ShareUpdate{Users: []ShareChange{
		{Email: "a@example.com", Role: "editor"}, {Email: "b@example.com", Role: "viewer"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if p := paramsJSON(t, ft, "QDyure"); !sameJSON(t, []byte(p), fx.Request) {
		t.Fatalf("params %s", p)
	}
	ft = newFake(fx)
	err = New(ft).UpdateShare(context.Background(), nb1, ShareUpdate{Users: []ShareChange{{Email: "c@x.com", Role: "remove"}}, Notify: true, Message: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	want := `[[["` + nb1 + `",[["c@x.com",null,4]],null,[1,"hi"]]],1,null,[2]]`
	if p := paramsJSON(t, ft, "QDyure"); !sameJSON(t, []byte(p), []byte(want)) {
		t.Fatalf("params %s", p)
	}
}

func TestUpdateShareValidation(t *testing.T) {
	ft := newFake()
	c := New(ft)
	bad := []ShareUpdate{
		{Users: []ShareChange{{Email: "a@x.com", Role: "owner"}}},
		{Users: []ShareChange{{Email: "", Role: "viewer"}}},
		{},
	}
	for i, u := range bad {
		if err := c.UpdateShare(context.Background(), nb1, u); err == nil {
			t.Fatalf("case %d must be rejected", i)
		}
	}
	if len(ft.calls) != 0 {
		t.Fatalf("calls = %d", len(ft.calls))
	}
}

func TestUpdateShareBothCalls(t *testing.T) {
	ft := newFake(loadFixture(t, "QDyure", "users"))
	pub := true
	err := New(ft).UpdateShare(context.Background(), nb1, ShareUpdate{Public: &pub, Users: []ShareChange{{Email: "a@x.com", Role: "viewer"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ft.calls) != 2 || !strings.Contains(mustJSON(ft.calls[0].Params), "[1]") {
		t.Fatalf("calls = %d", len(ft.calls))
	}
}
