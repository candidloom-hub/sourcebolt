// Package setup connects sourcebolt to the MCP apps (clients) installed for
// this user: it finds their config files, adds one server entry without
// touching anything else, keeps a backup, and can undo.
package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

// Entry is one stdio MCP server entry in a client config.
type Entry struct {
	Type    string   `json:"type,omitempty"` // "stdio" where the client's format has it
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

var bom = []byte{0xEF, 0xBB, 0xBF}

// ErrNotJSON means the file is not plain JSON (comments, trailing commas, or
// not an object), so it is left alone.
var ErrNotJSON = errors.New("the file is not plain JSON (it may contain comments); add the entry by hand")

// member is one "name": value pair; offsets are into the whole document.
type member struct {
	name             string
	keyStart, valEnd int
	valStart         int
}

// objectMembers lists the members of the object doc[start:end], where
// doc[start] is '{' and doc[end-1] is '}'.
func objectMembers(doc []byte, start, end int) ([]member, error) {
	dec := json.NewDecoder(bytes.NewReader(doc[start:end]))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, ErrNotJSON
	}
	var out []member
	for dec.More() {
		keyStart := start + int(dec.InputOffset())
		for keyStart < end && (isSpace(doc[keyStart]) || doc[keyStart] == ',') {
			keyStart++
		}
		t, err := dec.Token()
		if err != nil {
			return nil, ErrNotJSON
		}
		name, _ := t.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, ErrNotJSON
		}
		valEnd := start + int(dec.InputOffset())
		out = append(out, member{name: name, keyStart: keyStart, valStart: valEnd - len(raw), valEnd: valEnd})
	}
	return out, nil
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// root returns the bounds of the top-level value, after checking that the
// whole document is plain JSON (json.Valid also catches text after the
// object). objectMembers rejects a value that is not an object.
func root(doc []byte) (start, end int, err error) {
	if !json.Valid(doc) {
		return 0, 0, ErrNotJSON
	}
	start = len(doc) - len(bytes.TrimLeft(doc, " \t\r\n"))
	end = len(bytes.TrimRight(doc, " \t\r\n"))
	return start, end, nil
}

// indentOf guesses the file's indent unit from its first indented line.
func indentOf(doc []byte) string {
	for _, line := range strings.Split(string(doc), "\n")[1:] {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed != "" && len(trimmed) < len(line) {
			return line[:len(line)-len(trimmed)]
		}
	}
	return "  "
}

func find(ms []member, name string) (member, bool) {
	for _, m := range ms {
		if m.name == name {
			return m, true
		}
	}
	return member{}, false
}

// servers finds the top-level key object. ok is false when the key is
// missing; an existing key that is not an object is an error.
func servers(doc []byte, key string) (rootStart, rootEnd int, rootMs []member, obj member, ok bool, err error) {
	rootStart, rootEnd, err = root(doc)
	if err != nil {
		return
	}
	rootMs, err = objectMembers(doc, rootStart, rootEnd)
	if err != nil {
		return
	}
	obj, ok = find(rootMs, key)
	return
}

// Lookup returns the entry name under the top-level key, if any.
func Lookup(doc []byte, key, name string) (Entry, bool, error) {
	raw, ok, err := LookupRaw(doc, key, name)
	if err != nil || !ok {
		return Entry{}, false, err
	}
	var e Entry
	_ = json.Unmarshal(raw, &e)
	return e, true, nil
}

// LookupRaw returns the entry's JSON text, with every field it has.
func LookupRaw(doc []byte, key, name string) (json.RawMessage, bool, error) {
	doc = bytes.TrimPrefix(doc, bom)
	if len(bytes.TrimSpace(doc)) == 0 {
		return nil, false, nil
	}
	_, _, _, obj, ok, err := servers(doc, key)
	if err != nil || !ok {
		return nil, false, err
	}
	ms, err := objectMembers(doc, obj.valStart, obj.valEnd)
	if err != nil {
		return nil, false, err
	}
	m, ok := find(ms, name)
	if !ok {
		return nil, false, nil
	}
	return json.RawMessage(doc[m.valStart:m.valEnd]), true, nil
}

// marshalAt formats v for a member at the given depth (1 = member of the
// top-level object).
func marshalAt(v any, indent string, depth int) []byte {
	b, _ := json.MarshalIndent(v, strings.Repeat(indent, depth), indent)
	return b
}

// insert adds "name": value to the object doc[start:end] at depth.
func insert(doc []byte, start, end int, ms []member, name string, value []byte, indent string, depth int) []byte {
	key, _ := json.Marshal(name)
	pad := "\n" + strings.Repeat(indent, depth+1)
	var out bytes.Buffer
	if len(ms) == 0 {
		out.Write(doc[:start+1])
		out.WriteString(pad)
		out.Write(key)
		out.WriteString(": ")
		out.Write(value)
		out.WriteString("\n" + strings.Repeat(indent, depth))
		out.Write(doc[end-1:])
		return out.Bytes()
	}
	last := ms[len(ms)-1].valEnd
	out.Write(doc[:last])
	out.WriteString(",")
	out.WriteString(pad)
	out.Write(key)
	out.WriteString(": ")
	out.Write(value)
	out.Write(doc[last:])
	return out.Bytes()
}

// Merge adds (or replaces) the entry name under the top-level key, and
// keeps every other byte of the file. existed reports a replaced entry.
func Merge(doc []byte, key, name string, e any) (out []byte, existed bool, err error) {
	hadBOM := bytes.HasPrefix(doc, bom)
	doc = bytes.TrimPrefix(doc, bom)
	if len(bytes.TrimSpace(doc)) == 0 {
		doc = []byte("{}\n")
	}
	indent := indentOf(doc)
	rs, re, rootMs, obj, ok, err := servers(doc, key)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		out = insert(doc, rs, re, rootMs, key, marshalAt(map[string]any{name: e}, indent, 1), indent, 0)
	} else {
		ms, err := objectMembers(doc, obj.valStart, obj.valEnd)
		if err != nil {
			return nil, false, err
		}
		value := marshalAt(e, indent, 2)
		if m, found := find(ms, name); found {
			existed = true
			out = append(append(append([]byte{}, doc[:m.valStart]...), value...), doc[m.valEnd:]...)
		} else {
			out = insert(doc, obj.valStart, obj.valEnd, ms, name, value, indent, 1)
		}
	}
	if !json.Valid(out) {
		return nil, false, errors.New("setup: internal error: merged config is not valid JSON")
	}
	if hadBOM {
		out = append(append([]byte{}, bom...), out...)
	}
	return out, existed, nil
}

// Remove deletes the entry name under the top-level key, if present.
func Remove(doc []byte, key, name string) ([]byte, error) {
	hadBOM := bytes.HasPrefix(doc, bom)
	doc = bytes.TrimPrefix(doc, bom)
	_, _, _, obj, ok, err := servers(doc, key)
	if err != nil || !ok {
		return prefixBOM(doc, hadBOM), err
	}
	ms, err := objectMembers(doc, obj.valStart, obj.valEnd)
	if err != nil {
		return nil, err
	}
	i := -1
	for j, m := range ms {
		if m.name == name {
			i = j
		}
	}
	if i < 0 {
		return prefixBOM(doc, hadBOM), nil
	}
	var from, to int
	switch {
	case i > 0: // drop ", "name": value" after the previous member
		from, to = ms[i-1].valEnd, ms[i].valEnd
	case len(ms) > 1: // first of several: drop up to the next key
		from, to = ms[i].keyStart, ms[i+1].keyStart
	default: // the only member: leave an empty object
		from, to = obj.valStart+1, obj.valEnd-1
	}
	out := append(append([]byte{}, doc[:from]...), doc[to:]...)
	return prefixBOM(out, hadBOM), nil
}

func prefixBOM(doc []byte, hadBOM bool) []byte {
	if hadBOM {
		return append(append([]byte{}, bom...), doc...)
	}
	return doc
}
