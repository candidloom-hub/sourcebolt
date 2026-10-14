package nlm

import (
	"context"
	"errors"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

func TestResearchStatusDone(t *testing.T) {
	for s, want := range map[ResearchStatus]bool{0: false, 1: false, 2: true, 3: false, 4: false, 5: false, 6: true} {
		if s.Done() != want {
			t.Errorf("status %d Done = %v", s, s.Done())
		}
	}
}

func TestStartFastResearch(t *testing.T) {
	for name, corpus := range map[string]string{"fast_web": "web", "fast_drive": "drive"} {
		fx := loadFixture(t, "Ljjv0c", name)
		ft := newFake(fx)
		q := "go library for mcp servers"
		if corpus == "drive" {
			q = "quarterly plan"
		}
		j, err := New(ft).StartResearch(context.Background(), nb1, q, "fast", corpus)
		if err != nil {
			t.Fatal(err)
		}
		if p := paramsJSON(t, ft, "Ljjv0c"); !sameJSON(t, []byte(p), fx.Request) {
			t.Fatalf("%s params %s want %s", name, p, fx.Request)
		}
		if j.ID == "" || j.Mode != "fast" || j.Query != q {
			t.Fatalf("job = %+v", j)
		}
		c, _ := ft.callFor("Ljjv0c")
		if c.Idempotent || c.SourcePath != "/notebook/"+nb1 {
			t.Fatalf("call = %+v", c)
		}
	}
}

func TestStartFastDefaultsToWeb(t *testing.T) {
	ft := newFake(loadFixture(t, "Ljjv0c", "fast_web"))
	if _, err := New(ft).StartResearch(context.Background(), nb1, "go library for mcp servers", "fast", ""); err != nil {
		t.Fatal(err)
	}
	if p := paramsJSON(t, ft, "Ljjv0c"); p != `[["go library for mcp servers",1],null,1,"`+nb1+`"]` {
		t.Fatalf("params %s", p)
	}
}

// QA9ei: the meaning of the two answer elements is conflicting; both come back raw.
func TestStartDeepResearchReturnsBothIDs(t *testing.T) {
	fx := loadFixture(t, "QA9ei", "deep_web")
	ft := newFake(fx)
	j, err := New(ft).StartResearch(context.Background(), nb1, "history of the go module proxy", "deep", "web")
	if err != nil {
		t.Fatal(err)
	}
	if p := paramsJSON(t, ft, "QA9ei"); !sameJSON(t, []byte(p), fx.Request) {
		t.Fatalf("params %s want %s", p, fx.Request)
	}
	if j.ID != "id-a" || j.AltID != "id-b" || j.Mode != "deep" {
		t.Fatalf("job = %+v", j)
	}
}

func TestStartResearchRejections(t *testing.T) {
	cases := []struct {
		name, q, mode, corpus string
		kind                  transport.Kind
	}{
		{"discover", "some query here", "discover", "web", transport.KindUnsupported},
		{"deep drive", "some query here", "deep", "drive", transport.KindInvalidArgument},
		{"empty query", "  ", "fast", "web", transport.KindInvalidArgument},
		{"bad mode", "some query here", "turbo", "web", transport.KindInvalidArgument},
		{"bad corpus", "some query here", "fast", "ftp", transport.KindInvalidArgument},
	}
	for _, c := range cases {
		ft := newFake()
		_, err := New(ft).StartResearch(context.Background(), nb1, c.q, c.mode, c.corpus)
		var te *transport.Error
		if !errors.As(err, &te) || te.Kind != c.kind {
			t.Errorf("%s: err = %v", c.name, err)
		}
		if len(ft.calls) != 0 {
			t.Errorf("%s: %d upstream calls, want 0", c.name, len(ft.calls))
		}
	}
}

func TestPollResearchLive(t *testing.T) {
	live := loadFixture(t, "e3bVqc", "live_2026-10-13_01")
	if live.Synthetic {
		t.Fatal("must be live")
	}
	ft := newFake(live)
	jobs, err := New(ft).PollResearch(context.Background(), fakeNB)
	if err != nil || len(jobs) != 0 {
		t.Fatalf("jobs = %v err = %v", jobs, err)
	}
	if p := paramsJSON(t, ft, "e3bVqc"); !sameJSON(t, []byte(p), live.Request) {
		t.Fatalf("poll params %s want %s", p, live.Request)
	}
	c, _ := ft.callFor("e3bVqc")
	if !c.Idempotent || c.SourcePath != "/notebook/"+fakeNB {
		t.Fatalf("call = %+v", c)
	}
}

func TestPollResearchRows(t *testing.T) {
	const rows = `[["task-1",[null,["go mcp",1],1,[[["https://a.example/","A","desc-a"],["https://b.example/","B","desc-b"]],"summary-1"],2],[1,0],[1,0],"acct"],` +
		`["task-2",[null,["deep topic",1],5,[[[null,"Deep report","md"]],"report-md"],1,[1]],[1,0],[1,0],"acct"]]`
	for name, payload := range map[string]string{"flat": rows, "wrapped": "[" + rows + "]"} {
		jobs, err := New(newFake(inlineFixture("e3bVqc", payload))).PollResearch(context.Background(), nb1)
		if err != nil || len(jobs) != 2 {
			t.Fatalf("%s: jobs = %+v err = %v", name, jobs, err)
		}
		a, b := jobs[0], jobs[1]
		if a.ID != "task-1" || a.Mode != "fast" || a.Query != "go mcp" || !a.Status.Done() || len(a.Candidates) != 2 ||
			a.Candidates[1].URL != "https://b.example/" || a.Candidates[1].Title != "B" || a.Candidates[0].Description != "desc-a" || a.Report != "summary-1" {
			t.Fatalf("%s: a = %+v", name, a)
		}
		if b.ID != "task-2" || b.Mode != "deep" || b.ModeCode != 5 || b.Status.Done() || b.Status != 1 ||
			len(b.Candidates) != 1 || b.Candidates[0].Kind != "report" || b.Report != "report-md" {
			t.Fatalf("%s: b = %+v", name, b)
		}
	}
	// status 6 is also done (ruling)
	jobs, err := New(newFake(inlineFixture("e3bVqc", `[["t",[null,["q",1],1,[[],""],6]]]`))).PollResearch(context.Background(), nb1)
	if err != nil || len(jobs) != 1 || !jobs[0].Status.Done() || jobs[0].Status != 6 {
		t.Fatalf("status 6: %+v %v", jobs, err)
	}
}

func TestCancelResearch(t *testing.T) {
	fx := loadFixture(t, "Zbrupe", "cancel")
	ft := newFake(fx)
	if err := New(ft).CancelResearch(context.Background(), "task-1"); err != nil {
		t.Fatal(err)
	}
	if p := paramsJSON(t, ft, "Zbrupe"); !sameJSON(t, []byte(p), fx.Request) {
		t.Fatalf("params %s want %s", p, fx.Request)
	}
	if err := New(newFake()).CancelResearch(context.Background(), ""); err == nil {
		t.Fatal("empty run id accepted")
	}
}

const nbWithA = `[["Title",[[["s1"],"Existing A",[null,null,null,null,null,null,null,["https://example.com/a/"]],[null,2]]],"` + nb1 + `"]]`

func mixedPicks() []Candidate {
	return []Candidate{
		{Kind: "web", URL: "https://example.com/a", Title: "Example A"},
		{Kind: "drive", DriveID: "drive-doc-1", MIME: "application/vnd.google-apps.document", Title: "Plan doc"},
		{Kind: "report", Title: "Deep report", Report: "# Findings\ntext-1"},
	}
}

func TestImportResearchEntryShapes(t *testing.T) {
	fx := loadFixture(t, "LBwxtb", "import_mixed")
	ft := newFake(fx, inlineFixture("rLM1Ne", `[["Title",[],"`+nb1+`"]]`))
	out, err := New(ft).ImportResearch(context.Background(), nb1, "task-1", mixedPicks())
	if err != nil {
		t.Fatal(err)
	}
	if p := paramsJSON(t, ft, "LBwxtb"); !sameJSON(t, []byte(p), fx.Request) {
		t.Fatalf("params\n got  %s\n want %s", p, fx.Request)
	}
	want := []string{"src-a", "src-b", "src-c"}
	for i, o := range out {
		if o.Index != i || o.Status != AddAdded || o.SourceID != want[i] {
			t.Fatalf("out[%d] = %+v", i, o)
		}
	}
	c, _ := ft.callFor("LBwxtb")
	if c.Idempotent || c.SourcePath != "/notebook/"+nb1 {
		t.Fatalf("call = %+v", c)
	}
}

// A retry after a partial import must not send URLs the notebook already has.
func TestImportResearchSkipsPresent(t *testing.T) {
	fx := loadFixture(t, "LBwxtb", "import_mixed")
	ft := newFake(fx, inlineFixture("rLM1Ne", nbWithA))
	out, err := New(ft).ImportResearch(context.Background(), nb1, "task-1", mixedPicks())
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Status != AddSkipped || out[0].Index != 0 {
		t.Fatalf("out[0] = %+v", out[0])
	}
	// the web entry is gone from the request: two entries remain (drive, report)
	p := paramsJSON(t, ft, "LBwxtb")
	if want := `[null,[1],"task-1","` + nb1 + `",[` + mustJSON(driveEntry("drive-doc-1", "application/vnd.google-apps.document", "Plan doc")) + `,` + mustJSON(reportEntry("Deep report", "# Findings\ntext-1")) + `]]`; p != want {
		t.Fatalf("params\n got  %s\n want %s", p, want)
	}
	// response rows pair with the sent entries by title, in order
	if out[1].Status != AddAdded || out[2].Status != AddAdded {
		t.Fatalf("out = %+v", out)
	}
}

func TestImportResearchAllPresentSendsNothing(t *testing.T) {
	ft := newFake(inlineFixture("rLM1Ne", nbWithA))
	out, err := New(ft).ImportResearch(context.Background(), nb1, "task-1", []Candidate{{Kind: "web", URL: "https://example.com/a"}})
	if err != nil || len(out) != 1 || out[0].Status != AddSkipped {
		t.Fatalf("out = %+v err = %v", out, err)
	}
	if _, sent := ft.callFor("LBwxtb"); sent {
		t.Fatal("LBwxtb sent although everything is present")
	}
}

// LBwxtb is never retried: a failure after send surfaces once.
func TestImportResearchNeverRetried(t *testing.T) {
	ft := newFake(inlineFixture("rLM1Ne", `[["Title",[],"`+nb1+`"]]`))
	boom := &transport.Error{Kind: transport.KindTransport, RPC: "research.import", Message: "reset"}
	ft.errs["LBwxtb"] = boom
	_, err := New(ft).ImportResearch(context.Background(), nb1, "task-1", mixedPicks())
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	n := 0
	for _, c := range ft.calls {
		if c.ID == "LBwxtb" {
			n++
			if c.Idempotent {
				t.Fatal("LBwxtb marked idempotent")
			}
		}
	}
	if n != 1 {
		t.Fatalf("LBwxtb sent %d times", n)
	}
}

func TestImportResearchRejectsBadPicks(t *testing.T) {
	bad := [][]Candidate{
		nil,
		{{Kind: "web"}},
		{{Kind: "drive", DriveID: "x"}},
		{{Kind: "report", Title: "t"}},
		{{Kind: "pdf", URL: "u"}},
	}
	for _, picks := range bad {
		ft := newFake()
		if _, err := New(ft).ImportResearch(context.Background(), nb1, "task-1", picks); err == nil {
			t.Errorf("picks %+v accepted", picks)
		}
		if len(ft.calls) != 0 {
			t.Errorf("picks %+v caused %d upstream calls", picks, len(ft.calls))
		}
	}
	if _, err := New(newFake()).ImportResearch(context.Background(), nb1, "", mixedPicks()); err == nil {
		t.Error("empty task id accepted")
	}
}
