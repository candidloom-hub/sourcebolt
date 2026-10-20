package wire

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"testing"
)

func liveStream(t *testing.T, name string) []Snapshot {
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
	snaps, err := drain(NewStreamDecoder(bytes.NewReader([]byte(fx.Body))))
	if err != nil {
		t.Fatal(err)
	}
	return snaps
}

// The final answer snapshot keeps its raw type block
// [rich, null, null, passages, 1]: saving the answer as a note (CYK0Xb)
// is built from it. Only that one snapshot carries a copy, so a stream
// costs one copy, not one per snapshot.
func TestAnswerBlockOnFinalSnapshotOnly(t *testing.T) {
	for _, name := range []string{"live3_2026-10-20_01", "live_2026-10-20_01"} {
		snaps := liveStream(t, name)
		finals := 0
		for _, s := range snaps {
			switch {
			case s.Final && !s.Thinking:
				finals++
				var tb []jsontext.Value
				if err := json.Unmarshal(s.AnswerBlock, &tb); err != nil || len(tb) != 5 || string(tb[4]) != "1" || string(tb[1]) != "null" {
					t.Fatalf("%s: answer block %.200s (%v)", name, s.AnswerBlock, err)
				}
			case s.AnswerBlock != nil:
				t.Fatalf("%s: non-final snapshot carries an answer block", name)
			}
		}
		if finals != 1 {
			t.Fatalf("%s: %d final snapshots", name, finals)
		}
	}
}

// The block must not alias the decoder's reused buffers: a frame decoded
// after the final snapshot must leave it intact.
func TestAnswerBlockIsACopy(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "GenerateFreeFormStreamed", "live3_2026-10-20_01.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Body string `json:"response_body"`
	}
	if err := json.Unmarshal(b, &fx); err != nil {
		t.Fatal(err)
	}
	want := LastAnswer(liveStream(t, "live3_2026-10-20_01")).AnswerBlock
	inner, _ := json.Marshal([]any{[]any{string(bytes.Repeat([]byte("x"), 4*len(want)))}})
	frame, _ := json.Marshal([]any{[]any{"wrb.fr", nil, string(inner)}})
	body := fx.Body + "\n" + string(frame) + "\n"
	snaps, err := drain(NewStreamDecoder(bytes.NewReader([]byte(body))))
	if err != nil {
		t.Fatal(err)
	}
	var got []byte
	for _, s := range snaps {
		if s.AnswerBlock != nil {
			got = s.AnswerBlock
		}
	}
	if !bytes.Equal(got, want) || !jsontext.Value(got).IsValid() {
		t.Fatalf("answer block changed by a later frame: %.200s", got)
	}
}
