package link

import (
	"context"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
)

func NewCmdCatalogLink() *cobra.Command {
	return &cobra.Command{
		Use:   "link",
		Short: "List public campus links",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}
			data, err := api.ParseResponseRaw(client.CatalogLinkList(api.Ctx()))
			if err != nil {
				return err
			}
			list := cmdutil.NewListResult(data, "links")
			return output.OutputList(list.Raw, list.Rows, []output.Column{
				{Header: "Title", Key: "title"},
				{Header: "Group", Key: "group"},
				{Header: "URL", Key: "url"},
				{Header: "Slug", Key: "slug"},
			}, list.Total, list.Page)
		},
	}
}

func NewCmdWorkspaceLinkPin() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link-pin <command>",
		Short: "Manage personal campus link pins",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newCmdListPins(), newCmdSetPin("pin"), newCmdSetPin("unpin"))
	return cmd
}

func newCmdListPins() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List pinned campus link slugs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			data, err := api.ParseResponseRaw(
				client.WorkspaceLinkPinList(api.Ctx()),
			)
			if err != nil {
				return err
			}
			return output.OutputDetail(data, []output.FieldDef{
				{Key: "pinnedSlugs", Label: "Pinned links"},
				{Key: "maxPinnedLinks", Label: "Maximum pins"},
			}, "Link pins")
		},
	}
}

func newCmdSetPin(action string) *cobra.Command {
	return &cobra.Command{
		Use:   action + " <slug>",
		Short: action + " a campus link",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			value := openapi.WorkspaceLinkPinRequestSchemaAction(action)
			data, err := api.ParseResponseRaw(
				client.WorkspaceLinkPinSetWithFormdataBody(
					api.Ctx(),
					openapi.WorkspaceLinkPinSetFormdataRequestBody{
						Action: &value,
						Slug:   args[0],
					},
					func(_ context.Context, request *http.Request) error {
						request.Header.Set("Accept", "application/json")
						return nil
					},
				),
			)
			if err != nil {
				return err
			}
			return output.JSON(data)
		},
	}
}
