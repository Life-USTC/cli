package young_organizer

import (
	"fmt"
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

  # View one organizer and its event groups
  life-ustc catalog young-organizer get <organizer-id>`,
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
	data, err := client.DoJSON(cmd.Context(), http.MethodGet, youngutil.OrganizersPath, params, nil)
	if err != nil {
		return err
	}
	list := cmdutil.NewListResult(data, "data")
	return output.OutputList(list.Raw, list.Rows, []output.Column{
		{Header: "ID", Key: "id"},
		{Header: "Name", Key: "name"},
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
			return runGet(cmd, args[0])
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

	params := url.Values{"organizerId": []string{organizerID}}
	events, err := youngutil.FetchAllPages(
		cmd.Context(),
		client,
		youngutil.EventsPath,
		params,
		"data",
		100,
		"youngId",
	)
	if err != nil {
		return err
	}
	eventList := cmdutil.NewListResult(events, "data")
	if output.IsJSON() {
		result := make(map[string]any)
		if organizer := cmdutil.AsMap(data); organizer != nil {
			for key, value := range organizer {
				result[key] = value
			}
		}
		result["events"] = eventList.Rows
		return output.JSON(result)
	}
	if err := output.OutputDetail(data, []output.FieldDef{
		{Key: "id", Label: "ID"},
		{Key: "name", Label: "Name"},
		{Key: "normalizedName", Label: "Normalized name", SkipEmpty: true},
		{Key: "activeCount", Label: "Active events"},
		{Key: "upcomingCount", Label: "Upcoming events"},
		{Key: "historyCount", Label: "Historical events"},
	}, "Young organizer"); err != nil {
		return err
	}
	if len(eventList.Rows) == 0 {
		return nil
	}
	fmt.Println()
	output.Bold("  Events")
	output.Table(eventList.Rows, []output.Column{
		{Header: "Name", Key: "name"},
		{Header: "Category", Key: "category"},
		{Header: "Start", Key: "startAt"},
		{Header: "End", Key: "endAt"},
		{Header: "Active", Key: "isActive"},
		{Header: "Young ID", Key: "youngId"},
	})
	return nil
}
