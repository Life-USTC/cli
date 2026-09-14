package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
)

func newCmdClient() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client <command>",
		Short: "Inspect the current OAuth client",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newCmdClientActivity())
	return cmd
}

func newCmdClientActivity() *cobra.Command {
	var cursor string
	var limit int
	cmd := &cobra.Command{
		Use:   "activity",
		Short: "List activity performed by this client",
		Long:  "List activity performed by the current OAuth client for the current user.",
		Example: `  # Show the latest client activity
  life-ustc account client activity

  # Request more entries or continue from a cursor
  life-ustc account client activity --limit 20 --cursor <next-cursor>`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 0 {
				return fmt.Errorf("--limit must be zero or greater")
			}
			client, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			data, err := fetchActivity(client, cursor, limit)
			if err != nil {
				return err
			}
			list := cmdutil.NewListResult(data, "items")
			if err := output.OutputList(list.Raw, list.Rows, []output.Column{
				{Header: "Created", Key: "createdAt"},
				{Header: "Action", Key: "action"},
				{Header: "Outcome", Key: "outcome"},
				{Header: "Channel", Key: "channel"},
				{Header: "Target", Key: "targetType"},
				{Header: "ID", Key: "id"},
			}, list.Total, list.Page); err != nil {
				return err
			}
			if output.IsJSON() {
				return nil
			}
			if next := cmdutil.AsMap(data)["nextCursor"]; next != nil && fmt.Sprint(next) != "" {
				output.Dim(fmt.Sprintf("  Next cursor: %s", next))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue from a previous nextCursor")
	cmd.Flags().IntVarP(&limit, "limit", "L", 0, "Maximum activity entries (1-50)")
	return cmd
}

func fetchActivity(client *api.TypedClient, cursor string, limit int) (any, error) {
	if limit < 0 {
		return nil, fmt.Errorf("--limit must be zero or greater")
	}
	params := &openapi.AccountClientActivityListParams{
		Cursor: cmdutil.StringPtrIfSet(cursor),
		Limit:  cmdutil.Int64PtrIfPositive(limit),
	}
	return api.ParseResponseRaw(client.AccountClientActivityList(api.Ctx(), params))
}
