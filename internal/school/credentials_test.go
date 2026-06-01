package school

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveCredentialsFallsBackToDotEnv(t *testing.T) {
	t.Setenv("PASSPORT_UNDERGRADUATE_USERNAME", "")
	t.Setenv("PASSPORT_GRADUATE_USERNAME", "")
	t.Setenv("PASSPORT_PASSWORD", "")
	t.Setenv("PASSPORT_TOTP", "")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PASSPORT_UNDERGRADUATE_USERNAME=test-user\nPASSPORT_PASSWORD=test-pass\nPASSPORT_TOTP=123456\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatal(err)
		}
	}()

	creds, err := ResolveCredentials("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if creds.Username != "test-user" || creds.Password != "test-pass" || creds.TOTP != "123456" {
		t.Fatalf("ResolveCredentials() = %+v", creds)
	}
}

func TestResolveGraduateCredentialsPreferGraduateUsername(t *testing.T) {
	t.Setenv("PASSPORT_GRADUATE_USERNAME", "graduate-specific")
	t.Setenv("PASSPORT_UNDERGRADUATE_USERNAME", "undergrad-alias")
	t.Setenv("PASSPORT_PASSWORD", "shared-pass")
	t.Setenv("PASSPORT_TOTP", "")

	creds, err := ResolveCredentialsForProgram(ProgramGraduate, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if creds.Username != "graduate-specific" || creds.Password != "shared-pass" {
		t.Fatalf("ResolveCredentialsForProgram() = %+v", creds)
	}
}

func TestDetectCredentialProgramsUsesProgramSpecificUsernames(t *testing.T) {
	chdirTemp(t)
	t.Setenv("PASSPORT_UNDERGRADUATE_USERNAME", "undergrad-user")
	t.Setenv("PASSPORT_GRADUATE_USERNAME", "graduate-user")

	programs := DetectCredentialPrograms()
	if len(programs) != 2 || programs[0] != ProgramUndergraduate || programs[1] != ProgramGraduate {
		t.Fatalf("DetectCredentialPrograms() = %v, want undergraduate and graduate", programs)
	}
}

func TestDetectCredentialProgramsReturnsNoneWithoutProgramUsernames(t *testing.T) {
	chdirTemp(t)
	t.Setenv("PASSPORT_UNDERGRADUATE_USERNAME", "")
	t.Setenv("PASSPORT_GRADUATE_USERNAME", "")

	if programs := DetectCredentialPrograms(); len(programs) != 0 {
		t.Fatalf("DetectCredentialPrograms() = %v, want none", programs)
	}
}

func chdirTemp(t *testing.T) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCurrentOTPFromOtpauthURL(t *testing.T) {
	code, err := CurrentOTP("otpauth://totp/LifeUSTC:test?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&algorithm=SHA1&digits=8&period=30", time.Unix(59, 0))
	if err != nil {
		t.Fatal(err)
	}
	if code != "94287082" {
		t.Fatalf("CurrentOTP() = %q, want %q", code, "94287082")
	}
}

func TestPickSemesterPrefersRequestedOrCurrent(t *testing.T) {
	semesters := []Semester{
		{ID: 1, SemesterCn: "Older"},
		{ID: 2, SemesterCn: "Current", IsLast: true},
	}

	current, err := PickSemester(semesters, 0)
	if err != nil {
		t.Fatal(err)
	}
	if current.ID != 2 {
		t.Fatalf("current semester ID = %d, want 2", current.ID)
	}

	selected, err := PickSemester(semesters, 1)
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID != 1 {
		t.Fatalf("selected semester ID = %d, want 1", selected.ID)
	}
}

func TestPickSemesterFallsBackToHighestID(t *testing.T) {
	semesters := []Semester{
		{ID: 381},
		{ID: 362},
		{ID: 221},
	}

	current, err := PickSemester(semesters, 0)
	if err != nil {
		t.Fatal(err)
	}
	if current.ID != 381 {
		t.Fatalf("current semester ID = %d, want 381", current.ID)
	}
}
