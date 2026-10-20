package nlm

import (
	"context"
	"testing"
)

// Review item 10. The live cFji9 answers (2026-10-13, 2026-10-20) hold only
// notes: meta[0] 1 (text note) or 2 (saved chat answer), meta[3] null. The
// catalog says the list also carries note-backed mind maps, created with
// meta [2,null,null,5,[[sid]..]] (meta[3] = 5). Rows with a non-null
// meta[3], or a meta[0] other than 1 or 2, are not notes and are not listed.
func TestListNotesOnlyNotes(t *testing.T) {
	ft := newFake(inlineFixture("cFji9", `[[`+
		`["n1",["n1","body",[1,"o",[1700000000,0],null,null,[1700000000,0],true],null,"Text note"]],`+
		`["n2",["n2","answer",[2,"o",[1700000000,0],null,null,[1700000000,0],true],null,"Chat note"]],`+
		`["m1",["m1","{\"name\":\"root\"}",[2,null,null,5,[["s1"]]],null,"Mind map"]],`+
		`["a1",["a1","x",[4,"o",[1700000000,0]],null,"Studio row"]],`+
		`["n3",["n3","legacy",null,null,"No meta"]]`+
		`]]`))
	ns, err := New(ft).ListNotes(context.Background(), nb1)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, n := range ns {
		ids = append(ids, n.ID)
	}
	if len(ids) != 3 || ids[0] != "n1" || ids[1] != "n2" || ids[2] != "n3" {
		t.Fatalf("notes = %v, want n1 n2 n3 (no mind map, no studio row)", ids)
	}
}
