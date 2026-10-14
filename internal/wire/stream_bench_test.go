package wire

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"io"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// synthChunk renders one cumulative snapshot as a length-prefixed stream
// chunk: answer text of textLen bytes, plus nPass passages of ~2 KB each.
func synthChunk(i, textLen, nPass int) []byte {
	text := strings.Repeat("lorem ipsum dolor sit amet ", textLen/27+1)[:textLen]
	passages := make([]any, nPass)
	markers := make([]any, 0, nPass)
	for p := range passages {
		passages[p] = []any{[]any{nil}, []any{nil, nil, nil, nil, strings.Repeat("cited ", 340),
			[]any{[]any{[]any{fmt.Sprintf("s0000000-0000-4000-8000-%012d", p)}}}, nil}}
		markers = append(markers, []any{[]any{nil, p * 10, p*10 + 5}, []any{p, p + 1}})
	}
	tb := []any{[]any{[]any{}}, nil, nil, passages, 1}
	row := []any{text, nil, []any{"conv", "resp", 1}, nil, tb}
	inner, _ := json.Marshal([]any{row, nil, markers, nil, i == 10, nil, "x"})
	frame, _ := json.Marshal([]any{[]any{"wrb.fr", nil, string(inner)}})
	var b bytes.Buffer
	b.WriteString(strconv.Itoa(len(frame)))
	b.WriteByte('\n')
	b.Write(frame)
	b.WriteByte('\n')
	return b.Bytes()
}

// synthStream is the benchmark body: 11 cumulative snapshots, ~1.7 MB total.
func synthStream() []byte {
	var b bytes.Buffer
	b.WriteString(")]}'\n\n")
	for i := 0; i < 11; i++ {
		b.Write(synthChunk(i, 3000*(i+1)*2+40000+i*8000, 20))
	}
	b.WriteString("38\n[[\"di\",12],[\"af.httprm\",12,\"-1\",3]]\n")
	return b.Bytes()
}

func BenchmarkStreamDecode(b *testing.B) {
	body := synthStream()
	b.Logf("stream size %d bytes", len(body))
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d := NewStreamDecoder(bytes.NewReader(body))
		n := 0
		for {
			_, err := d.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatal(err)
			}
			n++
		}
		if n != 11 {
			b.Fatalf("snapshots = %d", n)
		}
	}
}

// genReader replays one pre-rendered chunk n times so the test body never
// exists in memory in full and the generator adds no garbage of its own.
type genReader struct {
	n, i   int
	size   int
	chunk  []byte // one pre-rendered snapshot, replayed n times
	cur    []byte
	prefix bool
}

func (g *genReader) Read(p []byte) (int, error) {
	for len(g.cur) == 0 {
		switch {
		case !g.prefix:
			g.cur, g.prefix = []byte(")]}'\n\n"), true
		case g.i < g.n:
			g.cur = g.chunk
			g.i++
		default:
			return 0, io.EOF
		}
	}
	k := copy(p, g.cur)
	g.cur = g.cur[k:]
	return k, nil
}

// Memory must track one snapshot, not the stream: 20 snapshots of ~2 MB
// (40 MB of body) may not push the heap anywhere near the body size.
func TestStreamMemoryProportionalToSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("memory probe")
	}
	const snap, n = 2 << 20, 20
	defer debug.SetGCPercent(debug.SetGCPercent(20))
	gen := &genReader{n: n, size: snap, chunk: synthChunk(0, snap, 4)}
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)

	var mu sync.Mutex
	var peak uint64
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		var ms runtime.MemStats
		for {
			select {
			case <-stop:
				return
			case <-time.After(2 * time.Millisecond):
			}
			runtime.ReadMemStats(&ms)
			mu.Lock()
			if ms.HeapAlloc > peak {
				peak = ms.HeapAlloc
			}
			mu.Unlock()
		}
	}()

	d := NewStreamDecoder(gen)
	got := 0
	for {
		s, err := d.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(s.Text) != snap {
			t.Fatalf("text len %d", len(s.Text))
		}
		got++
	}
	close(stop)
	<-done
	if got != n {
		t.Fatalf("snapshots = %d", got)
	}
	mu.Lock()
	growth := int64(peak) - int64(base.HeapAlloc)
	mu.Unlock()
	t.Logf("body ~%d MB, snapshot %d MB, sampled peak heap growth %d MB", n*snap>>20, snap>>20, growth>>20)
	if growth > 12*snap {
		t.Errorf("heap growth %d MB is not proportional to one snapshot (%d MB)", growth>>20, snap>>20)
	}
}
