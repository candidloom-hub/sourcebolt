package nlm

import (
	"encoding/json/v2"
	"testing"
)

// The 8 ids the flashcards dialog sent to otmP3b in live capture #2:
// sources and notes alike, each as [id].
var suggestIDs2 = []string{
	"00000000-0000-4000-8000-000000000004", "00000000-0000-4000-8000-000000000005",
	"00000000-0000-4000-8000-000000000012", "00000000-0000-4000-8000-000000000001",
	"00000000-0000-4000-8000-000000000017", "00000000-0000-4000-8000-000000000015",
	"00000000-0000-4000-8000-000000000006", "00000000-0000-4000-8000-000000000007",
}

// TestPromptSuggestMode9Live compares the mode 9 (flashcards dialog)
// request with the live capture #2 requests, unmodified:
// [HDR, nb, [[id]..], 9] without a query, [.., 9, null, query] with one.
func TestPromptSuggestMode9Live(t *testing.T) {
	for fixture, query := range map[string]string{"live_2026-10-20_01": "", "live_2026-10-20_02": "text-92"} {
		fx := loadFixture(t, "otmP3b", fixture)
		got, _ := json.Marshal(promptSuggestParams(fakeNB2, suggestIDs2, flashcardsSuggestMode, query))
		if !sameJSON(t, got, fx.Request) {
			t.Errorf("%s:\n got  %s\n want %s", fixture, got, fx.Request)
		}
		// The answer decodes like the chat surface's: [[title, prompt]..].
		ss, err := New(newFake(fx)).Suggest(t.Context(), fakeNB2, "chat", suggestIDs2, query)
		if err != nil || len(ss) != 3 || ss[0].Title == "" || ss[0].Prompt == "" || ss[0].Title == ss[0].Prompt {
			t.Fatalf("%s: suggestions = %+v %v", fixture, ss, err)
		}
	}
}

// Mode 4 (chat) was not captured: its request stays the catalog's
// 6-element envelope, query slot included.
func TestPromptSuggestMode4Unchanged(t *testing.T) {
	got, _ := json.Marshal(promptSuggestParams(fakeNB2, []string{"s1"}, chatSuggestMode, ""))
	want := `[[2,null,[1],[1,null,null,null,null,null,null,null,null,null,[1,3]]],"` + fakeNB2 + `",[["s1"]],4,null,null]`
	if !sameJSON(t, got, []byte(want)) {
		t.Fatalf("got %s want %s", got, want)
	}
}
