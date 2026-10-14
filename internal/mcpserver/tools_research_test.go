package mcpserver

import (
	"testing"
	"time"
)

const pollRows = `[["task-1",[null,["go mcp servers",1],1,[[["https://a.example/","A","desc-a"],["https://b.example/","B","desc-b"]],"summary"],2],[1,0],[1,0],"acct"]]`

const pollRunning = `[["task-1",[null,["go mcp servers",1],1,[[],""],1],[1,0],[1,0],"acct"]]`

func TestResearchStartHappyAndShortQuery(t *testing.T) {
	st := newStub()
	st.payloads["Ljjv0c"] = `["task-1"]`
	cs := connect(t, st, "default")

	res := call(t, cs, "research_start", map[string]any{"notebook": idA, "query": "go"})
	if e := envelope(t, res); e.Code != "INVALID_ARGUMENT" {
		t.Fatalf("envelope = %+v", e)
	}
	if st.count("Ljjv0c") != 0 {
		t.Fatal("short query reached the transport")
	}
	out := decode[ResearchStartOut](t, call(t, cs, "research_start", map[string]any{"notebook": idA, "query": "go", "force": true}))
	if out.Job.ID != "task-1" || out.Job.Mode != "fast" {
		t.Fatalf("out = %+v", out)
	}
	out = decode[ResearchStartOut](t, call(t, cs, "research_start", map[string]any{"notebook": idA, "query": "go mcp server libraries"}))
	if out.Job.ID != "task-1" {
		t.Fatalf("out = %+v", out)
	}
}

func TestResearchStartDiscoverUnsupported(t *testing.T) {
	cs := connect(t, newStub(), "default")
	res := call(t, cs, "research_start", map[string]any{"notebook": idA, "query": "a long enough query here", "mode": "discover"})
	if e := envelope(t, res); e.Code != "UNSUPPORTED_BY_TRANSPORT" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestResearchStatusCandidatesWithIndices(t *testing.T) {
	st := newStub()
	st.payloads["e3bVqc"] = pollRows
	cs := connect(t, st, "default")
	out := decode[ResearchStatusOut](t, call(t, cs, "research_status", map[string]any{"notebook": idA}))
	if !out.Done || len(out.Jobs) != 1 || len(out.Jobs[0].Candidates) != 2 || out.Jobs[0].Candidates[1].Index != 1 {
		t.Fatalf("out = %+v", out)
	}
	if out.Jobs[0].Report != "" {
		t.Fatal("report included without include_report")
	}
}

func TestResearchStatusWaitPollsUntilDone(t *testing.T) {
	old := researchPollEvery
	researchPollEvery = 5 * time.Millisecond
	t.Cleanup(func() { researchPollEvery = old })

	st := newStub()
	st.payloads["e3bVqc"] = pollRunning
	cs := connect(t, st, "default")
	go func() {
		time.Sleep(30 * time.Millisecond)
		st.mu.Lock()
		st.payloads["e3bVqc"] = pollRows
		st.mu.Unlock()
	}()
	out := decode[ResearchStatusOut](t, call(t, cs, "research_status", map[string]any{"notebook": idA, "job": "task-1", "wait_seconds": 10}))
	if !out.Done {
		t.Fatalf("out = %+v", out)
	}
	if n := st.count("e3bVqc"); n < 2 {
		t.Fatalf("polls = %d, want >= 2", n)
	}
}

func TestResearchStatusWaitBounds(t *testing.T) {
	cs := connect(t, newStub(), "default")
	res := call(t, cs, "research_status", map[string]any{"notebook": idA, "wait_seconds": 121})
	if e := envelope(t, res); e.Code != "INVALID_ARGUMENT" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestResearchImportSelect(t *testing.T) {
	st := newStub()
	st.payloads["e3bVqc"] = pollRows
	st.payloads["LBwxtb"] = `[[[["src-b"],"B"]]]`
	cs := connect(t, st, "default")
	out := decode[ResearchImportOut](t, call(t, cs, "research_import", map[string]any{"notebook": idA, "job": "task-1", "select": []int{1}}))
	if out.Added != 1 || out.Results[0].Index != 1 || out.Results[0].SourceID != "src-b" {
		t.Fatalf("out = %+v", out)
	}
}

func TestResearchImportErrors(t *testing.T) {
	st := newStub()
	st.payloads["e3bVqc"] = pollRows
	cs := connect(t, st, "default")
	for _, args := range []map[string]any{
		{"notebook": idA, "job": "task-1"},
		{"notebook": idA, "job": "task-1", "select": []int{9}},
		{"notebook": idA, "job": "missing", "all": true},
	} {
		if e := envelope(t, call(t, cs, "research_import", args)); e.Code != "INVALID_ARGUMENT" {
			t.Fatalf("args %v: envelope = %+v", args, e)
		}
	}
	if st.count("LBwxtb") != 0 {
		t.Fatal("import reached the transport")
	}
}
