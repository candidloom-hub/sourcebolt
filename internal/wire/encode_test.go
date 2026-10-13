package wire

import (
	"net/url"
	"strings"
	"testing"
)

func TestEncodeBatchSingle(t *testing.T) {
	got, err := EncodeBatch([]BatchItem{{ID: "wXbhsf", Params: []any{nil, 1, nil, []any{2}}}}, "tok")
	if err != nil {
		t.Fatal(err)
	}
	want := "f.req=" + url.QueryEscape(`[[["wXbhsf","[null,1,null,[2]]",null,"generic"]]]`) + "&at=tok&"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestEncodeBatchMulti(t *testing.T) {
	got, err := EncodeBatch([]BatchItem{{ID: "a1", Params: []any{1}}, {ID: "b2", Params: []any{"x"}}}, "tok")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := url.QueryUnescape(strings.TrimSuffix(strings.TrimPrefix(got, "f.req="), "&at=tok&"))
	want := `[[["a1","[1]",null,"generic"],["b2","[\"x\"]",null,"generic"]]]`
	if raw != want {
		t.Fatalf("got  %s\nwant %s", raw, want)
	}
}

func TestEncodeStream(t *testing.T) {
	got, err := EncodeStream([]any{"q"}, "tok")
	if err != nil {
		t.Fatal(err)
	}
	want := "f.req=" + url.QueryEscape(`[null,"[\"q\"]"]`) + "&at=tok&"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestEncodeSpaces(t *testing.T) {
	got, err := EncodeBatch([]BatchItem{{ID: "x", Params: []any{"a b"}}}, "t")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "+") || !strings.Contains(got, "%20") {
		t.Fatalf("spaces must encode as %%20: %s", got)
	}
}
