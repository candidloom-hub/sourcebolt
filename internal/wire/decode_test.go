package wire

import (
	"errors"
	"strconv"
	"testing"
	"unicode/utf16"
)

const xssi = ")]}'\n\n"

// chunk builds one rt=c chunk with an explicit declared length.
func chunk(declared int, row string) string { return strconv.Itoa(declared) + "\n" + row + "\n" }

func pickOK(t *testing.T, body, id string) string {
	t.Helper()
	frames, _, err := Decode([]byte(body))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	v, err := Pick(frames, id, NullIsError)
	if err != nil {
		t.Fatalf("Pick: %v", err)
	}
	return string(v)
}

func wantWireErr(t *testing.T, err error) *Error {
	t.Helper()
	var we *Error
	if !errors.As(err, &we) {
		t.Fatalf("want *wire.Error, got %T %v", err, err)
	}
	return we
}

func TestDecodeSimple(t *testing.T) {
	row := `[["wrb.fr","wXbhsf","[[\"t\"]]",null,null,null,"generic"]]`
	got := pickOK(t, xssi+chunk(len(row), row), "wXbhsf")
	if got != `[["t"]]` {
		t.Fatalf("payload = %s", got)
	}
}

func TestDecodeWrongLengths(t *testing.T) {
	row := `[["wrb.fr","wXbhsf","[[\"日本😀\"]]",null,null,null,"generic"]]`
	for name, declared := range map[string]int{
		"bytes+2": len(row) + 2,
		"utf16":   len(utf16.Encode([]rune(row))),
		"zero":    0,
	} {
		t.Run(name, func(t *testing.T) {
			if got := pickOK(t, xssi+chunk(declared, row)+chunk(25, `[["di",42],["af.httprm",42,"-1",3]]`), "wXbhsf"); got != `[["日本😀"]]` {
				t.Fatalf("payload = %s", got)
			}
		})
	}
}

func TestDecodeControlChars(t *testing.T) {
	row := "[[\"wrb.fr\",\"cFji9\",\"[[\\\"line1\nline2\\\"]]\",null,null,null,\"generic\"]]"
	if got := pickOK(t, xssi+chunk(len(row), row), "cFji9"); got != `[["line1\nline2"]]` {
		t.Fatalf("payload = %q", got)
	}
}

func TestDecodeTripleEscaped(t *testing.T) {
	row := `[["wrb.fr","x1","\"\\\"[1,2]\\\"\"",null,null,null,"generic"]]`
	if got := pickOK(t, xssi+chunk(len(row), row), "x1"); got != `[1,2]` {
		t.Fatalf("payload = %s", got)
	}
}

func TestStatusIsError(t *testing.T) {
	row := `[["wrb.fr","cFji9",null,null,null,[3],"generic"]]`
	frames, _, err := Decode([]byte(xssi + chunk(len(row), row)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Pick(frames, "cFji9", NullAllowed)
	if we := wantWireErr(t, err); we.Code != 3 {
		t.Fatalf("code = %d", we.Code)
	}
}

func TestQuotaStatus(t *testing.T) {
	row := `[["wrb.fr","R7cb6c",null,null,null,[8,"",[["type.googleapis.com/google.internal.labs.tailwind.UserDisplayableError",[1]]]],"generic"]]`
	frames, _, _ := Decode([]byte(xssi + chunk(len(row), row)))
	_, err := Pick(frames, "R7cb6c", NullAllowed)
	we := wantWireErr(t, err)
	if we.Code != 8 || we.Status == nil || len(we.Status.Details) != 1 {
		t.Fatalf("got %+v", we)
	}
}

func TestErFrame(t *testing.T) {
	row := `[["er","wXbhsf",500]]`
	frames, _, _ := Decode([]byte(xssi + chunk(len(row), row)))
	_, err := Pick(frames, "wXbhsf", NullAllowed)
	if we := wantWireErr(t, err); !we.ServerErr {
		t.Fatalf("got %+v", we)
	}
}

func TestDrift(t *testing.T) {
	row := `[["wrb.fr","rLM1Ne","[1]",null,null,null,"generic"]]`
	frames, _, _ := Decode([]byte(xssi + chunk(len(row), row)))
	_, err := Pick(frames, "wXbhsf", NullAllowed)
	if we := wantWireErr(t, err); !we.Drift {
		t.Fatalf("got %+v", we)
	}
}

func TestSignedOut(t *testing.T) {
	_, _, err := Decode([]byte("<!doctype html><html><script>var a=[1,2];</script></html>"))
	if we := wantWireErr(t, err); !we.SignedOut {
		t.Fatalf("html: got %+v", we)
	}
	frames, _, err := Decode([]byte(xssi + chunk(10, `[["di",17]]`)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Pick(frames, "wXbhsf", NullAllowed)
	if we := wantWireErr(t, err); !we.SignedOut {
		t.Fatalf("no frames: got %+v", we)
	}
}

func TestNumericBody(t *testing.T) {
	_, _, err := Decode([]byte(")]}'\n16"))
	if we := wantWireErr(t, err); we.Code != 16 {
		t.Fatalf("got %+v", we)
	}
}

func TestNullPolicies(t *testing.T) {
	row := `[["wrb.fr","fejl7e",null,null,null,null,"generic"]]`
	frames, _, _ := Decode([]byte(xssi + chunk(len(row), row)))
	if _, err := Pick(frames, "fejl7e", NullIsError); err == nil {
		t.Fatal("NullIsError: want error")
	}
	v, err := Pick(frames, "fejl7e", NullAllowed)
	if err != nil || v != nil {
		t.Fatalf("NullAllowed: v=%s err=%v", v, err)
	}
}

func TestLastNonNullFrameWins(t *testing.T) {
	body := xssi + chunk(1, `[["wrb.fr","a",null,null,null,null,"generic"]]`) +
		chunk(1, `[["wrb.fr","a","[7]",null,null,null,"generic"]]`)
	if got := pickOK(t, body, "a"); got != `[7]` {
		t.Fatalf("payload = %s", got)
	}
}

func TestTrailers(t *testing.T) {
	row := `[["wrb.fr","a","[1]",null,null,null,"generic"],["af.httprm",13,"-123",9]]`
	_, tr, err := Decode([]byte(xssi + chunk(len(row), row)))
	if err != nil {
		t.Fatal(err)
	}
	if tr.FSID != "-123" {
		t.Fatalf("FSID = %q", tr.FSID)
	}
}

func TestMalformed(t *testing.T) {
	_, _, err := Decode([]byte(xssi + "5\n[[\"wrb.fr\",\n9\n[oops,}]\n"))
	if err == nil {
		t.Fatal("want malformed error")
	}
}

func TestAccessors(t *testing.T) {
	v := []byte(`[["title",3,"id-1"],[null,[5,"x"]]]`)
	if s := Str(v, 0, 0); s != "title" {
		t.Fatalf("Str = %q", s)
	}
	if n, ok := Int(v, 0, 1); !ok || n != 3 {
		t.Fatalf("Int = %d %v", n, ok)
	}
	if _, ok := At(v, 1, 0, 9); ok {
		t.Fatal("At into null should be !ok")
	}
	if s := Str(v, 1, 1, 1); s != "x" {
		t.Fatalf("Str nested = %q", s)
	}
}

func TestElems(t *testing.T) {
	es := Elems([]byte(`[[1],[2],[3]]`), 0)
	if len(es) != 1 || string(es[0]) != "1" {
		t.Fatalf("Elems(v,0) = %q", es)
	}
	if len(Elems([]byte(`[[1],[2],[3]]`))) != 3 {
		t.Fatal("Elems(v) should list top-level elements")
	}
	if Elems([]byte(`null`)) != nil {
		t.Fatal("Elems(null) should be nil")
	}
}

func TestSegments(t *testing.T) {
	segs, err := Segments([]byte(xssi + chunk(3, `[["di",1]]`) + chunk(3, "[[\"a\",\"x\ny\"]]")))
	if err != nil || len(segs) != 2 || string(segs[1]) != `[["a","x\ny"]]` {
		t.Fatalf("segs=%q err=%v", segs, err)
	}
	if _, err := Segments([]byte("<html>")); err == nil {
		t.Fatal("html must error")
	}
}

func TestNodeMatchesAccessors(t *testing.T) {
	v := []byte(`[["title",3,"id-1",null,[1.5,"x",[7,8]]],[null,[5,"y"]]]`)
	n, err := Parse(v)
	if err != nil {
		t.Fatal(err)
	}
	if n.At(0, 0).Str() != Str(v, 0, 0) || n.At(1, 1, 1).Str() != "y" {
		t.Fatal("Str mismatch")
	}
	if i, ok := n.At(0, 4, 2, 1).Int(); !ok || i != 8 {
		t.Fatalf("Int = %d %v", i, ok)
	}
	if !n.At(0, 3).IsNull() || !n.At(9).IsNull() || !n.At(0, 0, 1).IsNull() {
		t.Fatal("missing, null and non-array steps must be null")
	}
	if n.At(0).Len() != 5 || len(n.At(0, 4).Elems()) != 3 || n.At(0, 0).Len() != 0 {
		t.Fatal("Len/Elems mismatch")
	}
	if _, err := Parse([]byte(`[1,`)); err == nil {
		t.Fatal("invalid JSON must error")
	}
}
