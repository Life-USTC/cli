package room

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/output"
)

// NewCmdRoom exposes room map lookups from the public catalog.
func NewCmdRoom() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "room [command]",
		Aliases: []string{"rooms"},
		Short:   "Look up campus room maps",
		Args:    cobra.NoArgs,
	}
	cmd.AddCommand(newCmdMap())
	return cmd
}

func newCmdMap() *cobra.Command {
	return &cobra.Command{
		Use:     "map <code>",
		Aliases: []string{"get", "show"},
		Short:   "Show the map for a room or building",
		Example: `  life-ustc catalog room map <room-code>
  life-ustc catalog room map <building-code> --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			code, err := normalizeRoomCode(args[0])
			if err != nil {
				return err
			}
			client, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}
			data, err := fetchRoomMap(client, code)
			if err != nil {
				return err
			}
			return output.OutputDetail(data, []output.FieldDef{
				{Key: "code", Label: "Code"},
				{Key: "status", Label: "Status"},
				{Key: "building", Label: "Building", SkipEmpty: true},
				{Key: "floor", Label: "Floor", SkipEmpty: true},
				{Key: "imageUrl", Label: "Map image", SkipEmpty: true},
				{Key: "sourceImageUrl", Label: "Source image", SkipEmpty: true},
			}, "Room map")
		},
	}
}

func fetchRoomMap(client *api.TypedClient, code string) (any, error) {
	return api.ParseResponseRaw(client.CatalogRoomsMap(api.Ctx(), code))
}

func normalizeRoomCode(value string) (string, error) {
	code := strings.TrimSpace(value)
	if code == "" {
		return "", fmt.Errorf("room code must not be empty")
	}
	return code, nil
}
