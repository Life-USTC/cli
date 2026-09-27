package me

import (
	"github.com/Life-USTC/CLI/internal/openapi"
	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/output"
)

func NewCmdProfile() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Show your profile",
		Long:  "Show the account currently authenticated with the active Life@USTC server.",
		Example: `  # Show your profile
  life-ustc account profile

  # Use the dedicated personal commands
  life-ustc workspace todo --pending
  life-ustc workspace homework --pending`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			data, err := api.ParseResponse[openapi.MeResponseSchema](c.AccountProfileGet(api.Ctx()))
			if err != nil {
				return err
			}
			return output.OutputDetail(data, profileFields(), "Profile")
		},
	}

	return cmd
}

func profileFields() []output.FieldDef {
	return []output.FieldDef{
		{Key: "id", Label: "ID"},
		{Key: "name", Label: "Name"},
		{Key: "email", Label: "Email"},
		{Key: "username", Label: "Username"},
	}
}
