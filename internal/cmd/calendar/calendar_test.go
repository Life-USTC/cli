package calendar

import (
	"strings"
	"testing"
)

func TestSetRequiresSemesterID(t *testing.T) {
	for _, args := range [][]string{
		{"999999"},
		{"999999", "--semester-id", " "},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
			t.Setenv("LIFE_USTC_SERVER", "http://127.0.0.1:1")

			cmd := newCmdSet()
			cmd.SetArgs(args)

			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected calendar set without --semester-id to fail")
			}
			if !strings.Contains(err.Error(), "--semester-id is required") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
