package nlm

import (
	"context"
	"encoding/json/v2"
	"fmt"

	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// Saving a chat answer as a note (CYK0Xb), pinned by live captures #2 and
// #3 (2026-10-20). The web app re-sends the stream's final answer:
//
//	[nb, answer_text, [2,null×5,1], passages, title, block, HDR]
//
// block is the stream's type block [rich, null, null, [[pid], passage]..,
// flag] and passages is the list of its passage bodies alone. Both are
// re-encoded from the stream's compact form into the web app's dense one:
//
//   - every run's format [bold?, italic?, ?, url, …, code?, ?] becomes nine
//     slots with the five flags as 0/1 (absent or [] is all 0);
//   - a paragraph without a style gets the default [null, 1];
//   - a paragraph's list meta [null, null, n, {ext}] becomes 25 slots with
//     the extension object dropped;
//   - a raw passage block's text [text, ""] drops the trailing "";
//   - each passage's score ([2]) becomes null;
//   - the thinking steps a block can carry at [8] get the paragraph rule.
//
// Anything else is sent as the stream had it. Both captured saves are
// reproduced exactly from the decoded streams (notes_answer_test.go); rich
// structures that neither capture had (tables, images) are sent unchanged
// and may not match what the web app would send.

// answerKind is the note kind of a saved chat answer (a text note is 1).
const answerKind = 2

// answerNoteParams builds the CYK0Xb request for a streamed answer.
func answerNoteParams(nb string, a Answer, title string) ([]any, error) {
	if len(a.block) == 0 {
		return nil, &transport.Error{Kind: transport.KindUnsupported, RPC: "notes.create",
			Message: "this answer cannot be saved as a note: the stream sent no final answer block (only a complete streamed answer can be saved)"}
	}
	var tb []any
	if err := json.Unmarshal(a.block, &tb); err != nil || len(tb) < 4 {
		return nil, &transport.Error{Kind: transport.KindDrift, RPC: "notes.create", Message: fmt.Sprintf("answer block has an unexpected shape: %v", err)}
	}
	block := clone(tb)
	if rich, ok := block[0].([]any); ok && len(rich) > 0 {
		rich[0] = mapList(rich[0], noteBlock)
	}
	var passages []any
	if ps, ok := block[3].([]any); ok {
		for _, p := range ps {
			pair, ok := p.([]any)
			if !ok || len(pair) < 2 {
				continue
			}
			body := notePassage(pair[1])
			pair[1] = body
			passages = append(passages, body)
		}
	}
	kind := []any{answerKind, nil, nil, nil, nil, nil, 1}
	return []any{nb, a.Text, kind, passages, title, block, requestOptions()}, nil
}

// clone deep-copies decoded JSON so the transform never edits the input.
func clone(v []any) []any {
	out := make([]any, len(v))
	for i, x := range v {
		if l, ok := x.([]any); ok {
			out[i] = clone(l)
		} else {
			out[i] = x
		}
	}
	return out
}

func mapList(v any, f func(any) any) any {
	l, ok := v.([]any)
	if !ok {
		return v
	}
	for i, x := range l {
		l[i] = f(x)
	}
	return l
}

// notePassage re-encodes one passage body [null, null, score, ranges,
// [blocks], sources, [pid]].
func notePassage(v any) any {
	p, ok := v.([]any)
	if !ok {
		return v
	}
	if len(p) > 2 {
		p[2] = nil
	}
	if len(p) > 4 {
		if c, ok := p[4].([]any); ok && len(c) > 0 {
			c[0] = mapList(c[0], noteBlock)
		}
	}
	return p
}

// noteBlock re-encodes one block: a paragraph [start, end, content, ...],
// a raw block [start, end, null×4, [text, ""]] or a thinking block
// [null×8, [steps]].
func noteBlock(v any) any {
	b, ok := v.([]any)
	if !ok {
		return v
	}
	if len(b) > 2 {
		if c, ok := b[2].([]any); ok {
			b[2] = noteContent(c)
		}
	}
	if len(b) > 6 {
		b[6] = trimEmpty(b[6])
	}
	if len(b) > 8 {
		if th, ok := b[8].([]any); ok && len(th) > 0 {
			th[0] = mapList(th[0], func(s any) any {
				st, ok := s.([]any)
				if ok && len(st) >= 3 && st[0] == nil && st[1] == nil {
					if c, ok := st[2].([]any); ok {
						st[2] = noteContent(c)
					}
				}
				return s
			})
		}
	}
	return b
}

// noteContent re-encodes a paragraph [runs, style, null, meta].
func noteContent(c []any) []any {
	if len(c) > 0 {
		c[0] = mapList(c[0], noteRun)
	}
	if len(c) < 2 {
		c = append(c, []any{nil, 1})
	}
	if len(c) > 3 {
		if m, ok := c[3].([]any); ok {
			c[3] = noteMeta(m)
		}
	}
	return c
}

// noteRun re-encodes a run [start, end, [text, format]] (a raw run keeps
// its text at [6]).
func noteRun(v any) any {
	r, ok := v.([]any)
	if !ok {
		return v
	}
	if len(r) > 2 {
		if t, ok := r[2].([]any); ok && len(t) > 0 {
			var f any
			if len(t) > 1 {
				f = t[1]
			}
			nt := append([]any{t[0], noteFormat(f)}, t[min(len(t), 2):]...)
			r[2] = nt
		}
	}
	if len(r) > 6 {
		r[6] = trimEmpty(r[6])
	}
	return r
}

// noteFormat is the dense nine-slot run format.
func noteFormat(v any) []any {
	f, _ := v.([]any)
	at := func(i int) any {
		if i < len(f) {
			return f[i]
		}
		return nil
	}
	flag := func(i int) any {
		switch x := at(i).(type) {
		case bool:
			if x {
				return 1
			}
		case float64:
			if x == 1 {
				return 1
			}
		}
		return 0
	}
	return []any{flag(0), flag(1), flag(2), at(3), at(4), at(5), at(6), flag(7), flag(8)}
}

// noteMeta is the dense 25-slot paragraph meta without extension objects.
func noteMeta(m []any) []any {
	out := make([]any, max(len(m), 25))
	for i, x := range m {
		if _, isObj := x.(map[string]any); !isObj {
			out[i] = x
		}
	}
	return out
}

func trimEmpty(v any) any {
	l, ok := v.([]any)
	if !ok {
		return v
	}
	for len(l) > 0 && l[len(l)-1] == "" {
		l = l[:len(l)-1]
	}
	return l
}

// SaveAnswerNote saves a streamed chat answer as a note (CYK0Xb, never
// retried), as the web app's "Save to note" does: kind 2 with the answer's
// passages and rich block. Only an answer from a complete stream can be
// saved (see answerNoteParams). The server sets its own title and returns
// the note row [[id, content, meta, passages, title, rich]].
func (c *Client) SaveAnswerNote(ctx context.Context, nb string, a Answer, title string) (Note, error) {
	params, err := answerNoteParams(nb, a, title)
	if err != nil {
		return Note{}, err
	}
	p, err := c.do(ctx, "notes.create", params, callOpts{sourcePath: notebookPath(nb), null: wire.NullAllowed})
	if err != nil {
		return Note{}, err
	}
	row, ok := wire.At(p, 0)
	if !ok || wire.Str(row, 0) == "" {
		return Note{}, &transport.Error{Kind: transport.KindDrift, RPC: "notes.create", Message: "create answered without a note id"}
	}
	return Note{ID: wire.Str(row, 0), Content: wire.Str(row, 1), Title: wire.Str(row, 4), Updated: stamp(row, 2, 2)}, nil
}
