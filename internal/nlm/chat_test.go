package nlm

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/transport/rpc"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

const (
	liveNB   = "00000000-0000-4000-8000-000000000001"
	liveConv = "00000000-0000-4000-8000-000000000002"
	liveSrc  = "00000000-0000-4000-8000-000000000010"
)

// The web RPC transport is the production Streamer.
var _ Streamer = (*rpc.Transport)(nil)

// fakeStreamer is a fakeTransport that also streams: it replays snapshots
// and records each Stream call's params.
type fakeStreamer struct {
	*fakeTransport
	mu     sync.Mutex
	snaps  []wire.Snapshot
	err    error
	nbs    []string
	params []any
	// onSnap runs between snapshots (tests advance a fake clock here).
	between func(i int)
}

func (f *fakeStreamer) Stream(ctx context.Context, nb string, params any, onSnap func(wire.Snapshot)) (wire.Snapshot, error) {
	f.mu.Lock()
	f.nbs, f.params = append(f.nbs, nb), append(f.params, params)
	f.mu.Unlock()
	if f.err != nil {
		return wire.Snapshot{}, f.err
	}
	var last wire.Snapshot
	for i, s := range f.snaps {
		if f.between != nil {
			f.between(i)
		}
		if onSnap != nil {
			onSnap(s)
		}
		if !s.Thinking {
			last = s
		}
	}
	return last, nil
}

// liveSnaps decodes the live stream capture, with the stream's own id
// replaced so tests can tell it apart from the real conversation id.
func liveSnaps(t *testing.T) []wire.Snapshot {
	t.Helper()
	fx := loadFixture(t, "GenerateFreeFormStreamed", "live_2026-10-13_01")
	d := wire.NewStreamDecoder(bytes.NewReader([]byte(fx.ResponseBody)))
	var out []wire.Snapshot
	for {
		s, err := d.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		s.ConversationID = "stream-session-id"
		out = append(out, s)
	}
	return out
}

func TestAskParamsMatchLiveRequest(t *testing.T) {
	fx := loadFixture(t, "GenerateFreeFormStreamed", "live_2026-10-13_01")
	got, err := json.Marshal(askParams([]string{liveSrc}, "text-141", liveConv, liveNB, 1))
	if err != nil {
		t.Fatal(err)
	}
	want := jsontext.Value(append([]byte(nil), fx.Request...))
	if err := want.Compact(); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("params\n got %s\nwant %s", got, want)
	}
}

func TestAskDefaultConversation(t *testing.T) {
	fs := &fakeStreamer{fakeTransport: newFake(loadFixture(t, "hPTbtc", "live_2026-10-13_01")), snaps: liveSnaps(t)}
	a, err := New(fs).Ask(context.Background(), liveNB, "text-141", AskOptions{SourceIDs: []string{liveSrc}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs.params) != 1 || fs.nbs[0] != liveNB {
		t.Fatalf("stream calls = %v", fs.nbs)
	}
	p, _ := json.Marshal(fs.params[0])
	fx := loadFixture(t, "GenerateFreeFormStreamed", "live_2026-10-13_01")
	if !sameJSON(t, p, fx.Request) {
		t.Fatalf("params %s", p)
	}
	if c, ok := fs.callFor("hPTbtc"); !ok || c.SourcePath != "/notebook/"+liveNB {
		t.Fatalf("hPTbtc call = %+v %v", c, ok)
	}
	if a.ConversationID != liveConv {
		t.Fatalf("conversation = %q (must come from hPTbtc, never the stream session id)", a.ConversationID)
	}
	snaps := liveSnaps(t)
	last := snaps[len(snaps)-1]
	if a.Text != last.Text || a.ResponseID != last.ResponseID || len(a.Citations) != len(last.Citations) || len(a.Citations) == 0 {
		t.Fatalf("answer = %+v", a)
	}
}

func TestAskExplicitConversationSkipsLookup(t *testing.T) {
	fs := &fakeStreamer{fakeTransport: newFake(), snaps: liveSnaps(t)}
	a, err := New(fs).Ask(context.Background(), liveNB, "q", AskOptions{SourceIDs: []string{"s1", "s2"}, ConversationID: "conv-x"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs.calls) != 0 {
		t.Fatalf("unexpected RPCs: %+v", fs.calls)
	}
	p, _ := json.Marshal(fs.params[0])
	want := `[[[["s1"]],[["s2"]]],"q",null,[2,null,[1],[1,null,null,null,null,null,null,null,null,null,[1,3]]],"conv-x",null,null,"` + liveNB + `",1]`
	if string(p) != want {
		t.Fatalf("params %s", p)
	}
	if a.ConversationID != "conv-x" {
		t.Fatalf("conversation = %q", a.ConversationID)
	}
}

func TestAskEmptySourcesUsesNotebook(t *testing.T) {
	fs := &fakeStreamer{
		fakeTransport: newFake(loadFixture(t, "rLM1Ne", "live_2026-10-13_03"), loadFixture(t, "hPTbtc", "live_2026-10-13_01")),
		snaps:         liveSnaps(t),
	}
	if _, err := New(fs).Ask(context.Background(), liveNB, "text-141", AskOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := fs.callFor("rLM1Ne"); !ok {
		t.Fatal("source ids not read from the notebook")
	}
	p, _ := json.Marshal(fs.params[0])
	fx := loadFixture(t, "GenerateFreeFormStreamed", "live_2026-10-13_01")
	if !sameJSON(t, p, fx.Request) {
		t.Fatalf("params %s", p)
	}
}

func TestAskNoSourcesInNotebook(t *testing.T) {
	fs := &fakeStreamer{fakeTransport: newFake(loadFixture(t, "rLM1Ne", "live_2026-10-13_01")), snaps: liveSnaps(t)}
	_, err := New(fs).Ask(context.Background(), liveNB, "q", AskOptions{}, nil)
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindInvalidArgument || len(fs.params) != 0 {
		t.Fatalf("err = %v, streams = %d", err, len(fs.params))
	}
}

func TestAskNewConversation(t *testing.T) {
	fs := &fakeStreamer{fakeTransport: newFake(loadFixture(t, "hPTbtc", "live_2026-10-13_01")), snaps: liveSnaps(t)}
	a, err := New(fs).Ask(context.Background(), liveNB, "q", AskOptions{SourceIDs: []string{"s1"}, NewConversation: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := json.Marshal(fs.params[0])
	var arr []any
	json.Unmarshal(p, &arr)
	if arr[4] != nil {
		t.Fatalf("new conversation must send a null id, got %v", arr[4])
	}
	if a.ConversationID != liveConv {
		t.Fatalf("conversation = %q (read back through hPTbtc after the ask)", a.ConversationID)
	}
}

func TestAskProgressThrottled(t *testing.T) {
	snaps := []wire.Snapshot{
		{Text: "thinking", Thinking: true},
		{Text: "a"}, {Text: "ab"}, {Text: "abc"}, {Text: "abcd"}, {Text: "abcde", Final: true},
	}
	now := time.Unix(1000, 0)
	// Clock per snapshot: 0, 0, 0.3, 0.6, 1.2, 1.5 s.
	steps := []time.Duration{0, 0, 300 * time.Millisecond, 600 * time.Millisecond, 1200 * time.Millisecond, 1500 * time.Millisecond}
	fs := &fakeStreamer{fakeTransport: newFake(), snaps: snaps}
	var cur time.Time
	fs.between = func(i int) { cur = now.Add(steps[i]) }
	c := New(fs)
	old := askNow
	askNow = func() time.Time { return cur }
	t.Cleanup(func() { askNow = old })
	var got []string
	a, err := c.Ask(context.Background(), liveNB, "q", AskOptions{SourceIDs: []string{"s"}, ConversationID: "c"}, func(s string) { got = append(got, s) })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "abcd" {
		t.Fatalf("progress = %q, want [a abcd]", got)
	}
	if a.Text != "abcde" {
		t.Fatalf("answer = %q", a.Text)
	}
}

func TestAskWithoutStreamer(t *testing.T) {
	_, err := New(newFake()).Ask(context.Background(), liveNB, "q", AskOptions{SourceIDs: []string{"s"}, ConversationID: "c"}, nil)
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindUnsupported {
		t.Fatalf("err = %v", err)
	}
}

func TestAskStreamErrorPassesThrough(t *testing.T) {
	quota := &transport.Error{Kind: transport.KindQuota, RPC: "chat.ask", Message: "limit"}
	fs := &fakeStreamer{fakeTransport: newFake(), err: quota}
	_, err := New(fs).Ask(context.Background(), liveNB, "q", AskOptions{SourceIDs: []string{"s"}, ConversationID: "c"}, nil)
	if !errors.Is(err, quota) {
		t.Fatalf("err = %v", err)
	}
}

func TestAskEmptyQuestion(t *testing.T) {
	fs := &fakeStreamer{fakeTransport: newFake()}
	_, err := New(fs).Ask(context.Background(), liveNB, "  ", AskOptions{SourceIDs: []string{"s"}}, nil)
	var te *transport.Error
	if !errors.As(err, &te) || te.Kind != transport.KindInvalidArgument {
		t.Fatalf("err = %v", err)
	}
}
