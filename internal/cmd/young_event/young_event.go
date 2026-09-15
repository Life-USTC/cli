package young_event

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/cmd/youngutil"
	"github.com/Life-USTC/CLI/internal/output"
)

type listOpts struct {
	active      string
	dateUnknown string
	category    string
	search      string
	organizerID string
	dateFrom    string
	dateTo      string
	timeBasis   string
	page        int
	pageSize    int
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

  # Filter by organizer and an activity date range
  life-ustc catalog young-event --organizer-id <organizer-id> --date-from 2026-09-01 --date-to 2026-09-30

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
	cmd.AddCommand(newCmdDate())
	return cmd
}

func addListFlags(cmd *cobra.Command, opts *listOpts) {
	cmd.Flags().StringVar(&opts.active, "active", "", "Filter by signup status (true or false)")
	cmd.Flags().StringVar(&opts.dateUnknown, "date-unknown", "", "Filter events with unknown activity dates (true or false)")
	cmd.Flags().StringVar(&opts.category, "category", "", "Exact event category")
	cmd.Flags().StringVarP(&opts.search, "search", "s", "", "Search event names")
	cmd.Flags().StringVar(&opts.organizerID, "organizer-id", "", "Filter by stable organizer ID")
	cmd.Flags().StringVar(&opts.dateFrom, "date-from", "", "Inclusive date/time range start")
	cmd.Flags().StringVar(&opts.dateTo, "date-to", "", "Inclusive date/time range end")
	cmd.Flags().StringVar(&opts.timeBasis, "time-basis", "", "Date fields for range filtering (activity or registration)")
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
		Args:    cobra.NoArgs,
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
	client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	data, err := youngutil.FetchAllIfUnpaged(
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
	list := cmdutil.NewListResult(data, "data")
	return output.OutputList(list.Raw, list.Rows, []output.Column{
		{Header: "Name", Key: "name"},
		{Header: "Category", Key: "category"},
		{Header: "Start", Key: dateStartKey(opts.timeBasis)},
		{Header: "End", Key: dateEndKey(opts.timeBasis)},
		{Header: "Active", Key: "isActive"},
		{Header: "Young ID", Key: "youngId"},
	}, list.Total, list.Page)
}

func fetchList(ctx context.Context, client *api.Client, params url.Values) (any, error) {
	return client.DoJSON(ctx, http.MethodGet, youngutil.EventsPath, params, nil)
}

func buildListParams(opts listOpts) (url.Values, error) {
	params, err := youngutil.PageParams(opts.page, opts.pageSize)
	if err != nil {
		return nil, err
	}
	active, err := normalizeActive(opts.active)
	if err != nil {
		return nil, err
	}
	if active != "" {
		params.Set("active", active)
	}
	dateUnknown, err := normalizeBooleanFilter("--date-unknown", opts.dateUnknown)
	if err != nil {
		return nil, err
	}
	from := strings.TrimSpace(opts.dateFrom)
	to := strings.TrimSpace(opts.dateTo)
	if (from == "") != (to == "") {
		return nil, fmt.Errorf("--date-from and --date-to must be provided together")
	}
	if dateUnknown != "" && (from != "" || to != "") {
		return nil, fmt.Errorf("--date-unknown cannot be combined with --date-from or --date-to")
	}
	if dateUnknown != "" {
		params.Set("dateUnknown", dateUnknown)
	}
	for key, value := range map[string]string{
		"category":    opts.category,
		"search":      opts.search,
		"organizerId": opts.organizerID,
		"dateFrom":    from,
		"dateTo":      to,
	} {
		if value = strings.TrimSpace(value); value != "" {
			params.Set(key, value)
		}
	}
	timeBasis := strings.TrimSpace(opts.timeBasis)
	if timeBasis != "" {
		if timeBasis != "activity" && timeBasis != "registration" {
			return nil, fmt.Errorf("--time-basis must be activity or registration")
		}
		params.Set("timeBasis", timeBasis)
	}
	return params, nil
}

func normalizeActive(value string) (string, error) {
	return normalizeBooleanFilter("--active", value)
}

func normalizeBooleanFilter(flag, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if value != "true" && value != "false" {
		return "", fmt.Errorf("%s must be true or false", flag)
	}
	return value, nil
}

func newCmdGet() *cobra.Command {
	return &cobra.Command{
		Use:     "get <young-id>",
		Aliases: []string{"show"},
		Short:   "View a Young event",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			youngID, err := youngutil.RequireID(args[0], "<young-id>")
			if err != nil {
				return err
			}
			client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}
			data, err := client.DoJSON(
				cmd.Context(),
				http.MethodGet,
				youngutil.PathID(youngutil.EventsPath, youngID),
				nil,
				nil,
			)
			if err != nil {
				return err
			}
			return output.OutputDetail(data, []output.FieldDef{
				{Key: "youngId", Label: "Young ID"},
				{Key: "name", Label: "Name"},
				{Key: "category", Label: "Category", SkipEmpty: true},
				{Key: "department", Label: "Department", SkipEmpty: true},
				{Key: "organizer", Label: "Organizer", SkipEmpty: true},
				{Key: "organizerId", Label: "Organizer ID", SkipEmpty: true},
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
				{Key: "sourceMissing", Label: "Source missing", SkipEmpty: true},
				{Key: "lastSeenAt", Label: "Last seen", SkipEmpty: true},
				{Key: "createdAt", Label: "Created", SkipEmpty: true},
				{Key: "imageUrl", Label: "Image", SkipEmpty: true},
			}, "Young event")
		},
	}
}

type dateOpts struct {
	active      string
	category    string
	search      string
	organizerID string
	timeBasis   string
}

func newCmdDate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "date <command>",
		Short: "List every event in a day, week, or month",
		Args:  cobra.NoArgs,
	}
	for _, view := range []youngutil.DateView{
		youngutil.DateDay,
		youngutil.DateWeek,
		youngutil.DateMonth,
	} {
		cmd.AddCommand(newDateViewCommand(view))
	}
	return cmd
}

func newDateViewCommand(view youngutil.DateView) *cobra.Command {
	opts := dateOpts{}
	cmd := &cobra.Command{
		Use:     string(view) + " [YYYY-MM-DD]",
		Short:   "List all Young events for a " + string(view),
		Args:    cobra.MaximumNArgs(1),
		Example: "  life-ustc catalog young-event date " + string(view) + " 2026-09-15",
		RunE: func(cmd *cobra.Command, args []string) error {
			anchor := ""
			if len(args) == 1 {
				anchor = args[0]
			}
			return runDateView(cmd, view, anchor, opts)
		},
	}
	cmd.Flags().StringVar(&opts.active, "active", "", "Filter by signup status (true or false)")
	cmd.Flags().StringVar(&opts.category, "category", "", "Exact event category")
	cmd.Flags().StringVarP(&opts.search, "search", "s", "", "Search event names")
	cmd.Flags().StringVar(&opts.organizerID, "organizer-id", "", "Filter by stable organizer ID")
	cmd.Flags().StringVar(&opts.timeBasis, "time-basis", "activity", "Date fields for range filtering (activity or registration)")
	return cmd
}

func runDateView(cmd *cobra.Command, view youngutil.DateView, anchor string, opts dateOpts) error {
	dateFrom, dateTo, err := youngutil.DateRange(view, anchor, timeNow())
	if err != nil {
		return err
	}
	params := listOpts{
		active:      opts.active,
		category:    opts.category,
		search:      opts.search,
		organizerID: opts.organizerID,
		dateFrom:    dateFrom,
		dateTo:      dateTo,
		timeBasis:   opts.timeBasis,
	}
	query, err := buildListParams(params)
	if err != nil {
		return err
	}
	client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	data, err := fetchDateEvents(cmd.Context(), client, query)
	if err != nil {
		return err
	}
	if !output.IsJSON() {
		meta := cmdutil.AsMap(data)
		output.KVWithTitle([]output.KVPair{
			{Key: "Source", Value: output.Resolve(meta, "source.status")},
			{Key: "Last synced", Value: output.Resolve(meta, "source.lastSyncedAt")},
			{Key: "Unknown dates", Value: output.Resolve(meta, "unknownDateCount")},
		}, "Calendar source")
		fmt.Println("Unknown-date activities: life-ustc catalog young-event list --date-unknown true --time-basis " + opts.timeBasis)
	}
	list := cmdutil.NewListResult(data, "data")
	return output.OutputList(list.Raw, list.Rows, []output.Column{
		{Header: "Name", Key: "name"},
		{Header: "Category", Key: "category"},
		{Header: "Start", Key: dateStartKey(opts.timeBasis)},
		{Header: "End", Key: dateEndKey(opts.timeBasis)},
		{Header: "Organizer", Key: "organizer"},
		{Header: "Young ID", Key: "youngId"},
	}, list.Total, list.Page)
}

func dateStartKey(timeBasis string) string {
	if strings.TrimSpace(timeBasis) == "registration" {
		return "applyStartAt"
	}
	return "startAt"
}

func dateEndKey(timeBasis string) string {
	if strings.TrimSpace(timeBasis) == "registration" {
		return "applyEndAt"
	}
	return "endAt"
}

func fetchDateEvents(ctx context.Context, client *api.Client, query url.Values) (any, error) {
	return youngutil.FetchAllPages(
		ctx,
		client,
		youngutil.EventsPath,
		query,
		"data",
		100,
		"youngId",
	)
}

// timeNow is a variable for deterministic date-range tests.
var timeNow = func() time.Time { return time.Now() }
