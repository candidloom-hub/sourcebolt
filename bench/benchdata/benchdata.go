// Package benchdata builds synthetic but realistic NotebookLM responses for
// the benchmarks. The row structure comes from the sanitized live wXbhsf
// capture (testdata/fixtures/wXbhsf/live_2026-10-13_01.json); rows are
// cloned with fresh ids until the requested size is reached. Nothing here is
// real account data.
package benchdata

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// Root returns the repository root, found relative to this source file.
func Root() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

var (
	once     sync.Once
	template []any // the live fixture's rows
)

func rows() []any {
	once.Do(func() {
		raw, err := os.ReadFile(filepath.Join(Root(), "testdata", "fixtures", "wXbhsf", "live_2026-10-13_01.json"))
		if err != nil {
			panic(err)
		}
		var fx struct {
			ResponseBody string `json:"response_body"`
		}
		if err := json.Unmarshal(raw, &fx); err != nil {
			panic(err)
		}
		// ")]}'" "" "<len>" "<chunk>..."
		lines := strings.SplitN(fx.ResponseBody, "\n", 5)
		if len(lines) < 4 {
			panic("benchdata: unexpected fixture framing")
		}
		var chunk []any
		if err := json.Unmarshal([]byte(lines[3]), &chunk); err != nil {
			panic(err)
		}
		frame := chunk[0].([]any) // ["wrb.fr","wXbhsf","<payload json>", ...]
		var payload []any
		if err := json.Unmarshal([]byte(frame[2].(string)), &payload); err != nil {
			panic(err)
		}
		template = payload[0].([]any)
	})
	return template
}

// ListPayload returns the inner wXbhsf payload ([[row, ...]]) with n rows.
func ListPayload(n int) []byte {
	src := rows()
	out := make([]any, 0, n)
	for i := range n {
		b, _ := json.Marshal(src[i%len(src)])
		var row []any
		_ = json.Unmarshal(b, &row)
		row[2] = fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
		if t, ok := row[0].(string); ok && t == "" {
			row[0] = "Notebook " + strconv.Itoa(i+1)
		}
		out = append(out, row)
	}
	b, err := json.Marshal([]any{out})
	if err != nil {
		panic(err)
	}
	return b
}

// Body wraps payload in a batchexecute response body the way the server
// frames it: XSSI prefix, chunk length, one wrb.fr row plus the usual
// metadata rows, then a trailer chunk.
func Body(rpc string, payload []byte) []byte {
	q, _ := json.Marshal(string(payload))
	chunk := `[["wrb.fr","` + rpc + `",` + string(q) + `,null,null,null,"generic"],["di",88],["af.httprm",88,"-55",4]]`
	trailer := `[["e",4,null,null,` + strconv.Itoa(len(chunk)) + `]]`
	return []byte(")]}'\n\n" + strconv.Itoa(len(chunk)) + "\n" + chunk + "\n" + strconv.Itoa(len(trailer)) + "\n" + trailer + "\n")
}

// ListBody is Body for a list response with n notebooks.
func ListBody(n int) []byte { return Body("wXbhsf", ListPayload(n)) }

// ListBodyOfSize returns a list response whose body is at least target
// bytes, using as few rows as possible, and the row count.
func ListBodyOfSize(target int) ([]byte, int) {
	for n := 1; ; n++ {
		if b := ListBody(n); len(b) >= target {
			return b, n
		}
	}
}
