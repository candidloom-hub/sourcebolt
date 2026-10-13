package resolve

import (
	"errors"
	"testing"
)

var items = []Item{
	{ID: "0b1c2d3e-0000-4000-8000-000000000001", Title: "Q3 plan"},
	{ID: "0b1c2d3e-0000-4000-8000-000000000002", Title: "Q3 planning notes"},
	{ID: "7f00aa11-0000-4000-8000-000000000003", Title: "Research: LLM evals"},
}

func TestExactID(t *testing.T) {
	got, err := Resolve("7f00aa11-0000-4000-8000-000000000003", items)
	if err != nil || got.Title != "Research: LLM evals" {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestUniquePrefix(t *testing.T) {
	got, err := Resolve("7f00aa", items)
	if err != nil || got.ID != items[2].ID {
		t.Fatalf("got %+v %v", got, err)
	}
	if _, err := Resolve("7f00a", items); err == nil {
		t.Fatal("prefix shorter than 6 must not match by id")
	}
}

func TestExactTitleCaseInsensitive(t *testing.T) {
	got, err := Resolve("q3 PLAN", items)
	if err != nil || got.ID != items[0].ID {
		t.Fatalf("exact title must win over substring: %+v %v", got, err)
	}
}

func TestUniqueSubstring(t *testing.T) {
	got, err := Resolve("evals", items)
	if err != nil || got.ID != items[2].ID {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestAmbiguous(t *testing.T) {
	_, err := Resolve("plan", items)
	var amb *ErrAmbiguous
	if !errors.As(err, &amb) || len(amb.Candidates) != 2 {
		t.Fatalf("got %v", err)
	}
}

func TestNotFoundSuggestions(t *testing.T) {
	_, err := Resolve("Q3 plam", items[2:])
	var nf *ErrNotFound
	if !errors.As(err, &nf) {
		t.Fatalf("got %v", err)
	}
	_, err = Resolve("Q4 plann", items)
	if !errors.As(err, &nf) || len(nf.Suggestions) == 0 || len(nf.Suggestions) > 3 || nf.Suggestions[0].ID != items[0].ID {
		t.Fatalf("suggestions = %+v", nf)
	}
}

func TestEmptyRef(t *testing.T) {
	var nf *ErrNotFound
	if _, err := Resolve("  ", items); !errors.As(err, &nf) {
		t.Fatalf("got %v", err)
	}
}
