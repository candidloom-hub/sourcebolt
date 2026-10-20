package wire

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func liveBody(t *testing.T, name string) string {
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
	return fx.Body
}

// liveStream decodes a live stream fixture with the answer block kept.
func liveStream(t *testing.T, name string) []Snapshot {
	t.Helper()
	d := NewStreamDecoder(bytes.NewReader([]byte(liveBody(t, name))))
	d.KeepAnswerBlock()
	snaps, err := drain(d)
	if err != nil {
		t.Fatal(err)
	}
	return snaps
}

// The answer block is opt-in: a plain decoder (every chat_ask without
// save_as_note) never copies it, which costs one allocation less.
func TestAnswerBlockOptIn(t *testing.T) {
	body := []byte(liveBody(t, "live3_2026-10-20_01"))
	plain, err := drain(NewStreamDecoder(bytes.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range plain {
		if s.AnswerBlock != nil {
			t.Fatal("a plain decoder kept the answer block")
		}
	}
	if raceEnabled {
		return // the race detector's allocation noise (MBs) hides an 18 KB copy
	}
	// Malloc counts drift by a few dozen between runs, so compare bytes:
	// the least allocated over several runs, with and without the copy.
	// The difference must be most of the block's size.
	block := len(LastAnswer(liveStream(t, "live3_2026-10-20_01")).AnswerBlock)
	run := func(keep bool) uint64 {
		least := uint64(math.MaxUint64)
		for range 8 {
			var a, b runtime.MemStats
			runtime.ReadMemStats(&a)
			d := NewStreamDecoder(bytes.NewReader(body))
			if keep {
				d.KeepAnswerBlock()
			}
			_, _ = drain(d)
			runtime.ReadMemStats(&b)
			least = min(least, b.TotalAlloc-a.TotalAlloc)
		}
		return least
	}
	p, k := run(false), run(true)
	if k < p || k-p < uint64(block)/2 {
		t.Fatalf("bytes allocated: plain %d, keep %d, block %d: the plain decoder should skip the block copy", p, k, block)
	}
}

// The request to keep the block travels in the context.
func TestAnswerBlockContext(t *testing.T) {
	if AnswerBlockWanted(context.Background()) || !AnswerBlockWanted(WithAnswerBlock(context.Background())) {
		t.Fatal("context flag")
	}
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
	fx := struct{ Body string }{liveBody(t, "live3_2026-10-20_01")}
	want := LastAnswer(liveStream(t, "live3_2026-10-20_01")).AnswerBlock
	inner, _ := json.Marshal([]any{[]any{string(bytes.Repeat([]byte("x"), 4*len(want)))}})
	frame, _ := json.Marshal([]any{[]any{"wrb.fr", nil, string(inner)}})
	body := fx.Body + "\n" + string(frame) + "\n"
	d := NewStreamDecoder(bytes.NewReader([]byte(body)))
	d.KeepAnswerBlock()
	snaps, err := drain(d)
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
