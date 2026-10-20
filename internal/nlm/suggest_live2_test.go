package nlm

import (
	"encoding/json/v2"
	"testing"
)

// TestPromptSuggestLive pins the otmP3b envelope to live capture #2
// (2026-10-20): [HDR, nb, [[sid]..], mode] without a query, and
// [.., mode, null, query] with one. The capture came from the flashcards
// dialog (mode 9, MOOD's flashcards mode, with the flashcards topic as
// query); the chat surface sends its own mode 4 in the same envelope.
func TestPromptSuggestLive(t *testing.T) {
	ids := []string{
		"00000000-0000-4000-8000-000000000004", "00000000-0000-4000-8000-000000000005",
		"00000000-0000-4000-8000-000000000012", "00000000-0000-4000-8000-000000000001",
		"00000000-0000-4000-8000-000000000017", "00000000-0000-4000-8000-000000000015",
		"00000000-0000-4000-8000-000000000006", "00000000-0000-4000-8000-000000000007",
	}
	for fixture, query := range map[string]string{"live_2026-10-20_01": "", "live_2026-10-20_02": "text-92"} {
		fx := loadFixture(t, "otmP3b", fixture)
		var want []any
		if err := json.Unmarshal(fx.Request, &want); err != nil {
			t.Fatal(err)
		}
		if want[3] != float64(9) {
			t.Fatalf("%s: captured mode %v", fixture, want[3])
		}
		want[3] = chatSuggestMode
		wb, _ := json.Marshal(want)
		got, _ := json.Marshal(chatSuggestParams(fakeNB2, ids, query))
		if !sameJSON(t, got, wb) {
			t.Errorf("%s:\n got  %s\n want %s", fixture, got, wb)
		}
		ss, err := New(newFake(fx)).Suggest(t.Context(), fakeNB2, "chat", ids, query)
		if err != nil || len(ss) != 3 || ss[0].Title == "" || ss[0].Prompt == "" || ss[0].Title == ss[0].Prompt {
			t.Fatalf("%s: suggestions = %+v %v", fixture, ss, err)
		}
	}
}
