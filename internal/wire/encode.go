// Package wire encodes and decodes NotebookLM's internal web RPC formats:
// batchexecute requests, chunked responses and the streamed chat envelope.
package wire

import (
	"encoding/json/v2"
	"net/url"
	"strings"
)

// BatchItem is one RPC call inside a batchexecute request.
type BatchItem struct {
	ID     string // wire RPC id, e.g. "wXbhsf"
	Params any    // positional payload, JSON-encodable
}

// EncodeBatch builds the form body f.req=...&at=...& for batchexecute.
func EncodeBatch(calls []BatchItem, at string) (string, error) {
	rows := make([]any, 0, len(calls))
	for _, c := range calls {
		inner, err := json.Marshal(c.Params)
		if err != nil {
			return "", err
		}
		rows = append(rows, []any{c.ID, string(inner), nil, "generic"})
	}
	outer, err := json.Marshal([]any{rows})
	if err != nil {
		return "", err
	}
	return formBody(outer, at), nil
}

// EncodeStream builds the form body for GenerateFreeFormStreamed:
// f.req=[null,"<params JSON>"]&at=...&.
func EncodeStream(params any, at string) (string, error) {
	inner, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	outer, err := json.Marshal([]any{nil, string(inner)})
	if err != nil {
		return "", err
	}
	return formBody(outer, at), nil
}

func formBody(freq []byte, at string) string {
	return "f.req=" + escape(string(freq)) + "&at=" + escape(at) + "&"
}

// escape matches JavaScript encodeURIComponent for spaces (%20, not +).
func escape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
