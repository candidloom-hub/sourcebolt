package mcpserver

import (
	"context"
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const srcRow = `[["new1"],"Page",[null,null,null,null,null,null,null,["https://93.184.216.34/page"]]]`

func decode[T any](t *testing.T, res *mcp.CallToolResult) T {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool error: %s", text(res))
	}
	var out T
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil {
		t.Fatalf("decode %q: %v", text(res), err)
	}
	return out
}

func TestSourceAddHappy(t *testing.T) {
	st := newStub()
	st.payloads["izAoDd"] = `[[` + srcRow + `]]`
	cs := connect(t, st, "default")
	res := call(t, cs, "source_add", map[string]any{"notebook": idA, "items": []any{map[string]any{"url": "https://93.184.216.34/page"}}})
	out := decode[SourceAddOut](t, res)
	if out.Requested != 1 || out.Succeeded != 1 || out.Results[0].SourceID != "new1" || out.Results[0].Status != "added" {
		t.Fatalf("out = %+v", out)
	}
	if st.count("izAoDd") != 1 {
		t.Fatalf("izAoDd calls = %d", st.count("izAoDd"))
	}
}

func TestSourceAddRejectsPrivateURLWithoutCalls(t *testing.T) {
	st := newStub()
	cs := connect(t, st, "default")
	res := call(t, cs, "source_add", map[string]any{"notebook": idA, "items": []any{
		map[string]any{"url": "https://93.184.216.34/ok"},
		map[string]any{"url": "http://127.0.0.1/admin"},
	}})
	if e := envelope(t, res); e.Code != "INVALID_ARGUMENT" {
		t.Fatalf("envelope = %+v", e)
	}
	if len(st.calls) != 0 {
		t.Fatalf("domain calls = %d, want 0", len(st.calls))
	}
}

func TestSourceAddDriveAndFileUnsupported(t *testing.T) {
	cs := connect(t, newStub(), "default")
	for _, item := range []map[string]any{
		{"drive": map[string]any{"document_id": "d1", "mime": "application/pdf"}},
		{"file": map[string]any{"path": "x.pdf"}},
	} {
		res := call(t, cs, "source_add", map[string]any{"notebook": idA, "items": []any{item}})
		if e := envelope(t, res); e.Code != "UNSUPPORTED_BY_TRANSPORT" {
			t.Fatalf("envelope = %+v", e)
		}
	}
}

func TestSourceAddNeedsExactlyOneField(t *testing.T) {
	cs := connect(t, newStub(), "default")
	res := call(t, cs, "source_add", map[string]any{"notebook": idA, "items": []any{map[string]any{}}})
	if e := envelope(t, res); e.Code != "INVALID_ARGUMENT" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestSourceListFilter(t *testing.T) {
	cs := connect(t, newStub(), "default")
	all := decode[SourceListOut](t, call(t, cs, "source_list", map[string]any{"notebook": idA}))
	if all.Total != 1 || all.Sources[0].StatusName != "ready" || all.Sources[0].ID != "s1" {
		t.Fatalf("all = %+v", all)
	}
	none := decode[SourceListOut](t, call(t, cs, "source_list", map[string]any{"notebook": idA, "status": "error"}))
	if none.Total != 0 {
		t.Fatalf("none = %+v", none)
	}
}

func TestSourceReadPaging(t *testing.T) {
	st := newStub()
	st.payloads["hizoJc"] = `[[["s1"],"t"],null,null,null,[null,"0123456789abcdefghij"]]`
	cs := connect(t, st, "default")
	out := decode[SourceReadOut](t, call(t, cs, "source_read", map[string]any{"notebook": idA, "source": "Doc", "max_chars": 8}))
	if out.Text != "01234567" || !out.Truncated || out.NextOffset != 8 || out.TotalChars != 20 || out.SourceID != "s1" {
		t.Fatalf("out = %+v", out)
	}
	out = decode[SourceReadOut](t, call(t, cs, "source_read", map[string]any{"notebook": idA, "source": "s1", "offset": 8, "max_chars": 100}))
	if out.Text != "89abcdefghij" || out.Truncated || out.NextOffset != 0 {
		t.Fatalf("out = %+v", out)
	}
}

func TestSourceReadGuide(t *testing.T) {
	st := newStub()
	st.payloads["tr032e"] = `[[[[["s1"]],["Summary text"],[["k1","k2"]],[]]]]`
	cs := connect(t, st, "default")
	out := decode[SourceReadOut](t, call(t, cs, "source_read", map[string]any{"notebook": idA, "source": "Doc", "mode": "guide"}))
	if out.SourceID != "s1" || out.Text != "Summary text" || out.TotalChars != 12 || strings.Join(out.Keywords, ",") != "k1,k2" {
		t.Fatalf("out = %+v", out)
	}
	if st.count("tr032e") != 1 || st.count("hizoJc") != 0 {
		t.Fatalf("calls: guide %d, load %d", st.count("tr032e"), st.count("hizoJc"))
	}
}

func TestSourceUpdateRenameAndFresh(t *testing.T) {
	st := newStub()
	st.payloads["b7Wfje"] = `[[["s1"],"New"]]`
	st.payloads["yR9Yof"] = `[]`
	cs := connect(t, st, "default")
	out := decode[SourceUpdateOut](t, call(t, cs, "source_update", map[string]any{"notebook": idA, "source": "Doc", "title": "New", "check_freshness": true}))
	if !out.Renamed || out.Fresh == nil || !*out.Fresh {
		t.Fatalf("out = %+v", out)
	}
	if st.count("b7Wfje") != 1 {
		t.Fatalf("rename calls = %d", st.count("b7Wfje"))
	}
}

func TestSourceUpdateRefreshUnsupported(t *testing.T) {
	cs := connect(t, newStub(), "default")
	res := call(t, cs, "source_update", map[string]any{"notebook": idA, "source": "s1", "refresh": true})
	if e := envelope(t, res); e.Code != "UNSUPPORTED_BY_TRANSPORT" {
		t.Fatalf("envelope = %+v", e)
	}
}

func TestSourceToolAnnotations(t *testing.T) {
	cs := connect(t, newStub(), "default")
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*mcp.ToolAnnotations{}
	for _, tl := range res.Tools {
		got[tl.Name] = tl.Annotations
	}
	for _, n := range []string{"source_list", "source_read", "research_status"} {
		if a := got[n]; a == nil || !a.ReadOnlyHint || !a.IdempotentHint {
			t.Errorf("%s = %+v", n, a)
		}
	}
	for n, idem := range map[string]bool{"source_add": false, "source_update": true, "research_start": false, "research_import": false} {
		a := got[n]
		if a == nil || a.ReadOnlyHint || a.DestructiveHint == nil || *a.DestructiveHint || a.IdempotentHint != idem {
			t.Errorf("%s = %+v", n, a)
		}
	}
	if !strings.Contains(got["source_add"].Title, "Add") {
		t.Errorf("title = %q", got["source_add"].Title)
	}
}

// Adding a source changes the notebook's source count, so the cached
// notebook list must be refreshed on the next read.
func TestSourceAddInvalidatesNotebookCache(t *testing.T) {
	st := newStub()
	st.payloads["izAoDd"] = `[[` + srcRow + `]]`
	cs := connect(t, st, "default")
	call(t, cs, "notebook_list", nil)
	call(t, cs, "source_add", map[string]any{"notebook": idA, "items": []any{map[string]any{"url": "https://93.184.216.34/page"}}})
	call(t, cs, "notebook_list", nil)
	if n := st.count("wXbhsf"); n != 2 {
		t.Fatalf("notebook list fetched %d times, want 2 (cache invalidated by source_add)", n)
	}
}

func paramsJSON(t *testing.T, st *stub, id string) string {
	t.Helper()
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, c := range st.calls {
		if c.ID == id {
			b, err := json.Marshal(c.Params)
			if err != nil {
				t.Fatal(err)
			}
			return string(b)
		}
	}
	t.Fatalf("no call to %s", id)
	return ""
}

func TestSourceAddSendsValidatedNormalizedURL(t *testing.T) {
	st := newStub()
	st.payloads["izAoDd"] = `[[` + srcRow + `]]`
	cs := connect(t, st, "default")
	res := call(t, cs, "source_add", map[string]any{"notebook": idA, "items": []any{map[string]any{"url": "  HTTPS://93.184.216.34/Page  "}}})
	decode[SourceAddOut](t, res)
	got := paramsJSON(t, st, "izAoDd")
	if !strings.Contains(got, `"https://93.184.216.34/Page"`) || strings.Contains(got, "HTTPS") || strings.Contains(got, "  ") {
		t.Fatalf("params = %s", got)
	}
}

func TestNotebookCreateSendsValidatedNormalizedURL(t *testing.T) {
	st := moreStub()
	st.payloads["izAoDd"] = `[[` + srcRow + `]]`
	cs := connect(t, st, "default")
	decode[NotebookCreateOut](t, call(t, cs, "notebook_create", map[string]any{
		"title": "T", "sources": []any{map[string]any{"url": "  HTTPS://93.184.216.34/Page  "}},
	}))
	got := paramsJSON(t, st, "izAoDd")
	if !strings.Contains(got, `"https://93.184.216.34/Page"`) || strings.Contains(got, "HTTPS") || strings.Contains(got, "  ") {
		t.Fatalf("params = %s", got)
	}
}

func TestNotebookCreateAndSourceAddRejectSameInputs(t *testing.T) {
	cases := map[string][]any{
		"empty":     {map[string]any{}},
		"two":       {map[string]any{"url": "https://93.184.216.34/x", "text": map[string]any{"title": "t", "content": "a"}}},
		"private":   {map[string]any{"url": "http://127.0.0.1/x"}},
		"scheme":    {map[string]any{"url": "ftp://93.184.216.34/x"}},
		"blankText": {map[string]any{"text": map[string]any{"title": "t", "content": " "}}},
		"drive":     {map[string]any{"drive": map[string]any{"document_id": "d"}}},
		"file":      {map[string]any{"file": map[string]any{"path": "x.pdf"}}},
		"second":    {map[string]any{"url": "https://93.184.216.34/ok"}, map[string]any{"url": "http://10.0.0.1/x"}},
	}
	for name, items := range cases {
		t.Run(name, func(t *testing.T) {
			cs := connect(t, moreStub(), "default")
			a := envelope(t, call(t, cs, "source_add", map[string]any{"notebook": idA, "items": items}))
			b := envelope(t, call(t, cs, "notebook_create", map[string]any{"title": "T", "sources": items}))
			if a.Code != b.Code || a.Error != b.Error {
				t.Fatalf("source_add = %q %q; notebook_create = %q %q", a.Code, a.Error, b.Code, b.Error)
			}
		})
	}
}
