package nlm

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

func TestListArtifactsParamsMatchLive(t *testing.T) {
	live := loadFixture(t, "gArtLc", "live_2026-10-13_01")
	got, _ := json.Marshal(listArtifactsParams(fakeNB))
	var gs, ls []jsonRaw
	if json.Unmarshal(got, &gs) != nil || json.Unmarshal(live.Request, &ls) != nil || len(gs) != 3 || len(ls) != 3 {
		t.Fatalf("shapes %s vs %s", got, live.Request)
	}
	// The live filter string was sanitized to "text-43"; options and notebook id are real.
	for i := range 2 {
		if !sameJSON(t, gs[i], ls[i]) {
			t.Fatalf("param %d = %s want %s", i, gs[i], ls[i])
		}
	}
	if string(gs[2]) != `"NOT artifact.status = \"ARTIFACT_STATUS_SUGGESTED\""` {
		t.Fatalf("filter = %s", gs[2])
	}
	// The live notebook had no artifacts: payload "[]".
	as, err := New(newFake(live)).ListArtifacts(context.Background(), fakeNB)
	if err != nil || len(as) != 0 {
		t.Fatalf("live empty = %v %v", as, err)
	}
}

type jsonRaw = jsontext.Value

func TestListArtifacts(t *testing.T) {
	f := loadFixture(t, "gArtLc", "list_mixed")
	ft := newFake(f)
	as, err := New(ft).ListArtifacts(context.Background(), nb1)
	if err != nil {
		t.Fatal(err)
	}
	if p := paramsJSON(t, ft, "gArtLc"); !sameJSON(t, []byte(p), f.Request) {
		t.Fatalf("params %s", p)
	}
	c, _ := ft.callFor("gArtLc")
	if !c.Idempotent || c.SourcePath != "/notebook/"+nb1 {
		t.Fatalf("call = %+v", c)
	}
	// art-4 is suggested (status 5) and dropped client-side too.
	if len(as) != 4 {
		t.Fatalf("artifacts = %+v", as)
	}
	a := as[0]
	if a.ID != "art-1" || a.Title != "Deep dive" || a.Kind != StudioAudio || a.Status != ArtifactReady || a.Created.Unix() != 1760000000 {
		t.Fatalf("art-1 = %+v", a)
	}
	if len(a.MediaURLs) != 0 {
		t.Fatalf("media urls must stay empty until live capture: %+v", a.MediaURLs)
	}
	if as[1].Kind != StudioReport || as[1].Status != ArtifactFailed {
		t.Fatalf("art-2 = %+v", as[1])
	}
	if as[2].Kind != StudioInteractive || as[2].Status != ArtifactPending || !as[2].Created.IsZero() {
		t.Fatalf("art-3 = %+v", as[2])
	}
	if as[3].Kind != StudioUnknown || as[3].Status != ArtifactPending {
		t.Fatalf("art-5 = %+v", as[3])
	}
}

func TestKindFromTypeCode(t *testing.T) {
	for code, want := range map[int64]StudioKind{
		1: StudioAudio, 2: StudioReport, 3: StudioVideo, 4: StudioInteractive,
		7: StudioInfographic, 8: StudioSlides, 9: StudioDataTable, 11: StudioInteractiveReport, 0: StudioUnknown, 10: StudioUnknown,
	} {
		if got := KindFromTypeCode(code); got != want {
			t.Errorf("code %d = %q want %q", code, got, want)
		}
	}
}

func TestGetArtifact(t *testing.T) {
	f := loadFixture(t, "v9rmvd", "interactive")
	ft := newFake(f)
	ac, err := New(ft).GetArtifact(context.Background(), "art-3")
	if err != nil {
		t.Fatal(err)
	}
	if p := paramsJSON(t, ft, "v9rmvd"); !sameJSON(t, []byte(p), f.Request) {
		t.Fatalf("params %s", p)
	}
	if ac.HTML != "<html>quiz</html>" || !sameJSON(t, ac.Tree, []byte(`[["n1","root"]]`)) {
		t.Fatalf("content = %+v", ac)
	}
	if _, err := New(newFake(inlineFixture("v9rmvd", `[[null]]`))).GetArtifact(context.Background(), "x"); err == nil {
		t.Fatal("empty content must be an error")
	}
	if _, err := New(newFake()).GetArtifact(context.Background(), ""); err == nil {
		t.Fatal("empty id must be rejected")
	}
}

func quotaFixture(left string) Fixture {
	return inlineFixture("EylDcb", `[null,[[null,null,null,null,1,[1760000000,0],null,`+left+`]]]`)
}

func TestCanGenerate(t *testing.T) {
	for _, tc := range []struct {
		left string
		want bool
	}{{"40", true}, {"0", false}} {
		ok, q, err := New(newFake(quotaFixture(tc.left))).CanGenerate(context.Background())
		if err != nil || ok != tc.want || len(q.Windows) != 1 {
			t.Fatalf("left %s: ok=%v q=%+v err=%v", tc.left, ok, q, err)
		}
	}
}

func TestGenerateBlocked(t *testing.T) {
	ft := newFake()
	_, err := New(ft).Generate(context.Background(), nb1, StudioRequest{Kind: StudioInteractiveReport, Instructions: "t"})
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindUnsupported || !strings.Contains(te.Message, "no live capture") {
		t.Fatalf("err = %v", err)
	}
	if len(ft.calls) != 0 {
		t.Fatal("stub must not call the network")
	}
}
