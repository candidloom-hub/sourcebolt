package nlm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
)

const (
	readNB  = "00000000-0000-4000-8000-000000000001"
	readSID = "00000000-0000-4000-8000-0000000000aa"
)

func TestSourceTextShapes(t *testing.T) {
	cases := []struct {
		name     string
		fixture  string
		markdown bool
		params   string
		want     string
	}{
		{"text", "synthetic_text", false, `[["` + readSID + `"],[2],[2,null,[1],[1,null,null,null,null,null,null,null,null,null,[1,3]]]]`, "text-1\ntext-2"},
		{"markdown", "synthetic_markdown", true, `[["` + readSID + `"],[3],[3]]`, "# text-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ft := newFake(loadFixture(t, "hizoJc", tc.fixture))
			got, err := New(ft).SourceText(context.Background(), readNB, readSID, tc.markdown)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("text = %q, want %q", got, tc.want)
			}
			c, ok := ft.callFor("hizoJc")
			if !ok {
				t.Fatal("hizoJc not called")
			}
			if !sameJSON(t, callParams(t, c), []byte(tc.params)) {
				t.Fatalf("params = %s, want %s", callParams(t, c), tc.params)
			}
			if c.SourcePath != "/notebook/"+readNB {
				t.Fatalf("source path = %q", c.SourcePath)
			}
		})
	}
}

func TestSourceTextFallsBackToHTML(t *testing.T) {
	ft := newFake(synthFixture(t, "hizoJc", `[[["`+readSID+`"],"title-1"],null,null,null,[null,"<p>text-1</p>"]]`))
	got, err := New(ft).SourceText(context.Background(), readNB, readSID, false)
	if err != nil || got != "<p>text-1</p>" {
		t.Fatalf("got %q, %v", got, err)
	}
}

// Review item 6: no text blocks (and no HTML) is drift naming hizoJc and
// the mode, never an empty text.
func TestSourceTextEmptyIsDrift(t *testing.T) {
	for _, payload := range []string{
		`[[["` + readSID + `"],"title-1"]]`,
		`[[["` + readSID + `"],"title-1"],null,null,[[]]]`,
	} {
		for _, md := range []bool{false, true} {
			ft := newFake(synthFixture(t, "hizoJc", payload))
			got, err := New(ft).SourceText(context.Background(), readNB, readSID, md)
			var te *transport.Error
			mode := "2"
			if md {
				mode = "3"
			}
			if !errors.As(err, &te) || te.Kind != transport.KindDrift || !strings.Contains(te.Message, "hizoJc") || !strings.Contains(te.Message, "mode "+mode) || got != "" {
				t.Fatalf("%s markdown=%v: got %q, %v", payload, md, got, err)
			}
		}
	}
}

func TestRenameSource(t *testing.T) {
	ft := newFake(loadFixture(t, "b7Wfje", "synthetic_rename"))
	if err := New(ft).RenameSource(context.Background(), readNB, readSID, "title-2"); err != nil {
		t.Fatal(err)
	}
	c, _ := ft.callFor("b7Wfje")
	want := `[null,["` + readSID + `"],[[["title-2"]]]]`
	if !sameJSON(t, callParams(t, c), []byte(want)) {
		t.Fatalf("params = %s, want %s", callParams(t, c), want)
	}
}

func TestRenameSourceValidates(t *testing.T) {
	ft := newFake()
	c := New(ft)
	for _, a := range [][2]string{{"", "t"}, {readSID, ""}, {readSID, "  "}} {
		err := c.RenameSource(context.Background(), readNB, a[0], a[1])
		var te *transport.Error
		if !errors.As(err, &te) || te.Kind != transport.KindInvalidArgument {
			t.Fatalf("%v: err = %v, want KindInvalidArgument", a, err)
		}
	}
	if len(ft.calls) != 0 {
		t.Fatal("invalid input reached the transport")
	}
}

func TestSourceFresh(t *testing.T) {
	cases := []struct {
		fixture string
		want    bool
	}{{"synthetic_fresh_url", true}, {"synthetic_stale_drive", false}}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			ft := newFake(loadFixture(t, "yR9Yof", tc.fixture))
			got, err := New(ft).SourceFresh(context.Background(), readNB, readSID)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("fresh = %v, want %v", got, tc.want)
			}
			c, _ := ft.callFor("yR9Yof")
			want := `[null,["` + readSID + `"],[2]]`
			if !sameJSON(t, callParams(t, c), []byte(want)) {
				t.Fatalf("params = %s, want %s", callParams(t, c), want)
			}
		})
	}
}

func TestSourceFreshDriveTrue(t *testing.T) {
	ft := newFake(synthFixture(t, "yR9Yof", `[[null,true,["drive-id-1"]]]`))
	got, err := New(ft).SourceFresh(context.Background(), readNB, readSID)
	if err != nil || !got {
		t.Fatalf("got %v, %v", got, err)
	}
}
