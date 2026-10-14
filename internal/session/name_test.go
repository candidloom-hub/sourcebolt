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

// A profile saved before the stricter rule (e.g. "work ") must stay
// loadable and deletable; only new saves are refused.
func TestLegacyNameLoadsAndDeletes(t *testing.T) {
	s, dir := newTestStore(t)
	legacy := "work "
	if err := s.Save(&Profile{Name: legacy, Jar: NewJar()}); err == nil {
		t.Fatal("Save accepted a trailing-space name")
	}
	if err := s.(*fileStore).save(&Profile{Name: legacy, Jar: NewJar()}); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Load(legacy)
	if err != nil {
		t.Fatalf("Load legacy: %v", err)
	}
	if err := s.Save(loaded); err != nil {
		t.Fatalf("Save over existing legacy profile (cookie rotation): %v", err)
	}
	if err := s.Delete(legacy); err != nil {
		t.Fatalf("Delete legacy: %v", err)
	}
	if _, err := s.Load(legacy); err == nil {
		t.Fatalf("legacy profile still present in %s", dir)
	}
	if _, err := s.Load("../x"); err == nil {
		t.Fatal("Load accepted a path")
	}
}
