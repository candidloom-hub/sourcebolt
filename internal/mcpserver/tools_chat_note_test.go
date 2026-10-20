package mcpserver

import (
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// chat_ask save_as_note saves the streamed answer with CYK0Xb (kind 2,
// the answer's block) and returns the server's note id and title.
func TestChatAskSaveAsNote(t *testing.T) {
	tr := newChatStub()
	tr.snaps = []wire.Snapshot{{Text: "Answer", Final: true,
		AnswerBlock: []byte(`[[[[0,6,[[[0,6,["Answer",[true]]]]]]],[]],null,null,[],1]`)}}
	tr.payloads["CYK0Xb"] = `[["note-1","Answer",[2,"o",[1792512645,0],null,null,[1792512645,0],true],null,"Server title"]]`
	cs := connectChat(t, tr, nil, nil)
	var out ChatAskOut
	res := call(t, cs, "chat_ask", map[string]any{"notebook": idA, "question": "why?", "save_as_note": true})
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil || out.Answer != "Answer" || out.NoteID != "note-1" || out.NoteTitle != "Server title" || out.NoteError != "" {
		t.Fatalf("out = %s %v", text(res), err)
	}
	var params string
	for _, c := range tr.calls {
		if c.ID == "CYK0Xb" {
			b, _ := json.Marshal(c.Params)
			params = string(b)
		}
	}
	for _, want := range []string{`"Answer",[2,null,null,null,null,null,1],[],"why?",`, `["Answer",[1,0,0,null,null,null,null,0,0]]`, `[null,1]`} {
		if !strings.Contains(params, want) {
			t.Fatalf("CYK0Xb params %s lack %s", params, want)
		}
	}
}

// Without the flag nothing is saved; an answer that cannot be saved is
// still returned, with note_error.
func TestChatAskSaveAsNoteOptional(t *testing.T) {
	saveable := newChatStub()
	saveable.snaps = []wire.Snapshot{{Text: "Answer", Final: true, AnswerBlock: []byte(`[[[],[]],null,null,[],1]`)}}
	saveable.payloads["CYK0Xb"] = `[["note-1","Answer",[2],null,"Server title"]]`
	res := call(t, connectChat(t, saveable, nil, nil), "chat_ask", map[string]any{"notebook": idA, "question": "why?"})
	if saveable.count("CYK0Xb") != 0 || strings.Contains(text(res), "note") {
		t.Fatalf("saved or tried to save a note without save_as_note: %s", text(res))
	}

	tr := newChatStub() // its answers carry no answer block
	cs := connectChat(t, tr, nil, nil)
	var out ChatAskOut
	res = call(t, cs, "chat_ask", map[string]any{"notebook": idA, "question": "why?", "save_as_note": true})
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil || out.Answer == "" || out.NoteID != "" || !strings.Contains(out.NoteError, "cannot be saved") {
		t.Fatalf("out = %s %v", text(res), err)
	}
	if tr.count("CYK0Xb") != 0 {
		t.Fatal("sent a note without an answer block")
	}
}
