package wire

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/candidloom-hub/sourcebolt/bench/benchdata"
)

// refDecode is Decode as it was before the allocation-free rewrite (one
// json.Unmarshal per row and column). The rewrite must return the same
// frames, trailers and errors for every input.
func refDecode(body []byte) ([]Frame, Trailers, error) {
	var tr Trailers
	rest, hasPrefix := stripPrefix(body)
	if !hasPrefix && bytes.HasPrefix(rest, []byte("<")) {
		return nil, tr, &Error{SignedOut: true, Msg: "html page"}
	}
	if t := bytes.TrimSpace(rest); len(t) > 0 {
		if n, err := strconv.Atoi(string(t)); err == nil {
			return nil, tr, &Error{Code: n, Msg: "numeric error body"}
		}
	}
	segs, unterminated := splitTopLevel(refEscapeControl(rest))
	bad, total := 0, len(segs)
	if unterminated {
		bad++
		total++
	}
	var frames []Frame
	for _, seg := range segs {
		rows, ok := elems(seg)
		if !ok {
			bad++
			continue
		}
		for _, r := range rows {
			if f, ok := refParseRow(r, &tr); ok {
				frames = append(frames, f)
			}
		}
	}
	if bad > 0 && bad*10 > total {
		return nil, tr, &Error{Malformed: true, Msg: "unparseable chunks"}
	}
	return frames, tr, nil
}

func refParseRow(r jsontext.Value, tr *Trailers) (Frame, bool) {
	cols, ok := elems(r)
	if !ok || len(cols) == 0 {
		return Frame{}, false
	}
	var tag string
	if json.Unmarshal(cols[0], &tag) != nil {
		return Frame{}, false
	}
	switch tag {
	case "wrb.fr":
		f := Frame{ID: Str(r, 1)}
		if len(cols) > 2 {
			f.Payload = refUnquote(cols[2])
		}
		if len(cols) > 5 {
			f.Status = parseStatus(cols[5])
		}
		if len(cols) > 6 {
			f.Index = Str(r, 6)
		}
		return f, true
	case "er":
		n, _ := Int(r, 2)
		return Frame{ID: Str(r, 1), ServerErr: true, ErrCode: int(n)}, true
	case "af.httprm":
		if s := Str(r, 2); s != "" {
			tr.FSID = s
		}
	}
	return Frame{}, false
}

func refUnquote(v jsontext.Value) jsontext.Value {
	if isNull(v) {
		return nil
	}
	cur := bytes.TrimSpace(v)
	for i := 0; i < maxUnquote && len(cur) > 0 && cur[0] == '"'; i++ {
		var s string
		if json.Unmarshal(cur, &s, lenient) != nil {
			break
		}
		cur = refEscapeControl(bytes.TrimSpace([]byte(s)))
	}
	if isNull(cur) {
		return nil
	}
	return jsontext.Value(cur)
}

func refEscapeControl(b []byte) []byte {
	var out []byte
	inStr, esc := false, false
	for i, c := range b {
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			case c < 0x20:
				if out == nil {
					out = append(make([]byte, 0, len(b)+16), b[:i]...)
				}
				switch c {
				case '\n':
					out = append(out, `\n`...)
				case '\r':
					out = append(out, `\r`...)
				case '\t':
					out = append(out, `\t`...)
				default:
					out = append(out, `\u00`...)
					out = append(out, hexDigit(c>>4), hexDigit(c&0xF))
				}
				continue
			}
		} else if c == '"' {
			inStr = true
		}
		if out != nil {
			out = append(out, c)
		}
	}
	if out == nil {
		return b
	}
	return out
}

func checkDecodeMatchesRef(t *testing.T, body []byte) {
	t.Helper()
	if g, w := escapeControl(body), refEscapeControl(body); !bytes.Equal(g, w) {
		t.Fatalf("%q: escapeControl = %q, want %q", body, g, w)
	}
	gf, gt, gerr := Decode(body)
	wf, wt, werr := refDecode(body)
	if (gerr == nil) != (werr == nil) {
		t.Fatalf("%q: err = %v, want %v", body, gerr, werr)
	}
	if gerr != nil {
		var ge, we *Error
		if !errors.As(gerr, &ge) || !errors.As(werr, &we) || !reflect.DeepEqual(*ge, *we) {
			t.Fatalf("%q: err = %#v, want %#v", body, gerr, werr)
		}
		return
	}
	if gt != wt {
		t.Fatalf("%q: trailers = %+v, want %+v", body, gt, wt)
	}
	if len(gf) != len(wf) {
		t.Fatalf("%q: %d frames, want %d", body, len(gf), len(wf))
	}
	for i := range gf {
		g, w := gf[i], wf[i]
		if g.ID != w.ID || g.Index != w.Index || g.ServerErr != w.ServerErr || g.ErrCode != w.ErrCode ||
			!bytes.Equal(g.Payload, w.Payload) || (g.Payload == nil) != (w.Payload == nil) ||
			!reflect.DeepEqual(g.Status, w.Status) {
			t.Fatalf("%q: frame %d = %+v, want %+v", body, i, g, w)
		}
	}
}

func decodeSeeds() [][]byte {
	seeds := [][]byte{
		[]byte(xssi + chunk(3, `[["wrb.fr","wXbhsf","[[\"a\"]]",null,null,null,"generic"],["di",12],["af.httprm",12,"-123",5]]`)),
		[]byte(xssi + chunk(3, `[["wrb.fr","x",null,null,null,[3,"bad",[]]],["er","y",7]]`)),
		[]byte(xssi + "\n123\n"),
		[]byte("12345"),
		[]byte(" 7 "),
		[]byte("<html>"),
		[]byte(xssi + chunk(3, "[[\"wrb.fr\",\"x\",\"[\\\"a\\nb\\\"]\"]]") + "[[\"wrb.fr\",\"z\",\"\\\"[1]\\\"\"]]"),
		[]byte(xssi + "[[\"wrb.fr\",\"q\",\"[\\\"raw\nnewline\\\", \\\"\\\\u0041\\\"]\"]]"),
		[]byte(xssi + "[[\"wrb.fr\",\"u\",\"\xff\\\"\"]]"),
		[]byte(xssi + `[["wrb.fr", "s", " [1] ", 1, 2, null, "idx"], [1,2], [], "x", [[]]]`),
		[]byte(xssi + `[["wrb.fr"]][[`),
	}
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "fixtures", "_wire", "*.txt"))
	for _, p := range files {
		if b, err := os.ReadFile(p); err == nil {
			seeds = append(seeds, b)
		}
	}
	body, _ := benchdata.ListBodyOfSize(40_000)
	return append(seeds, body)
}

// Callers may reuse the body buffer once Decode returns (rpc pools it), so
// nothing Decode returns may point into it.
func TestDecodeResultsDoNotAliasBody(t *testing.T) {
	for _, s := range decodeSeeds() {
		body := bytes.Clone(s)
		frames, tr, err := Decode(body)
		if err != nil {
			continue
		}
		want := make([]Frame, len(frames))
		for i, f := range frames {
			want[i] = f
			want[i].Payload = bytes.Clone(f.Payload)
			if f.Status != nil {
				st := *f.Status
				if st.Details != nil {
					st.Details = make([]jsontext.Value, 0, len(f.Status.Details))
				}
				for _, d := range f.Status.Details {
					st.Details = append(st.Details, bytes.Clone(d))
				}
				want[i].Status = &st
			}
		}
		wantTr := tr
		for i := range body {
			body[i] = 'X'
		}
		for i, f := range frames {
			if !bytes.Equal(f.Payload, want[i].Payload) || !reflect.DeepEqual(f.Status, want[i].Status) || f.ID != want[i].ID {
				t.Fatalf("%q: frame %d changed when the body was overwritten", s, i)
			}
		}
		if tr != wantTr {
			t.Fatalf("%q: trailers changed", s)
		}
	}
}

func TestDecodeMatchesRef(t *testing.T) {
	for _, s := range decodeSeeds() {
		checkDecodeMatchesRef(t, s)
	}
}

func FuzzDecodeMatchesRef(f *testing.F) {
	for _, s := range decodeSeeds() {
		f.Add(s)
	}
	f.Fuzz(checkDecodeMatchesRef)
}

// B4 budget: decoding a 40 KB list body must not copy the payload per
// column or per accessor.
func TestDecodeAllocBudget(t *testing.T) {
	if raceEnabled {
		t.Skip("allocation counts are not stable under the race detector")
	}
	body, _ := benchdata.ListBodyOfSize(40_000)
	var bytesPerOp uint64
	allocs := testing.AllocsPerRun(20, func() {
		frames, _, err := Decode(body)
		if err != nil || len(frames) != 1 {
			t.Fatalf("frames=%d err=%v", len(frames), err)
		}
	})
	res := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_, _, _ = Decode(body)
		}
	})
	bytesPerOp = uint64(res.AllocedBytesPerOp())
	if allocs > 20 || bytesPerOp > 2*uint64(len(body)) {
		t.Fatalf("Decode(40 KB) = %.0f allocs, %d B/op; budget 20 allocs, %d B", allocs, bytesPerOp, 2*len(body))
	}
	t.Logf("Decode(40 KB) = %.0f allocs, %d B/op", allocs, bytesPerOp)
}
