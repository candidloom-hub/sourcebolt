package session

import "testing"

func TestValidName(t *testing.T) {
	for _, ok := range []string{"default", "work", "a.b", "team-2", "Console", "nullable", "com10", "LPT"} {
		if err := validName(ok); err != nil {
			t.Errorf("validName(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{
		"", ".hidden", "a/b", `a\b`, "a:b", "a*", "a?", `a"`, "a<", "a>", "a|",
		"CON", "con", "Prn", "AUX", "NUL", "nul.txt", "COM1", "com9", "LPT1", "lpt9.x", "COM¹",
		"trail.", "trail ", " lead", "tab\tname", "nl\nname", "bell\x07", "del\x7f",
	} {
		if err := validName(bad); err == nil {
			t.Errorf("validName(%q) = nil, want error", bad)
		}
	}
}
