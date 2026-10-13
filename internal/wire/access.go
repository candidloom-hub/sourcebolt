package wire

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
)

var lenient = jsontext.AllowInvalidUTF8(true)

func isNull(v jsontext.Value) bool {
	t := bytes.TrimSpace(v)
	return len(t) == 0 || string(t) == "null"
}

// elems splits a JSON array into its raw elements without decoding them.
func elems(v jsontext.Value) ([]jsontext.Value, bool) {
	t := bytes.TrimSpace(v)
	if len(t) == 0 || t[0] != '[' {
		return nil, false
	}
	var out []jsontext.Value
	if err := json.Unmarshal(t, &out, lenient); err != nil {
		return nil, false
	}
	return out, true
}

// At walks positional indexes into nested arrays. ok is false when any
// step is out of range, null or not an array, or the value is null.
func At(v jsontext.Value, path ...int) (jsontext.Value, bool) {
	cur := v
	for _, i := range path {
		es, ok := elems(cur)
		if !ok || i < 0 || i >= len(es) {
			return nil, false
		}
		cur = es[i]
	}
	if isNull(cur) {
		return nil, false
	}
	return cur, true
}

// Str returns the string at path, or "" when absent or not a string.
func Str(v jsontext.Value, path ...int) string {
	x, ok := At(v, path...)
	if !ok {
		return ""
	}
	var s string
	if json.Unmarshal(x, &s, lenient) != nil {
		return ""
	}
	return s
}

// Int returns the integer at path.
func Int(v jsontext.Value, path ...int) (int64, bool) {
	x, ok := At(v, path...)
	if !ok {
		return 0, false
	}
	var f float64
	if json.Unmarshal(x, &f) != nil {
		return 0, false
	}
	return int64(f), true
}
