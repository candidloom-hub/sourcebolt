package wire

import (
	"bytes"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"testing"
)

// Review item 3: an index that is negative, fractional or not a number is
// skipped, never read as passage 0.
func TestCitationsSkipBadIndices(t *testing.T) {
	marker := []byte(`[[null,1,2],[-1,"x",null,1.5,true,[0],2]]`)
	got := citations([][]byte{marker}, nil)
	if len(got) != 1 || got[0].N != 3 {
		t.Fatalf("citations = %+v, want only N=3", got)
	}
}

// A hostile marker cannot expand into more than maxCitationRange citations.
func TestCitationsIndexCap(t *testing.T) {
	idx := make([]int, 3000)
	for i := range idx {
		idx[i] = i
	}
	list, _ := json.Marshal(idx)
	marker := []byte(`[[null,1,2],` + string(list) + `]`)
	got := citations([][]byte{marker}, nil)
	if len(got) != maxCitationRange || got[len(got)-1].N != maxCitationRange {
		t.Fatalf("citations = %d", len(got))
	}
}

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
