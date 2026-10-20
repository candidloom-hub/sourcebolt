package wire

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"unicode/utf8"
)

// Frame is one RPC result row: ["wrb.fr", id, payload, null, null, status, index]
// or a server error row ["er", id, code].
type Frame struct {
	ID        string
	Payload   jsontext.Value // inner JSON, unquoted; nil when null
	Status    *Status
	Index     string
	ServerErr bool
	ErrCode   int
}

// Trailers carries metadata rows that change client state.
type Trailers struct {
	FSID string // rotated f.sid from ["af.httprm", _, "<sid>", _]
}

// NullPolicy says how Pick treats a frame whose payload is null without a status.
type NullPolicy int

const (
	NullIsError NullPolicy = iota
	NullAllowed
	NullAllowedUnlessRejected
)

const maxUnquote = 3

// Decode parses a batchexecute or stream response body into frames.
func Decode(body []byte) ([]Frame, Trailers, error) {
	var tr Trailers
	rest, hasPrefix := stripPrefix(body)
	if !hasPrefix && bytes.HasPrefix(rest, []byte("<")) {
		return nil, tr, &Error{SignedOut: true, Msg: "html page"}
	}
	if n, ok := numericBody(rest); ok {
		return nil, tr, &Error{Code: n, Msg: "numeric error body"}
	}
	segs, unterminated := splitTopLevel(escapeControl(rest))
	bad, total := 0, len(segs)
	if unterminated {
		bad++
		total++
	}
	var frames []Frame
	for _, seg := range segs {
		// Validate each segment once, then walk it structurally: no copy
		// per row or column (B4 went from 93 allocs and 9x the body in
		// bytes to a handful of allocations).
		if !jsontext.Value(seg).IsValid(lenient) {
			bad++
			continue
		}
		for off := 1; ; {
			r, next, ok := nextElem(seg, off)
			if !ok {
				break
			}
			off = next
			if f, ok := parseRow(Node{r}, &tr); ok {
				frames = append(frames, f)
			}
		}
	}
	if bad > 0 && bad*10 > total {
		return nil, tr, &Error{Malformed: true, Msg: "unparseable chunks"}
	}
	return frames, tr, nil
}

// parseRow reads one row of a validated segment.
func parseRow(r Node, tr *Trailers) (Frame, bool) {
	var cb [7]Node
	cols := r.Head(cb[:])
	if len(cols) == 0 || cols[0].raw[0] != '"' {
		return Frame{}, false
	}
	switch strValue(cols[0]) {
	case "wrb.fr":
		f := Frame{ID: strValue(Get(cols, 1))}
		if len(cols) > 2 {
			f.Payload = unquote(cols[2].raw)
		}
		if len(cols) > 5 {
			f.Status = parseStatus(cols[5].raw)
		}
		if len(cols) > 6 {
			f.Index = strValue(cols[6])
		}
		return f, true
	case "er":
		var n int64
		if len(cols) > 2 {
			n, _ = Int(cols[2].raw)
		}
		return Frame{ID: strValue(Get(cols, 1)), ServerErr: true, ErrCode: int(n)}, true
	case "af.httprm":
		if s := strValue(Get(cols, 2)); s != "" {
			tr.FSID = s
		}
	}
	return Frame{}, false
}

// strValue decodes a string node exactly as Str on a raw value does
// (invalid UTF-8 becomes U+FFFD); "" for anything else.
func strValue(n Node) string {
	if len(n.raw) < 2 || n.raw[0] != '"' {
		return ""
	}
	in := n.raw[1 : len(n.raw)-1]
	if bytes.IndexByte(in, '\\') < 0 && utf8.Valid(in) {
		return string(in)
	}
	var s string
	if json.Unmarshal(n.raw, &s, lenient) != nil {
		return ""
	}
	return s
}

// unquote unwraps JSON-in-JSON: while the value is a JSON string, decode it
// and treat its text as JSON again (at most maxUnquote layers).
func unquote(v jsontext.Value) jsontext.Value {
	if isNull(v) {
		return nil
	}
	cur := bytes.TrimSpace(v)
	for i := 0; i < maxUnquote && len(cur) > 0 && cur[0] == '"'; i++ {
		s, err := jsontext.AppendUnquote(make([]byte, 0, len(cur)), cur)
		if err != nil {
			// Invalid UTF-8 (replaced, as below) or not one string.
			var str string
			if json.Unmarshal(cur, &str, lenient) != nil {
				break
			}
			s = []byte(str)
		}
		cur = escapeControl(bytes.TrimSpace(s))
	}
	if isNull(cur) {
		return nil
	}
	return jsontext.Value(cur)
}

// Pick returns the payload for id. The last non-null frame wins; an "er"
// frame, or a status on a null frame, is an error.
func Pick(frames []Frame, id string, policy NullPolicy) (jsontext.Value, error) {
	if len(frames) == 0 {
		return nil, &Error{ID: id, SignedOut: true}
	}
	var payload jsontext.Value
	var status *Status
	found := false
	for _, f := range frames {
		if f.ID != id {
			continue
		}
		found = true
		if f.ServerErr {
			return nil, &Error{ID: id, ServerErr: true, Code: f.ErrCode}
		}
		if f.Payload != nil {
			payload = f.Payload
		} else if f.Status != nil {
			status = f.Status
		}
	}
	switch {
	case !found:
		return nil, &Error{ID: id, Drift: true}
	case payload != nil:
		return payload, nil
	case status != nil:
		return nil, &Error{ID: id, Code: status.Code, Msg: status.Message, Status: status}
	case policy == NullIsError:
		return nil, &Error{ID: id, Msg: "null payload"}
	}
	return nil, nil
}

// Segments returns the raw top-level JSON segments of a response body with
// raw control characters escaped. Tools (fixture sanitizer) use it to
// rewrite bodies without depending on chunk lengths.
func Segments(body []byte) ([][]byte, error) {
	rest, hasPrefix := stripPrefix(body)
	if !hasPrefix && bytes.HasPrefix(rest, []byte("<")) {
		return nil, &Error{SignedOut: true, Msg: "html page"}
	}
	segs, unterminated := splitTopLevel(escapeControl(rest))
	if unterminated {
		return segs, &Error{Malformed: true, Msg: "unterminated segment"}
	}
	return segs, nil
}
