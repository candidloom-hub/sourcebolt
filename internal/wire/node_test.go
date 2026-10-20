package wire

import (
	"bytes"
	"encoding/json/jsontext"
	"testing"
	"unicode/utf8"
)

// nodeCases are valid JSON values whose structure trips naive scanners:
// brackets and quotes inside strings, escaped backslashes before a quote,
// objects, literals, exponents, whitespace and invalid UTF-8.
var nodeCases = []string{
	`[]`,
	`[ ]`,
	`[1]`,
	`[ "a" , [ 1 , 2 ] , { "k" : [ 3 ] } , null , true , false , -1.5e+3 ]`,
	`["]", "[", "\"]", "\\", "\\\"", "a\\\\", "]"]`,
	`[["title",3,"id-1",null,[1.5,"x",[7,8]]],[null,[5,"y"]]]`,
	`[{"a":"]","b":{"c":["}"]}},[[[[]]]],"x"]`,
	"[\n\t\"a\",\r\n 2 ]",
	"[\"\xff\xfe\", 1]",
	`[0,-0,1e5,2E-3,123456789012345678901234567890]`,
	`"plain"`,
	`7`,
	`null`,
}

// TestNodeMatchesReference checks the fast structural walk against the
// jsontext-based accessors for every element and nested element.
func TestNodeMatchesReference(t *testing.T) {
	for _, c := range nodeCases {
		checkNodeAgainstReference(t, []byte(c))
	}
}

func FuzzNodeMatchesReference(f *testing.F) {
	for _, c := range nodeCases {
		f.Add([]byte(c))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if !jsontext.Value(b).IsValid(lenient) {
			return
		}
		checkNodeAgainstReference(t, b)
	})
}

func checkNodeAgainstReference(t *testing.T, b []byte) {
	t.Helper()
	n, err := Parse(b)
	if err != nil {
		t.Fatalf("Parse(%q): %v", b, err)
	}
	var walk func(n Node, ref jsontext.Value, depth int)
	walk = func(n Node, ref jsontext.Value, depth int) {
		want, ok := elems(ref)
		got := n.Elems()
		if !ok {
			if got != nil || n.Len() != 0 {
				t.Fatalf("%q: Elems of non-array = %v", ref, got)
			}
			return
		}
		if len(got) != len(want) || n.Len() != len(want) {
			t.Fatalf("%q: %d elems, want %d", ref, len(got), len(want))
		}
		for i, w := range want {
			w = bytes.TrimSpace(w)
			if !bytes.Equal(got[i].raw, w) {
				t.Fatalf("%q: elem %d = %q, want %q", ref, i, got[i].raw, w)
			}
			at := n.At(i)
			if isNull(w) != at.IsNull() || (!at.IsNull() && !bytes.Equal(at.raw, w)) {
				t.Fatalf("%q: At(%d) = %q, want %q", ref, i, at.raw, w)
			}
			// Str keeps invalid UTF-8 bytes as sent (fast path); the
			// reference replaces them, so compare valid text only.
			if utf8.Valid(w) && at.Str() != Str(ref, i) {
				t.Fatalf("%q: At(%d).Str() = %q, want %q", ref, i, at.Str(), Str(ref, i))
			}
			if depth < 8 {
				walk(got[i], w, depth+1)
			}
		}
		if !n.At(len(want)).IsNull() || !n.At(-1).IsNull() {
			t.Fatalf("%q: out-of-range At must be null", ref)
		}
	}
	walk(n, n.raw, 0)
}

// The fast walk must not allocate for At on a parsed row.
func TestNodeAtNoAllocs(t *testing.T) {
	if raceEnabled {
		t.Skip("allocation counts are not stable under the race detector")
	}
	n, _ := Parse([]byte(`[["t",[["s1"],["s2"]],"id",null,null,[1,null,null,null,null,[1700000000,5],null,null,[1600000000,7]]]]`))
	r := n.At(0)
	allocs := testing.AllocsPerRun(100, func() {
		if r.At(5, 8, 0).IsNull() || r.At(2).Str() == "" {
			t.Fatal("missing value")
		}
	})
	if allocs > 1 { // Str copies the string once
		t.Fatalf("At = %.0f allocs/op, want <= 1", allocs)
	}
}
