package mcpserver

import (
	"encoding/json/v2"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/jobs"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func waitJob(t *testing.T, cs *mcp.ClientSession, id string) ChatStatusOut {
	t.Helper()
	for range 200 {
		var st ChatStatusOut
		if err := json.Unmarshal([]byte(text(call(t, cs, "chat_status", map[string]any{"job_id": id}))), &st); err != nil {
			t.Fatal(err)
		}
		if st.Status != jobs.StatusRunning {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return ChatStatusOut{}
}

func TestChatAskDetachedAndStatus(t *testing.T) {
	cs := connectChat(t, newChatStub(), jobs.New(), nil)
	res := call(t, cs, "chat_ask", map[string]any{"notebook": idA, "question": "q", "wait": false})
	var start ChatAskOut
	if err := json.Unmarshal([]byte(text(res)), &start); err != nil || start.JobID == "" {
		t.Fatalf("start = %q %v", text(res), err)
	}
	st := waitJob(t, cs, start.JobID)
	if st.Status != jobs.StatusDone || st.Kind != "chat_ask" {
		t.Fatalf("status = %+v", st)
	}
	r, _ := st.Result.(map[string]any)
	if r["answer"] != "Partial answer [1]" {
		t.Fatalf("result = %#v", st.Result)
	}
	if e := envelope(t, call(t, cs, "chat_status", map[string]any{"job_id": "nope"})); e.Code != "NOT_FOUND" {
		t.Fatalf("unknown job: %+v", e)
	}
}

func TestChatAskDetachedCap(t *testing.T) {
	tr := newChatStub()
	tr.block = true
	jt := jobs.New()
	t.Cleanup(jt.Close)
	cs := connectChat(t, tr, jt, nil)
	ask := map[string]any{"notebook": idA, "question": "q", "wait": false}
	for i := range maxDetachedAsks {
		res := call(t, cs, "chat_ask", ask)
		if res.IsError {
			t.Fatalf("ask %d: %s", i, text(res))
		}
	}
	e := envelope(t, call(t, cs, "chat_ask", ask))
	if e.Code != "RATE_LIMITED" || e.Error == "" {
		t.Fatalf("ask over the cap: %+v", e)
	}
}

func TestJobTableCloseStopsDetachedAsks(t *testing.T) {
	tr := newChatStub()
	tr.block = true
	jt := jobs.New()
	cs := connectChat(t, tr, jt, nil)
	res := call(t, cs, "chat_ask", map[string]any{"notebook": idA, "question": "q", "wait": false, "conversation_id": "conv-1"})
	var start ChatAskOut
	_ = json.Unmarshal([]byte(text(res)), &start)
	if j, _ := jt.Get(start.JobID); j.Meta["conversation_id"] != "conv-1" {
		t.Fatalf("job meta = %v", j.Meta)
	}
	jt.Close()
	if st := waitJob(t, cs, start.JobID); st.Status != jobs.StatusCancelled {
		t.Fatalf("status after Close = %+v", st)
	}
}

func TestChatAskDetachedFailure(t *testing.T) {
	tr := newChatStub()
	tr.stub.payloads["rLM1Ne"] = `[["x",[],"` + idA + `"]]` // no sources: Ask rejects
	cs := connectChat(t, tr, jobs.New(), nil)
	res := call(t, cs, "chat_ask", map[string]any{"notebook": idA, "question": "q", "wait": false})
	var start ChatAskOut
	_ = json.Unmarshal([]byte(text(res)), &start)
	st := waitJob(t, cs, start.JobID)
	if st.Status != jobs.StatusFailed || st.Error == "" {
		t.Fatalf("status = %+v", st)
	}
}

func TestJobCancelStopsAsk(t *testing.T) {
	tr := newChatStub()
	tr.block = true
	cs := connectChat(t, tr, jobs.New(), nil)
	res := call(t, cs, "chat_ask", map[string]any{"notebook": idA, "question": "q", "wait": false, "conversation_id": "conv-9"})
	var start ChatAskOut
	_ = json.Unmarshal([]byte(text(res)), &start)

	res = call(t, cs, "job_cancel", map[string]any{"job_id": start.JobID})
	var c JobCancelOut
	if err := json.Unmarshal([]byte(text(res)), &c); err != nil || !c.Cancelled || c.Status != jobs.StatusCancelled {
		t.Fatalf("cancel = %q %v", text(res), err)
	}
	if tr.count("XgrPMd") != 1 {
		t.Fatalf("CancelChat calls = %d", tr.count("XgrPMd"))
	}
	if st := waitJob(t, cs, start.JobID); st.Status != jobs.StatusCancelled {
		t.Fatalf("status = %+v", st)
	}
	// A second cancel is a no-op, not an error.
	res = call(t, cs, "job_cancel", map[string]any{"job_id": start.JobID})
	if res.IsError || tr.count("XgrPMd") != 1 {
		t.Fatalf("second cancel: %s", text(res))
	}
	if e := envelope(t, call(t, cs, "job_cancel", map[string]any{"job_id": "nope"})); e.Code != "NOT_FOUND" {
		t.Fatalf("unknown job: %+v", e)
	}
}
