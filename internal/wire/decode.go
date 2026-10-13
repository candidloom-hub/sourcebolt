package wire

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
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
		rows, ok := elems(seg)
		if !ok {
			bad++
			continue
		}
		for _, r := range rows {
			if f, ok := parseRow(r, &tr); ok {
				frames = append(frames, f)
			}
		}
	}
	if bad > 0 && bad*10 > total {
		return nil, tr, &Error{Malformed: true, Msg: "unparseable chunks"}
	}
	return frames, tr, nil
}

func parseRow(r jsontext.Value, tr *Trailers) (Frame, bool) {
	cols, ok := elems(r)
	if !ok || len(cols) == 0 {
		return Frame{}, false
	}
	var tag string
	if json.Unmarshal(cols[0], &tag) != nil {
		return Frame{}, false
	}
	switch tag {
	case "wrb.fr":
		f := Frame{ID: Str(r, 1)}
		if len(cols) > 2 {
			f.Payload = unquote(cols[2])
		}
		if len(cols) > 5 {
			f.Status = parseStatus(cols[5])
		}
		if len(cols) > 6 {
			f.Index = Str(r, 6)
		}
		return f, true
	case "er":
		n, _ := Int(r, 2)
		return Frame{ID: Str(r, 1), ServerErr: true, ErrCode: int(n)}, true
	case "af.httprm":
		if s := Str(r, 2); s != "" {
			tr.FSID = s
		}
	}
	return Frame{}, false
}

// unquote unwraps JSON-in-JSON: while the value is a JSON string, decode it
// and treat its text as JSON again (at most maxUnquote layers).
func unquote(v jsontext.Value) jsontext.Value {
	if isNull(v) {
		return nil
	}
	cur := bytes.TrimSpace(v)
	for i := 0; i < maxUnquote && len(cur) > 0 && cur[0] == '"'; i++ {
		var s string
		if json.Unmarshal(cur, &s, lenient) != nil {
			break
		}
		cur = escapeControl(bytes.TrimSpace([]byte(s)))
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
