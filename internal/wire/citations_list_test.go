package wire

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// lastPayload returns the last answer payload of a live stream fixture.
func lastPayload(t *testing.T, name string) []any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "GenerateFreeFormStreamed", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Body string `json:"response_body"`
	}
	if err := json.Unmarshal(b, &fx); err != nil {
		t.Fatal(err)
	}
	var last []any
	for _, line := range bytes.Split([]byte(fx.Body), []byte("\n")) {
		var frames [][]any
		if json.Unmarshal(line, &frames) != nil {
			continue
		}
		for _, f := range frames {
			if len(f) > 2 && f[0] == "wrb.fr" {
				if s, ok := f[2].(string); ok && s != "" {
					var p []any
					if json.Unmarshal([]byte(s), &p) == nil && len(p) > 0 {
						last = p
					}
				}
			}
		}
	}
	if last == nil {
		t.Fatalf("%s: no payload", name)
	}
	return last
}

// TestCitationMarkersAreIndexLists checks the list reading against both live
// captures. The answer row's type block carries its own citation list,
// row[4][0][1] = [[passage_id],[null,start,end]].., and its passages at
// row[4][3] = [[passage_id],..]. For every marker [[null,start,end],[i..]]
// the pairs (passage i's id, start) must be exactly that list:
//
//	#1: markers [0,1]@73 and [1,2]@184, list = p0@73 p1@73 p1@184 p2@184
//	    (a [from,to) range would give only p0@73 and p1@184);
//	#2: marker [0]@18, list = p0@18 (a range needs two numbers: none).
func TestCitationMarkersAreIndexLists(t *testing.T) {
	for _, name := range []string{"live_2026-10-13_01", "live_2026-10-20_01"} {
		p := lastPayload(t, name)
		tb := p[0].([]any)[4].([]any)
		var want []string
		for _, c := range tb[0].([]any)[1].([]any) {
			c := c.([]any)
			want = append(want, fmt.Sprint(c[0].([]any)[0], "@", c[1].([]any)[1]))
		}
		passages := tb[3].([]any)
		raw, _ := json.Marshal(p)
		var got []string
		for _, c := range LastAnswer(mustDrain(t, raw)).Citations {
			got = append(got, fmt.Sprint(passages[c.N-1].([]any)[0].([]any)[0], "@", c.Start))
		}
		slices.Sort(want)
		slices.Sort(got)
		if len(want) == 0 || !slices.Equal(got, want) {
			t.Fatalf("%s: decoded %v, type block lists %v", name, got, want)
		}
	}
}

// mustDrain decodes one payload as a single-frame stream.
func mustDrain(t *testing.T, payload []byte) []Snapshot {
	t.Helper()
	q, _ := json.Marshal(string(payload))
	frame := `[["wrb.fr",null,` + string(q) + `]]`
	body := fmt.Sprintf(")]}'\n\n%d\n%s\n", len(frame), frame)
	snaps, err := drain(NewStreamDecoder(bytes.NewReader([]byte(body))))
	if err != nil || len(snaps) == 0 {
		t.Fatalf("decode: %v", err)
	}
	return snaps
}

// Review item 4: the same N at another offset is a separate citation; only
// exact duplicates (same N, same offsets) are dropped.
func TestCitationsDropExactDuplicates(t *testing.T) {
	markers := [][]byte{
		[]byte(`[[null,6,6],[1,1]]`),
		[]byte(`[[null,6,6],[1]]`),
		[]byte(`[[null,9,9],[1]]`),
	}
	got := citations(markers, nil)
	want := []Citation{{N: 2, Start: 6, End: 6}, {N: 2, Start: 9, End: 9}}
	if !slices.Equal(got, want) {
		t.Fatalf("citations = %+v, want %+v", got, want)
	}
}

// The original synthetic fixtures encode markers as [from, to) ranges (made
// before any live capture). They are kept unchanged; under the live
// index-list reading their markers name the listed indices.
func TestStreamRangeFormSyntheticFixtures(t *testing.T) {
	s := LastAnswer(mustDrainBody(t, streamFixture(t, "emoji_utf16.txt")))
	if len(s.Citations) != 2 || s.Citations[0].N != 1 || s.Citations[1].N != 2 || s.Citations[1].SourceID != "" {
		t.Fatalf("emoji_utf16 citations = %+v", s.Citations)
	}
	s = LastAnswer(mustDrainBody(t, streamFixture(t, "citations_rewrite.txt")))
	if len(s.Citations) != 2 || s.Citations[0].N != 2 || s.Citations[1].N != 6 {
		t.Fatalf("citations_rewrite citations = %+v", s.Citations)
	}
}

func mustDrainBody(t *testing.T, body []byte) []Snapshot {
	t.Helper()
	snaps, err := drain(NewStreamDecoder(bytes.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	return snaps
}
