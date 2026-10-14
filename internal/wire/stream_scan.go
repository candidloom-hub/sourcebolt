package wire

import (
	"bytes"
	"unicode/utf16"
	"unicode/utf8"
)

// splitElems splits a JSON array into its raw elements as sub-slices of b, no
// copies. Unlike elems it does not validate leaves; the leaf decoders do.
func splitElems(b []byte) ([][]byte, bool) {
	b = bytes.TrimSpace(b)
	if len(b) < 2 || b[0] != '[' || b[len(b)-1] != ']' {
		return nil, false
	}
	var out [][]byte
	depth, start := 0, 1
	inStr, esc := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
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
			inStr = true
		case '[', '{':
			depth++
		case ']', '}':
			depth--
			if depth < 0 {
				return nil, false
			}
			if depth == 0 && i == len(b)-1 {
				if e := bytes.TrimSpace(b[start:i]); len(e) > 0 || len(out) > 0 {
					out = append(out, e)
				}
			}
		case ',':
			if depth == 1 {
				out = append(out, bytes.TrimSpace(b[start:i]))
				start = i + 1
			}
		}
	}
	return out, depth == 0 && !inStr
}

// unquoteInto decodes the JSON string literal s into dst[:0], reusing its
// capacity. Bad escapes decode leniently; a missing closing quote ends the
// string at the end of s.
func unquoteInto(dst, s []byte) []byte {
	dst = dst[:0]
	if len(s) > 0 && s[0] == '"' {
		s = s[1:]
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' {
			break
		}
		if c != '\\' || i+1 >= len(s) {
			dst = append(dst, c)
			continue
		}
		i++
		switch s[i] {
		case 'n':
			dst = append(dst, '\n')
		case 't':
			dst = append(dst, '\t')
		case 'r':
			dst = append(dst, '\r')
		case 'b':
			dst = append(dst, '\b')
		case 'f':
			dst = append(dst, '\f')
		case 'u':
			r, ok := hex4(s[i+1:])
			if !ok {
				dst = append(dst, 'u')
				break
			}
			i += 4
			if utf16.IsSurrogate(r) {
				rest := s[i+1:]
				if len(rest) >= 6 && rest[0] == '\\' && rest[1] == 'u' {
					if r2, ok := hex4(rest[2:]); ok {
						if d := utf16.DecodeRune(r, r2); d != utf8.RuneError {
							r, i = d, i+6
						}
					}
				}
			}
			dst = utf8.AppendRune(dst, r)
		default: // \" \\ \/ and unknown escapes keep the escaped byte
			dst = append(dst, s[i])
		}
	}
	return dst
}

func hex4(s []byte) (r rune, ok bool) {
	if len(s) < 4 {
		return 0, false
	}
	for _, c := range s[:4] {
		var v byte
		switch {
		case c >= '0' && c <= '9':
			v = c - '0'
		case c >= 'a' && c <= 'f':
			v = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			v = c - 'A' + 10
		default:
			return 0, false
		}
		r = r<<4 | rune(v)
	}
	return r, true
}

// unquote unwraps JSON-in-JSON like the package-level unquote, decoding into
// the decoder's two reusable buffers so working memory stays at one snapshot.
// The result is valid until the next call.
func (d *StreamDecoder) unquote(v []byte) []byte {
	cur := bytes.TrimSpace(v)
	for i := 0; i < maxUnquote && len(cur) > 0 && cur[0] == '"'; i++ {
		d.ubuf[i%2] = unquoteInto(d.ubuf[i%2], cur)
		cur = escapeControl(bytes.TrimSpace(d.ubuf[i%2]))
	}
	if isNull(cur) {
		return nil
	}
	return cur
}
