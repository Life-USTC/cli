package young_event

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
)

type listOpts struct {
	active   string
	category string
	search   string
	page     int
	pageSize int
}

// NewCmdYoungEvent exposes the public second-classroom event catalog.
func NewCmdYoungEvent() *cobra.Command {
	opts := listOpts{}
	cmd := &cobra.Command{
		Use:     "young-event [command]",
		Aliases: []string{"young-events"},
		Short:   "Browse second-classroom signup events",
		Long:    "List and view public Young (second-classroom) signup events.",
		Example: `  # List events with signup currently open
  life-ustc catalog young-event --active true

  # Search events and request a specific page size
  life-ustc catalog young-event --search robotics --page 1 --limit 20

  # View one event
  life-ustc catalog young-event get <young-id>`,
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
	cmd.Flags().StringVar(&opts.active, "active", "", "Filter by signup status (true or false)")
	cmd.Flags().StringVar(&opts.category, "category", "", "Exact event category")
	cmd.Flags().StringVarP(&opts.search, "search", "s", "", "Search event names")
	cmd.Flags().IntVarP(&opts.page, "page", "p", 0, "Page number")
	// The CLI's standard --limit spelling maps to the API's canonical pageSize.
	cmd.Flags().IntVarP(&opts.pageSize, "limit", "L", 0, "Number of items per page")
}

func newCmdList() *cobra.Command {
	opts := listOpts{}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List Young events",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, opts)
		},
	}
	addListFlags(cmd, &opts)
	return cmd
}

func runList(cmd *cobra.Command, opts listOpts) error {
	params, err := buildListParams(opts)
	if err != nil {
		return err
	}
	client, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	data, err := fetchList(client, params)
	if err != nil {
		return err
	}
	list := cmdutil.NewListResult(data, "data")
	return output.OutputList(list.Raw, list.Rows, []output.Column{
		{Header: "Name", Key: "name"},
		{Header: "Category", Key: "category"},
		{Header: "Start", Key: "startAt"},
		{Header: "End", Key: "endAt"},
		{Header: "Active", Key: "isActive"},
		{Header: "Young ID", Key: "youngId"},
	}, list.Total, list.Page)
}

func fetchList(client *api.TypedClient, params *openapi.GetApiCatalogYoungEventsParams) (any, error) {
	return api.ParseResponseRaw(client.GetApiCatalogYoungEvents(api.Ctx(), params))
}

func buildListParams(opts listOpts) (*openapi.GetApiCatalogYoungEventsParams, error) {
	if opts.page < 0 {
		return nil, fmt.Errorf("--page must be zero or greater")
	}
	if opts.pageSize < 0 {
		return nil, fmt.Errorf("--limit must be zero or greater")
	}
	active, err := normalizeActive(opts.active)
	if err != nil {
		return nil, err
	}
	return &openapi.GetApiCatalogYoungEventsParams{
		Active:   active,
		Category: cmdutil.StringPtrIfSet(opts.category),
		Search:   cmdutil.StringPtrIfSet(opts.search),
		Page:     cmdutil.Int64PtrIfPositive(opts.page),
		PageSize: cmdutil.Int64PtrIfPositive(opts.pageSize),
	}, nil
}

func normalizeActive(value string) (*openapi.GetApiCatalogYoungEventsParamsActive, error) {
	if value == "" {
		return nil, nil
	}
	if value != "true" && value != "false" {
		return nil, fmt.Errorf("--active must be true or false")
	}
	active := openapi.GetApiCatalogYoungEventsParamsActive(value)
	return &active, nil
}

func newCmdGet() *cobra.Command {
	return &cobra.Command{
		Use:     "get <young-id>",
		Aliases: []string{"show"},
		Short:   "View a Young event",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}
			data, err := api.ParseResponseRaw(client.GetApiCatalogYoungEventsYoungId(api.Ctx(), args[0]))
			if err != nil {
				return err
			}
			return output.OutputDetail(data, []output.FieldDef{
				{Key: "youngId", Label: "Young ID"},
				{Key: "name", Label: "Name"},
				{Key: "category", Label: "Category", SkipEmpty: true},
				{Key: "department", Label: "Department", SkipEmpty: true},
				{Key: "organizer", Label: "Organizer", SkipEmpty: true},
				{Key: "location", Label: "Location", SkipEmpty: true},
				{Key: "startAt", Label: "Start", SkipEmpty: true},
				{Key: "endAt", Label: "End", SkipEmpty: true},
				{Key: "applyStartAt", Label: "Signup starts", SkipEmpty: true},
				{Key: "applyEndAt", Label: "Signup ends", SkipEmpty: true},
				{Key: "hours", Label: "Hours", SkipEmpty: true},
				{Key: "capacity", Label: "Capacity", SkipEmpty: true},
				{Key: "appliedCount", Label: "Applied", SkipEmpty: true},
				{Key: "registrationStatus", Label: "Registration", SkipEmpty: true},
				{Key: "status", Label: "Status", SkipEmpty: true},
				{Key: "isActive", Label: "Active"},
				{Key: "imageUrl", Label: "Image", SkipEmpty: true},
			}, "Young event")
		},
	}
}
