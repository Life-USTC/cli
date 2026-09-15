package young_organizer

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/cmd/youngutil"
	"github.com/Life-USTC/CLI/internal/output"
)

type listOpts struct {
	search   string
	page     int
	pageSize int
}

// NewCmdYoungOrganizer exposes the public organizer view for Young events.
func NewCmdYoungOrganizer() *cobra.Command {
	opts := listOpts{}
	cmd := &cobra.Command{
		Use:     "young-organizer [command]",
		Aliases: []string{"young-organizers"},
		Short:   "Browse second-classroom organizers",
		Long:    "List and view normalized Young (second-classroom) organizers.",
		Example: `  # List organizers
  life-ustc catalog young-organizer list

  # View organizer metadata and activity counts
  life-ustc catalog young-organizer get <organizer-id>

  # List the organizer's activities
  life-ustc catalog young-event --organizer-id <organizer-id>`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, opts)
		},
	}
	addListFlags(cmd, &opts)
	cmd.AddCommand(newCmdList())
	cmd.AddCommand(newCmdGet())
	return cmd
}

func addListFlags(cmd *cobra.Command, opts *listOpts) {
	cmd.Flags().StringVarP(&opts.search, "search", "s", "", "Search organizer names")
	cmd.Flags().IntVarP(&opts.page, "page", "p", 0, "Page number")
	cmd.Flags().IntVarP(&opts.pageSize, "limit", "L", 0, "Number of organizers per page")
}

func newCmdList() *cobra.Command {
	opts := listOpts{}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List Young organizers",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, opts)
		},
	}
	addListFlags(cmd, &opts)
	return cmd
}

func buildListParams(opts listOpts) (url.Values, error) {
	params, err := youngutil.PageParams(opts.page, opts.pageSize)
	if err != nil {
		return nil, err
	}
	if search := strings.TrimSpace(opts.search); search != "" {
		params.Set("search", search)
	}
	return params, nil
}

func runList(cmd *cobra.Command, opts listOpts) error {
	params, err := buildListParams(opts)
	if err != nil {
		return err
	}
	client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	data, err := youngutil.FetchAllIfUnpaged(
		cmd.Context(),
		client,
		youngutil.OrganizersPath,
		params,
		"data",
		100,
		"id",
	)
	if err != nil {
		return err
	}
	list := cmdutil.NewListResult(data, "data")
	return output.OutputList(list.Raw, list.Rows, []output.Column{
		{Header: "ID", Key: "id"},
		{Header: "Name", Key: "name"},
		{Header: "Total", Key: "totalCount"},
		{Header: "Active", Key: "activeCount"},
		{Header: "Upcoming", Key: "upcomingCount"},
		{Header: "History", Key: "historyCount"},
	}, list.Total, list.Page)
}

func newCmdGet() *cobra.Command {
	return &cobra.Command{
		Use:     "get <organizer-id>",
		Aliases: []string{"show"},
		Short:   "View a Young organizer",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			organizerID, err := youngutil.RequireID(args[0], "<organizer-id>")
			if err != nil {
				return err
			}
			return runGet(cmd, organizerID)
		},
	}
}

func runGet(cmd *cobra.Command, organizerID string) error {
	client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	data, err := client.DoJSON(
		cmd.Context(),
		http.MethodGet,
		youngutil.PathID(youngutil.OrganizersPath, organizerID),
		nil,
		nil,
	)
	if err != nil {
		return err
	}

	return output.OutputDetail(data, []output.FieldDef{
		{Key: "id", Label: "ID"},
		{Key: "name", Label: "Name"},
		{Key: "normalizedName", Label: "Normalized name", SkipEmpty: true},
		{Key: "totalCount", Label: "Total events"},
		{Key: "activeCount", Label: "Active events"},
		{Key: "upcomingCount", Label: "Upcoming events"},
		{Key: "historyCount", Label: "Historical events"},
	}, "Young organizer")
}
