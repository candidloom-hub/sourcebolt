package wire

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"strconv"
	"sync"
)

// Node is a lazy, allocation-light view of a JSON payload. It keeps the raw
// bytes and walks them with a pooled jsontext.Decoder, skipping unneeded
// values without decoding them. At/Str/Int on a raw jsontext.Value decode
// whole subtrees into fresh slices on every call; for a 100-notebook list
// that cost ~40k allocations, a Node walk costs a few per row.
type Node struct{ raw jsontext.Value }

var errInvalidJSON = errors.New("wire: invalid JSON")

// Parse validates v once and wraps it.
func Parse(v jsontext.Value) (Node, error) {
	t := bytes.TrimSpace(v)
	if len(t) == 0 {
		return Node{}, nil
	}
	if !jsontext.Value(t).IsValid(lenient) {
		return Node{}, errInvalidJSON
	}
	return Node{t}, nil
}

type scanner struct {
	r bytes.Reader
	d jsontext.Decoder
}

var scanners = sync.Pool{New: func() any { return new(scanner) }}

// child returns the raw bytes of element i of the array raw, or nil.
func child(raw jsontext.Value, i int) jsontext.Value {
	if len(raw) == 0 || raw[0] != '[' || i < 0 {
		return nil
	}
	s := scanners.Get().(*scanner)
	defer scanners.Put(s)
	s.r.Reset(raw)
	s.d.Reset(&s.r, lenient)
	if tok, err := s.d.ReadToken(); err != nil || tok.Kind() != '[' {
		return nil
	}
	for j := 0; s.d.PeekKind() != ']'; j++ {
		start := s.d.InputOffset()
		if err := s.d.SkipValue(); err != nil {
			return nil
		}
		if j == i {
			return jsontext.Value(bytes.TrimLeft(raw[start:s.d.InputOffset()], " \t\r\n,"))
		}
	}
	return nil
}

// At walks positional indexes. A missing index, null or non-array step
// yields a null Node.
func (n Node) At(path ...int) Node {
	cur := n.raw
	for _, i := range path {
		if cur = child(cur, i); cur == nil {
			return Node{}
		}
	}
	if string(cur) == "null" {
		return Node{}
	}
	return Node{cur}
}

func (n Node) IsNull() bool { return len(n.raw) == 0 || string(n.raw) == "null" }

// Str returns the string value, or "".
func (n Node) Str() string {
	if len(n.raw) < 2 || n.raw[0] != '"' {
		return ""
	}
	if bytes.IndexByte(n.raw[1:len(n.raw)-1], '\\') < 0 {
		return string(n.raw[1 : len(n.raw)-1]) // fast path: no escapes
	}
	var s string
	if json.Unmarshal(n.raw, &s, lenient) != nil {
		return ""
	}
	return s
}

// Int returns the numeric value truncated to int64.
func (n Node) Int() (int64, bool) {
	if len(n.raw) == 0 || !(n.raw[0] == '-' || (n.raw[0] >= '0' && n.raw[0] <= '9')) {
		return 0, false
	}
	if i, err := strconv.ParseInt(string(n.raw), 10, 64); err == nil {
		return i, true
	}
	f, err := strconv.ParseFloat(string(n.raw), 64)
	return int64(f), err == nil
}

// Len is the array length (0 for non-arrays).
func (n Node) Len() int { return len(n.Elems()) }

// Elems returns the array's elements (nil for non-arrays).
func (n Node) Elems() []Node {
	if len(n.raw) == 0 || n.raw[0] != '[' {
		return nil
	}
	s := scanners.Get().(*scanner)
	defer scanners.Put(s)
	s.r.Reset(n.raw)
	s.d.Reset(&s.r, lenient)
	if tok, err := s.d.ReadToken(); err != nil || tok.Kind() != '[' {
		return nil
	}
	var out []Node
	for s.d.PeekKind() != ']' {
		start := s.d.InputOffset()
		if err := s.d.SkipValue(); err != nil {
			return nil
		}
		out = append(out, Node{jsontext.Value(bytes.TrimLeft(n.raw[start:s.d.InputOffset()], " \t\r\n,"))})
	}
	return out
}
