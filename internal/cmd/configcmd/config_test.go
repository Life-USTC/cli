package configcmd

import "testing"

func TestParseSchoolPrograms(t *testing.T) {
	t.Parallel()

	got, err := parseSchoolPrograms("undergrad,graduate,graduate")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "undergraduate" || got[1] != "graduate" {
		t.Fatalf("parseSchoolPrograms() = %v, want undergraduate, graduate", got)
	}
}

func TestParseSchoolProgramsRejectsInvalid(t *testing.T) {
	t.Parallel()

	if _, err := parseSchoolPrograms("phd"); err == nil {
		t.Fatal("parseSchoolPrograms() expected an error")
	}
}
