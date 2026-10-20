package wire

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"strconv"
)

// Node is a lazy, allocation-free view of a JSON payload. It keeps the raw
// bytes, validated once by Parse, and walks them structurally, skipping
// unneeded values without decoding them. At/Str/Int on a raw jsontext.Value
// decode whole subtrees into fresh slices on every call; for a 100-notebook
// list that cost ~40k allocations, a Node walk costs none.
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

// skipValue returns the offset just past the JSON value starting at b[i].
// It trusts b to be valid (Parse checked it), so it only tracks strings and
// bracket depth: a byte loop that is several times faster than a validating
// jsontext.Decoder and never allocates.
func skipValue(b []byte, i int) int {
	switch b[i] {
	case '"':
		return skipString(b, i)
	case '[', '{':
		depth := 0
		for i < len(b) {
			switch b[i] {
			case '"':
				i = skipString(b, i)
				continue
			case '[', '{':
				depth++
			case ']', '}':
				depth--
				if depth == 0 {
					return i + 1
				}
			}
			i++
		}
		return i
	}
	for i < len(b) { // number or literal
		switch b[i] {
		case ',', ']', '}', ' ', '\t', '\r', '\n':
			return i
		}
		i++
	}
	return i
}

// skipString returns the offset just past the string starting at b[i] == '"'.
func skipString(b []byte, i int) int {
	i++
	for {
		j := bytes.IndexByte(b[i:], '"')
		if j < 0 {
			return len(b)
		}
		i += j
		bs := 0 // backslashes right before the quote
		for k := i - 1; k >= 0 && b[k] == '\\'; k-- {
			bs++
		}
		i++
		if bs%2 == 0 {
			return i
		}
	}
}

func skipSpace(b []byte, i int) int {
	for i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '\r' || b[i] == '\n') {
		i++
	}
	return i
}

// nextElem returns the element of array raw that starts at or after offset
// i (just past '[' or a previous element) and the offset after it; ok is
// false at the closing bracket. raw must be a valid array.
func nextElem(raw []byte, i int) (e []byte, next int, ok bool) {
	i = skipSpace(raw, i)
	if i < len(raw) && raw[i] == ',' {
		i = skipSpace(raw, i+1)
	}
	if i >= len(raw) || raw[i] == ']' {
		return nil, i, false
	}
	end := skipValue(raw, i)
	return raw[i:end], end, true
}

// child returns the raw bytes of element i of the array raw, or nil.
func child(raw jsontext.Value, i int) jsontext.Value {
	if len(raw) == 0 || raw[0] != '[' || i < 0 {
		return nil
	}
	off := 1
	for j := 0; ; j++ {
		e, next, ok := nextElem(raw, off)
		if !ok {
			return nil
		}
		if j == i {
			return e
		}
		off = next
	}
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
func (n Node) Len() int {
	if len(n.raw) == 0 || n.raw[0] != '[' {
		return 0
	}
	c := 0
	for off, ok := 1, true; ; c++ {
		if _, off, ok = nextElem(n.raw, off); !ok {
			return c
		}
	}
}

// Elems returns the array's elements (nil for non-arrays).
func (n Node) Elems() []Node {
	if len(n.raw) == 0 || n.raw[0] != '[' {
		return nil
	}
	var out []Node
	for off := 1; ; {
		e, next, ok := nextElem(n.raw, off)
		if !ok {
			return out
		}
		out = append(out, Node{e})
		off = next
	}
}
