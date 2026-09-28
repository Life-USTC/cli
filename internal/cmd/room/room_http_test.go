package room

import (
	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/specification"
	"testing"
)

func TestSpecFetchRoomMapUsesRoomCodePath(t *testing.T) {
	t.Run("room-map.cli-request", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
func acceptanceAdapter(t *testing.T, in specification.Input) specification.Observation {
	if in.Action == "normalize" {
		value, err := normalizeRoomCode(in.Value)
		return specification.Observation{Output: value, Err: err}
	}
	client, err := api.NewTypedClient(in.ServerURL, false)
	if err != nil {
		return specification.Observation{Err: err}
	}
	data, err := fetchRoomMap(client, "B001")
	return specification.Observation{Data: data, Err: err}
}
