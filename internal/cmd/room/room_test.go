package room

import (
	"github.com/Life-USTC/CLI/internal/specification"
	"testing"
)

func TestSpecNormalizeRoomCodeTrimsAndRejectsEmpty(t *testing.T) {
	t.Run("room-map.cli-input", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
