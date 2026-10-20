package mcpserver

import (
	"encoding/json"
	"testing"

	"github.com/candidloom-hub/sourcebolt/bench/benchdata"
)

// notebook_list returns 20 notebooks by default (B3: every listed notebook
// is encoded twice and decoded by the client) and reports the full total,
// so callers page with offset or raise limit.
func TestNotebookListDefaultPage(t *testing.T) {
	cs := connectWith(t, benchTransport{payload: benchdata.ListPayload(30)})
	var out NotebookListOut
	page := func(args map[string]any) {
		t.Helper()
		out = NotebookListOut{}
		if err := json.Unmarshal([]byte(text(call(t, cs, "notebook_list", args))), &out); err != nil {
			t.Fatal(err)
		}
	}
	page(nil)
	if len(out.Notebooks) != 20 || out.Total != 30 || !out.HasMore {
		t.Fatalf("default page: %d notebooks, total %d, has_more %v; want 20 of 30, more", len(out.Notebooks), out.Total, out.HasMore)
	}
	page(map[string]any{"offset": 20})
	if len(out.Notebooks) != 10 || out.Total != 30 || out.HasMore || out.Notebooks[0].ID != "00000000-0000-4000-8000-000000000021" {
		t.Fatalf("second page: %d notebooks, total %d, has_more %v, first %+v", len(out.Notebooks), out.Total, out.HasMore, out.Notebooks)
	}
	page(map[string]any{"limit": 50})
	if len(out.Notebooks) != 30 || out.HasMore {
		t.Fatalf("limit 50: %d notebooks, has_more %v; want 30, none more", len(out.Notebooks), out.HasMore)
	}
	page(map[string]any{"offset": 40})
	if len(out.Notebooks) != 0 || out.HasMore {
		t.Fatalf("past the end: %d notebooks, has_more %v", len(out.Notebooks), out.HasMore)
	}
}
