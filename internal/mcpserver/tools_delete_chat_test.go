package mcpserver

import (
	"encoding/json/v2"
	"fmt"
	"strings"
	"testing"
)

// Review item 9: the chat_history preview shows each conversation's
// created and updated times and its first question when the whole history
// was read.
func TestDeleteChatHistoryPreviewDetails(t *testing.T) {
	st := newStub()
	st.payloads["hPTbtc"] = `[[["conv-1",[1700000000,0],[1700000100,0]]]]`
	// khqZz, newest turn first, no cursor: the whole history.
	st.payloads["khqZz"] = `[[["t3",[1700000090,0],2,null,[["Answer two"]]],["t2",[1700000080,0],1,"Second question?"],["t1",[1700000010,0],1,"First question?"]]]`
	cs := connect(t, st, "default")
	var out DeleteOut
	res := call(t, cs, "delete", map[string]any{"kind": "chat_history", "notebook": idA, "ids": []string{"conv-1"}})
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil || out.Status != "preview" || len(out.Items) != 1 {
		t.Fatalf("preview = %s", text(res))
	}
	it := out.Items[0]
	if it.ID != "conv-1" || it.FirstQuestion != "First question?" || it.Created.Unix() != 1700000000 || it.Updated.Unix() != 1700000100 {
		t.Fatalf("item = %+v", it)
	}
	if st.count("J7Gthc") != 0 {
		t.Fatal("preview made a destructive call")
	}
}

// Without the whole history (a cursor is left), no first question is
// claimed; times still show.
func TestDeleteChatHistoryPreviewPartialHistory(t *testing.T) {
	st := newStub()
	st.payloads["hPTbtc"] = `[[["conv-1",[1700000000,0],[1700000100,0]]]]`
	st.payloads["khqZz"] = `[[["t9",[1700000090,0],1,"A late question?"]],"more"]`
	cs := connect(t, st, "default")
	var out DeleteOut
	res := call(t, cs, "delete", map[string]any{"kind": "chat_history", "notebook": idA, "ids": []string{"conv-1"}})
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil || len(out.Items) != 1 {
		t.Fatalf("preview = %s", text(res))
	}
	if it := out.Items[0]; it.FirstQuestion != "" || it.Created.IsZero() {
		t.Fatalf("item = %+v", it)
	}
}

// Review item 10: note delete only reaches rows that are notes; a mind-map
// or studio row id from the cFji9 list is refused before any delete call.
func TestDeleteNoteRefusesNonNoteRows(t *testing.T) {
	st := newStub()
	st.payloads["cFji9"] = `[[` +
		`["n1",["n1","body",[1,"o",[1700000000,0]],null,"First"]],` +
		`["m1",["m1","{}",[2,null,null,5,[["s1"]]],null,"Mind map"]],` +
		`["a1",["a1","x",[4,"o",[1700000000,0]],null,"Studio row"]]]]`
	st.payloads["AH0mwd"] = `[]`
	cs := connect(t, st, "default")
	for _, ref := range []string{"m1", "Mind map", "a1"} {
		if e := envelope(t, call(t, cs, "delete", map[string]any{"kind": "note", "notebook": idA, "ids": []string{ref}, "confirm": true})); e.Code != "NOT_FOUND" {
			t.Fatalf("%q: envelope = %+v", ref, e)
		}
	}
	if st.count("AH0mwd") != 0 {
		t.Fatal("a non-note row reached the note delete")
	}
	if res := call(t, cs, "delete", map[string]any{"kind": "note", "notebook": idA, "ids": []string{"n1"}, "confirm": true}); res.IsError || st.count("AH0mwd") != 1 {
		t.Fatalf("note n1: %s", text(res))
	}
}

// hPTbtc lists the newest 20 conversations and paging is not pinned by a
// capture: an id that is not on a full first page gets a clear
// UNSUPPORTED_BY_TRANSPORT error, not NOT_FOUND. On a short page it is
// NOT_FOUND.
func TestDeleteChatHistoryBeyondFirstPage(t *testing.T) {
	rows := make([]string, 20)
	for i := range rows {
		rows[i] = fmt.Sprintf(`["conv-%02d",[1700000000,0],[1700000100,0]]`, i)
	}
	st := newStub()
	st.payloads["hPTbtc"] = `[[` + strings.Join(rows, ",") + `]]`
	cs := connect(t, st, "default")
	e := envelope(t, call(t, cs, "delete", map[string]any{"kind": "chat_history", "notebook": idA, "ids": []string{"conv-old"}}))
	if e.Code != "UNSUPPORTED_BY_TRANSPORT" || !strings.Contains(e.Error, "newest 20") {
		t.Fatalf("envelope = %+v", e)
	}
	st.payloads["hPTbtc"] = `[[` + rows[0] + `]]`
	if e := envelope(t, call(t, cs, "delete", map[string]any{"kind": "chat_history", "notebook": idA, "ids": []string{"conv-old"}})); e.Code != "NOT_FOUND" {
		t.Fatalf("short page: envelope = %+v", e)
	}
	if st.count("J7Gthc") != 0 {
		t.Fatal("made a delete call")
	}
}
