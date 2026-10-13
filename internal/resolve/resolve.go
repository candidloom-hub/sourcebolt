// Package resolve lets every tool accept an id, a unique id prefix or a
// title (exact or unique substring) for notebooks, sources and artifacts.
package resolve

import (
	"fmt"
	"slices"
	"strings"
)

const minPrefix = 6

type Item struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type ErrNotFound struct {
	Ref         string
	Suggestions []Item
}

func (e *ErrNotFound) Error() string { return fmt.Sprintf("no match for %q", e.Ref) }

type ErrAmbiguous struct {
	Ref        string
	Candidates []Item
}

func (e *ErrAmbiguous) Error() string {
	return fmt.Sprintf("%q matches %d items; use an id", e.Ref, len(e.Candidates))
}

// Resolve picks one item. Order: exact id, unique id prefix (≥ 6 chars),
// exact title (case-insensitive), unique title substring.
func Resolve(ref string, items []Item) (Item, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return Item{}, &ErrNotFound{Ref: ref}
	}
	for _, it := range items {
		if it.ID == ref {
			return it, nil
		}
	}
	lower := strings.ToLower(ref)
	steps := []func(Item) bool{
		func(it Item) bool { return len(ref) >= minPrefix && strings.HasPrefix(strings.ToLower(it.ID), lower) },
		func(it Item) bool { return strings.EqualFold(it.Title, ref) },
		func(it Item) bool { return strings.Contains(strings.ToLower(it.Title), lower) },
	}
	for _, match := range steps {
		var hits []Item
		for _, it := range items {
			if match(it) {
				hits = append(hits, it)
			}
		}
		switch len(hits) {
		case 0:
			continue
		case 1:
			return hits[0], nil
		default:
			return Item{}, &ErrAmbiguous{Ref: ref, Candidates: hits}
		}
	}
	return Item{}, &ErrNotFound{Ref: ref, Suggestions: suggest(lower, items)}
}

func suggest(ref string, items []Item) []Item {
	type scored struct {
		it Item
		d  int
	}
	var s []scored
	for _, it := range items {
		s = append(s, scored{it, levenshtein(ref, strings.ToLower(it.Title))})
	}
	slices.SortStableFunc(s, func(a, b scored) int { return a.d - b.d })
	out := []Item{}
	for i := 0; i < len(s) && i < 3; i++ {
		if s[i].d <= max(len(ref), 3) {
			out = append(out, s[i].it)
		}
	}
	return out
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
