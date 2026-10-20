package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const assets = "../../docs/assets/term"

// wellFormed fails the test when svg is not valid XML, and returns its text.
func wellFormed(t *testing.T, svg []byte) string {
	t.Helper()
	d := xml.NewDecoder(bytes.NewReader(svg))
	var text strings.Builder
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return text.String()
		}
		if err != nil {
			t.Fatalf("not well-formed XML: %v\n%s", err, svg)
		}
		if c, ok := tok.(xml.CharData); ok {
			text.Write(c)
		}
	}
}

func TestEscapesXML(t *testing.T) {
	line := `PS C:\> echo "<a & b>" 'c'`
	svg := render(`x < y & "z"`, []string{line, `]]> <!-- -->`})
	for _, raw := range []string{"<a & b>", "<!-- -->", `"<a`} {
		if bytes.Contains(svg, []byte(raw)) {
			t.Fatalf("unescaped %q in output", raw)
		}
	}
	text := wellFormed(t, svg)
	for _, want := range []string{`echo "<a & b>" 'c'`, `]]> <!-- -->`, `x < y & "z"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("text %q missing from rendered SVG", want)
		}
	}
}

func TestDeterministicAndKeepsLines(t *testing.T) {
	src := []byte("title: T\r\nPS C:\\Users\\you> sourcebolt --version\r\nsourcebolt 0.3.1\r\n\r\n  - two  spaces kept\r\n")
	title, lines, err := parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if title != "T" || len(lines) != 4 || lines[3] != "  - two  spaces kept" {
		t.Fatalf("parse: %q %q", title, lines)
	}
	a, b := render(title, lines), render(title, lines)
	if !bytes.Equal(a, b) {
		t.Fatal("render is not deterministic")
	}
	if !bytes.Contains(a, []byte(`<tspan fill="#4fc1ff">PS C:\Users\you&gt; </tspan>sourcebolt --version`)) {
		t.Fatalf("prompt not highlighted:\n%s", a)
	}
	wellFormed(t, a)
}

func TestWidthFollowsLongestLine(t *testing.T) {
	short := render("t", []string{"a"})
	long := render("t", []string{strings.Repeat("x", 200)})
	if !bytes.Contains(short, []byte(`width="480"`)) || !bytes.Contains(long, []byte(`width="1752"`)) {
		t.Fatalf("widths:\n%s\n%s", short[:120], long[:120])
	}
}

func TestRejectsBadInput(t *testing.T) {
	for _, src := range []string{"no title line\n", "title: t\n\tTab\n"} {
		if _, _, err := parse([]byte(src)); err == nil {
			t.Fatalf("parse(%q) should fail", src)
		}
	}
}

// Every committed picture must match its transcript; regenerate with
// go run ./tools/termshot docs/assets/term
func TestAssetsAreCurrent(t *testing.T) {
	txts, _ := filepath.Glob(filepath.Join(assets, "*.txt"))
	svgs, _ := filepath.Glob(filepath.Join(assets, "*.svg"))
	if len(txts) == 0 || len(txts) != len(svgs) {
		t.Fatalf("%d .txt and %d .svg files in %s; each .svg needs its .txt", len(txts), len(svgs), assets)
	}
	for _, txt := range txts {
		want, err := convert(txt)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(strings.TrimSuffix(txt, ".txt") + ".svg")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n")), want) {
			t.Fatalf("%s is stale; run: go run ./tools/termshot docs/assets/term", txt)
		}
	}
}
