// Command termshot turns terminal transcripts into SVG "terminal window"
// pictures for the beginner guides.
//
//	go run ./tools/termshot docs/assets/term
//
// For every NAME.txt in the folder it writes NAME.svg next to it. The first
// line of a .txt file is "title: <window title>"; every other line is drawn
// as is, in a monospace font, without wrapping. The picture is as wide as the
// longest line. Lines that start with a shell prompt ("PS C:\...> " or
// "$ ") draw the prompt in a second color.
//
// The output depends only on the input, so a test can check that every
// committed .svg matches its .txt.
package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	fontSize   = 14
	charWidth  = 8.6 // generous for 14px Consolas, Menlo and DejaVu Sans Mono
	lineHeight = 20
	padX       = 16
	padY       = 14
	titleBar   = 32
	minWidth   = 480
)

var promptRE = regexp.MustCompile(`^(PS [^>]*> ?|\$ )`)

// parse splits a transcript into its window title and its lines.
func parse(src []byte) (title string, lines []string, err error) {
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	lines = strings.Split(text, "\n")
	head, ok := strings.CutPrefix(lines[0], "title: ")
	if !ok {
		return "", nil, fmt.Errorf(`first line must be "title: <window title>"`)
	}
	for i, l := range lines[1:] {
		if strings.Contains(l, "\t") {
			return "", nil, fmt.Errorf("line %d has a tab; use spaces", i+2)
		}
	}
	return head, lines[1:], nil
}

func esc(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s)) // never fails on a bytes.Buffer
	return b.String()
}

// render draws the transcript as an SVG terminal window.
func render(title string, lines []string) []byte {
	cols := 0
	for _, l := range lines {
		cols = max(cols, utf8.RuneCountInString(l))
	}
	w := max(minWidth, int(float64(cols)*charWidth)+2*padX)
	h := titleBar + 2*padY + len(lines)*lineHeight

	var b bytes.Buffer
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="%s">`+"\n", w, h, w, h, esc(title))
	fmt.Fprintf(&b, `<title>%s</title>`+"\n", esc(title))
	fmt.Fprintf(&b, `<rect width="%d" height="%d" rx="8" fill="#1e1e1e"/>`+"\n", w, h)
	fmt.Fprintf(&b, `<path d="M0 8a8 8 0 0 1 8-8h%d a8 8 0 0 1 8 8v%dH0z" fill="#323233"/>`+"\n", w-16, titleBar-8)
	for i, c := range []string{"#ff5f57", "#febc2e", "#28c840"} {
		fmt.Fprintf(&b, `<circle cx="%d" cy="16" r="6" fill="%s"/>`+"\n", 20+i*20, c)
	}
	fmt.Fprintf(&b, `<text x="%d" y="21" text-anchor="middle" font-family="Segoe UI, Helvetica, Arial, sans-serif" font-size="13" fill="#cccccc">%s</text>`+"\n", w/2, esc(title))
	fmt.Fprintf(&b, `<g font-family="Consolas, Menlo, 'DejaVu Sans Mono', monospace" font-size="%d" fill="#d4d4d4" xml:space="preserve">`+"\n", fontSize)
	for i, l := range lines {
		y := titleBar + padY + (i+1)*lineHeight - 5
		if l == "" {
			continue
		}
		if p := promptRE.FindString(l); p != "" {
			fmt.Fprintf(&b, `<text x="%d" y="%d"><tspan fill="#4fc1ff">%s</tspan>%s</text>`+"\n", padX, y, esc(p), esc(l[len(p):]))
			continue
		}
		fmt.Fprintf(&b, `<text x="%d" y="%d">%s</text>`+"\n", padX, y, esc(l))
	}
	b.WriteString("</g>\n</svg>\n")
	return b.Bytes()
}

// convert renders one .txt file.
func convert(path string) ([]byte, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	title, lines, err := parse(src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return render(title, lines), nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/termshot <folder with .txt files>")
		os.Exit(2)
	}
	txts, err := filepath.Glob(filepath.Join(os.Args[1], "*.txt"))
	if err == nil && len(txts) == 0 {
		err = fmt.Errorf("no .txt files in %s", os.Args[1])
	}
	for _, t := range txts {
		if err != nil {
			break
		}
		var svg []byte
		if svg, err = convert(t); err == nil {
			err = os.WriteFile(strings.TrimSuffix(t, ".txt")+".svg", svg, 0o644)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "termshot:", err)
		os.Exit(1)
	}
}
