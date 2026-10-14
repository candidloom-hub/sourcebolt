package wire

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func streamFixture(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "_stream", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func liveStreamBody(t testing.TB) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "GenerateFreeFormStreamed", "live_2026-10-13_01.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Body string `json:"response_body"`
	}
	if err := json.Unmarshal(b, &fx); err != nil {
		t.Fatal(err)
	}
	return []byte(fx.Body)
}

// drain reads every snapshot until the decoder stops; err is nil on io.EOF.
func drain(d *StreamDecoder) ([]Snapshot, error) {
	var out []Snapshot
	for {
		s, err := d.Next()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, s)
	}
}

func TestStreamLiveFixtureStructure(t *testing.T) {
	d := NewStreamDecoder(bytes.NewReader(liveStreamBody(t)))
	snaps, err := drain(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 9 {
		t.Fatalf("snapshots = %d, want 10", len(snaps))
	}
	if snaps[0].Text != "" || snaps[1].Text != "" {
		t.Errorf("first two snapshots should carry empty text: %q %q", snaps[0].Text, snaps[1].Text)
	}
	for i, s := range snaps {
		if s.Thinking {
			t.Errorf("snap %d: unexpected thinking flag", i)
		}
		if s.ConversationID != "00000000-0000-4000-8000-000000000002" || s.ResponseID != "00000000-0000-4000-8000-000000000012" {
			t.Errorf("snap %d ids = %q %q", i, s.ConversationID, s.ResponseID)
		}
		if s.Final != (i == len(snaps)-1) {
			t.Errorf("snap %d Final = %v", i, s.Final)
		}
	}
	// Citations appear from the fourth snapshot and only grow.
	if n := len(snaps[2].Citations); n != 0 {
		t.Errorf("snap 2 citations = %d, want 0", n)
	}
	last := snaps[len(snaps)-1]
	if last.Text != "text-136" {
		t.Errorf("last text = %q", last.Text)
	}
	if len(last.Citations) != 2 {
		t.Fatalf("last citations = %+v", last.Citations)
	}
	c := last.Citations[1]
	if c.SourceID != "00000000-0000-4000-8000-000000000010" {
		t.Errorf("live citation source id = %q (passage[1][5][0][0][0])", c.SourceID)
	}
	if c.N != 2 || c.Start != 184 || c.End != 184 {
		t.Errorf("citation 2 = %+v", c)
	}
	if got := d.Trailers().FSID; got != "-000000000000000021" {
		t.Errorf("FSID = %q", got)
	}
}

func TestStreamThinkingThenAnswer(t *testing.T) {
	snaps, err := drain(NewStreamDecoder(bytes.NewReader(streamFixture(t, "thinking_then_answer.txt"))))
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 5 {
		t.Fatalf("snapshots = %d", len(snaps))
	}
	for i, want := range []bool{true, true, false, false, false} {
		if snaps[i].Thinking != want {
			t.Errorf("snap %d Thinking = %v", i, snaps[i].Thinking)
		}
	}
	last := snaps[4]
	if last.Text != "Hello world." || !last.Final || last.Thinking {
		t.Errorf("last = %+v", last)
	}
	if got := LastAnswer(snaps); got.Text != "Hello world." {
		t.Errorf("LastAnswer = %q", got.Text)
	}
}

func TestStreamCitationsLastSnapshotWins(t *testing.T) {
	snaps, err := drain(NewStreamDecoder(bytes.NewReader(streamFixture(t, "citations_rewrite.txt"))))
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("snapshots = %d", len(snaps))
	}
	if n := len(snaps[0].Citations); n != 2 {
		t.Fatalf("first snapshot citations = %d", n)
	}
	last := LastAnswer(snaps)
	if last.Text != "Fact [2-5]." {
		t.Fatalf("text = %q", last.Text)
	}
	want := []Citation{
		{N: 2, SourceID: "s0000000-0000-4000-8000-000000000022", Text: "beta", Start: 6, End: 6},
		{N: 3, SourceID: "s0000000-0000-4000-8000-000000000011", Text: "gamma", Start: 6, End: 6},
		{N: 4, SourceID: "s0000000-0000-4000-8000-000000000022", Text: "delta", Start: 6, End: 6},
		{N: 5, SourceID: "s0000000-0000-4000-8000-000000000011", Text: "eps", Start: 6, End: 6},
	}
	if len(last.Citations) != len(want) {
		t.Fatalf("citations = %+v", last.Citations)
	}
	for i := range want {
		if last.Citations[i] != want[i] {
			t.Errorf("citation %d = %+v, want %+v", i, last.Citations[i], want[i])
		}
	}
}

func TestStreamEmojiUTF16Offsets(t *testing.T) {
	snaps, err := drain(NewStreamDecoder(bytes.NewReader(streamFixture(t, "emoji_utf16.txt"))))
	if err != nil {
		t.Fatal(err)
	}
	s := LastAnswer(snaps)
	if len(s.Citations) != 1 {
		t.Fatalf("citations = %+v", s.Citations)
	}
	c := s.Citations[0]
	if c.Start != 7 || c.End != 11 {
		t.Fatalf("offsets = %d..%d, want 7..11 (UTF-16 units)", c.Start, c.End)
	}
	if got := SliceUTF16(s.Text, c.Start, c.End); got != "fact" {
		t.Errorf("SliceUTF16 = %q", got)
	}
	if got := SliceUTF16(s.Text, 0, 2); got != "\U0001F600" {
		t.Errorf("emoji slice = %q", got)
	}
	// Out-of-range and mid-surrogate requests clamp instead of panicking.
	if got := SliceUTF16(s.Text, 5, 9999); got == "" {
		t.Error("clamped slice empty")
	}
	_ = SliceUTF16(s.Text, 1, 3)
	_ = SliceUTF16(s.Text, -4, 2)
	_ = SliceUTF16(s.Text, 9, 3)
}

func TestStreamHeartbeatsSkipped(t *testing.T) {
	snaps, err := drain(NewStreamDecoder(bytes.NewReader(streamFixture(t, "heartbeat.txt"))))
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 || snaps[0].Text != "one" || snaps[1].Text != "one two" || !snaps[1].Final {
		t.Fatalf("snaps = %+v", snaps)
	}
}

func TestStreamQuotaRejection(t *testing.T) {
	snaps, err := drain(NewStreamDecoder(bytes.NewReader(streamFixture(t, "quota_rejection.txt"))))
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 {
		t.Fatalf("snaps = %+v", snaps)
	}
	s := snaps[0]
	if s.Status == nil || s.Status.Code != 8 || len(s.Status.Details) != 1 || !s.Final {
		t.Fatalf("snapshot = %+v", s)
	}
	if !strings.Contains(string(s.Status.Details[0]), "UserDisplayableError") {
		t.Errorf("detail = %s", s.Status.Details[0])
	}
}

func TestStreamTruncated(t *testing.T) {
	d := NewStreamDecoder(bytes.NewReader(streamFixture(t, "truncated.txt")))
	snaps, err := drain(d)
	if len(snaps) != 2 || snaps[1].Text != "complete one two" {
		t.Fatalf("prior snapshots lost: %+v", snaps)
	}
	var we *Error
	if !errors.As(err, &we) || !we.Malformed {
		t.Fatalf("err = %v, want malformed *Error", err)
	}
	// The error is sticky; the decoder does not resume.
	if _, err2 := d.Next(); err2 == nil || errors.Is(err2, io.EOF) {
		t.Errorf("second Next err = %v", err2)
	}
}

func TestStreamServerErrorFrame(t *testing.T) {
	body := ")]}'\n\n20\n[[\"er\",null,277566]]\n"
	_, err := drain(NewStreamDecoder(strings.NewReader(body)))
	var we *Error
	if !errors.As(err, &we) || !we.ServerErr || we.Code != 277566 {
		t.Fatalf("err = %v", err)
	}
}

func TestStreamNonStreamBodies(t *testing.T) {
	_, err := drain(NewStreamDecoder(strings.NewReader("<html><body>sign in</body></html>")))
	var we *Error
	if !errors.As(err, &we) || !we.SignedOut {
		t.Errorf("html err = %v", err)
	}
	_, err = drain(NewStreamDecoder(strings.NewReader(")]}'\n16")))
	if !errors.As(err, &we) || we.Code != 16 {
		t.Errorf("numeric err = %v", err)
	}
	snaps, err := drain(NewStreamDecoder(strings.NewReader("")))
	if len(snaps) != 0 || err == nil {
		t.Errorf("empty body: snaps=%d err=%v", len(snaps), err)
	}
}

// A reader that returns one byte per call proves segments are assembled across
// arbitrary read boundaries, including mid-string and mid-escape.
func TestStreamOneByteReads(t *testing.T) {
	body := streamFixture(t, "emoji_utf16.txt")
	snaps, err := drain(NewStreamDecoder(oneByteReader{bytes.NewReader(body)}))
	if err != nil || len(snaps) != 1 || snaps[0].Citations[0].Start != 7 {
		t.Fatalf("snaps=%+v err=%v", snaps, err)
	}
}

type oneByteReader struct{ r io.Reader }

func (o oneByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return o.r.Read(p[:1])
}

// Snapshots must be delivered as segments arrive, not after the whole body.
func TestStreamIncremental(t *testing.T) {
	body := streamFixture(t, "thinking_then_answer.txt")
	pr, pw := io.Pipe()
	d := NewStreamDecoder(pr)
	go func() {
		// Write only up to the end of the first snapshot's chunk, then block.
		i := bytes.Index(body, []byte("\n[[\"wrb.fr\"")) // first chunk json starts after its length line
		j := bytes.Index(body[i+1:], []byte("\n")) + i + 2
		_, _ = pw.Write(body[:j+1])
	}()
	s, err := d.Next()
	if err != nil || !s.Thinking {
		t.Fatalf("first Next = %+v, %v", s, err)
	}
	pw.Close()
}
