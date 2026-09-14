package community

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/output"
)

func newCmdUser() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user <command>",
		Short: "Show a public community user",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(&cobra.Command{
		Use:     "get <identifier>",
		Aliases: []string{"show"},
		Short:   "View a public community user",
		Long:    "View a public community profile by username or user identifier.",
		Example: `  life-ustc community user get <username-or-id>
  life-ustc community user get <username-or-id> --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			identifier := strings.TrimSpace(args[0])
			if identifier == "" {
				return fmt.Errorf("user identifier must not be empty")
			}
			client, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}
			data, err := fetchUser(client, identifier)
			if err != nil {
				return err
			}
			if output.IsJSON() {
				return output.JSON(data)
			}
			m := cmdutil.AsMap(data)
			if err := output.OutputDetail(data, []output.FieldDef{
				{Key: "user.id", Label: "ID"},
				{Key: "user.username", Label: "Username", SkipEmpty: true},
				{Key: "user.name", Label: "Name", SkipEmpty: true},
				{Key: "user.createdAt", Label: "Joined"},
				{Key: "sectionCount", Label: "Sections"},
				{Key: "totalContributions", Label: "Contributions"},
				{Key: "user._count.comments", Label: "Comments"},
				{Key: "user._count.homeworksCreated", Label: "Homeworks created"},
				{Key: "user._count.subscribedSections", Label: "Subscribed sections"},
				{Key: "user._count.uploads", Label: "Uploads"},
			}, "Community user"); err != nil {
				return err
			}
			if weeks, ok := m["weeks"].([]any); ok && len(weeks) > 0 {
				fmt.Println()
				output.Bold("  Contributions by week")
				rows := make([]map[string]any, 0, len(weeks))
				for _, week := range weeks {
					if values, ok := week.([]any); ok {
						for _, value := range values {
							if row, ok := value.(map[string]any); ok {
								rows = append(rows, row)
							}
						}
					}
				}
				output.Table(rows, []output.Column{
					{Header: "Date", Key: "date"},
					{Header: "Count", Key: "count"},
				})
			}
			return nil
		},
	})
	return cmd
}

func fetchUser(client *api.TypedClient, identifier string) (any, error) {
	return api.ParseResponseRaw(client.CommunityUserGet(api.Ctx(), identifier))
}
