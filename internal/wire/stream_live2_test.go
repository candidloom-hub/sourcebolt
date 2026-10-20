package wire

import (
	"bytes"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"testing"
)

// TestStreamLive2 decodes the chat stream of live capture #2 (2026-10-20):
// a follow-up in an existing conversation. Its one marker,
// [[null,18,175],[0]], cites a single passage: the marker's second element
// is a list of passage indices, not a [from, to) range.
func TestStreamLive2(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "GenerateFreeFormStreamed", "live_2026-10-20_01.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Body string `json:"response_body"`
	}
	if err := json.Unmarshal(b, &fx); err != nil {
		t.Fatal(err)
	}
	snaps, err := drain(NewStreamDecoder(bytes.NewReader([]byte(fx.Body))))
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 12 {
		t.Fatalf("snapshots = %d", len(snaps))
	}
	last := LastAnswer(snaps)
	if last.Text != "text-65" || !last.Final || last.Thinking {
		t.Fatalf("last = %+v", last)
	}
	// The stream carried the conversation id the request named.
	if last.ConversationID != "00000000-0000-4000-8000-000000000009" || last.ResponseID != "00000000-0000-4000-8000-000000000010" {
		t.Fatalf("ids = %q %q", last.ConversationID, last.ResponseID)
	}
	want := Citation{N: 1, SourceID: "00000000-0000-4000-8000-000000000012", Start: 18, End: 175}
	if len(last.Citations) != 1 {
		t.Fatalf("citations = %+v", last.Citations)
	}
	c := last.Citations[0]
	c.Text = ""
	if c != want {
		t.Fatalf("citation = %+v, want %+v", last.Citations[0], want)
	}
}
