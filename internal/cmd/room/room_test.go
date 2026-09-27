package room

import "testing"

func TestSpecNormalizeRoomCodeTrimsAndRejectsEmpty(t *testing.T) {
	t.Run("room-map.cli-input", func(t *testing.T) {
		if got, err := normalizeRoomCode("  西区  "); err != nil || got != "西区" {
			t.Fatalf("normalizeRoomCode = %q, %v", got, err)
		}
		if _, err := normalizeRoomCode("   "); err == nil {
			t.Fatal("normalizeRoomCode accepted empty code")
		}

	})
}
