package wire

import (
	"bytes"
	"strconv"
)

// escapeControl rewrites raw U+0000–U+001F bytes inside JSON strings as
// \u00XX. Google sometimes sends titles and notes with literal newlines.
// Returns b unchanged (no copy) when nothing needs escaping.
func escapeControl(b []byte) []byte {
	var out []byte
	last := 0 // b[last:i] not yet copied to out
	i := 0
	for {
		// Outside a string: jump to the next quote.
		j := bytes.IndexByte(b[i:], '"')
		if j < 0 {
			break
		}
		i += j + 1
		// Inside a string: stop only at a quote, backslash or control byte.
		for i < len(b) {
			c := b[i]
			if !strSpecial[c] {
				i++
				continue
			}
			if c == '"' {
				i++
				break
			}
			if c == '\\' {
				i += 2 // the escaped byte is never rewritten
				continue
			}
			if out == nil {
				out = make([]byte, 0, len(b)+16)
			}
			out = append(out, b[last:i]...)
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
			i++
			last = i
		}
		if i >= len(b) {
			break
		}
	}
	if out == nil {
		return b
	}
	return append(out, b[min(last, len(b)):]...)
}

// strSpecial marks the bytes escapeControl must look at inside a string.
var strSpecial = func() (t [256]bool) {
	for c := range 0x20 {
		t[c] = true
	}
	t['"'], t['\\'] = true, true
	return t
}()

func hexDigit(n byte) byte { return "0123456789abcdef"[n] }

// splitTopLevel returns every balanced top-level [...] segment, ignoring
// anything outside brackets (chunk length lines). Declared chunk lengths
// are unreliable (UTF-16 units vs bytes), so structure decides.
// unterminated reports a trailing segment that never closed.
func splitTopLevel(b []byte) (segs [][]byte, unterminated bool) {
	depth, start := 0, -1
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
			}
			continue
		}
		switch c {
		case '"':
			if depth > 0 {
				inStr = true
			}
		case '[':
			if depth == 0 {
				start = i
			}
			depth++
		case ']':
			if depth > 0 {
				depth--
				if depth == 0 {
					segs = append(segs, b[start:i+1])
				}
			}
		}
	}
	return segs, depth > 0
}

var (
	xssiContiguous = []byte(")]}'")
	xssiPerLine    = []byte(")\n]\n}\n'")
)

// stripPrefix removes the XSSI guard, contiguous or one char per line.
func stripPrefix(b []byte) ([]byte, bool) {
	t := bytes.TrimLeft(b, " \t\r\n")
	if bytes.HasPrefix(t, xssiContiguous) {
		return t[len(xssiContiguous):], true
	}
	if bytes.HasPrefix(t, xssiPerLine) {
		return t[len(xssiPerLine):], true
	}
	return t, false
}

// numericBody reports a body that is only a decimal error code.
func numericBody(b []byte) (int, bool) {
	t := bytes.TrimSpace(b)
	if len(t) == 0 {
		return 0, false
	}
	// Check the bytes first: Atoi on a whole response body copied it twice
	// (the string conversion and the syntax error).
	for i, c := range t {
		if (c < '0' || c > '9') && !(i == 0 && (c == '+' || c == '-')) {
			return 0, false
		}
	}
	n, err := strconv.Atoi(string(t))
	return n, err == nil
}
