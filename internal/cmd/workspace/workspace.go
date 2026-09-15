package workspace

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/bus"
	"github.com/Life-USTC/CLI/internal/cmd/calendar"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/cmd/homework"
	"github.com/Life-USTC/CLI/internal/cmd/link"
	"github.com/Life-USTC/CLI/internal/cmd/schedule"
	schoolcmd "github.com/Life-USTC/CLI/internal/cmd/school"
	"github.com/Life-USTC/CLI/internal/cmd/todo"
	"github.com/Life-USTC/CLI/internal/cmd/upload"
	"github.com/Life-USTC/CLI/internal/cmd/young_workspace"
	"github.com/Life-USTC/CLI/internal/cmd/youngutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
)

func NewCmdWorkspace() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace <command>",
		Short: "Manage your campus work",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(
		newCmdOverview(),
		newCmdCalendar(),
		schedule.NewCmdWorkspaceSchedule(),
		newCmdExam(),
		todo.NewCmdTodo(),
		homework.NewCmdMyHomework(),
		calendar.NewCmdSubscription(),
		young_workspace.NewCmdYoungEventSubscription(),
		young_workspace.NewCmdYoungOrganizerSubscription(),
		young_workspace.NewCmdYoungNotification(),
		bus.NewCmdBusPreferences(),
		link.NewCmdWorkspaceLinkPin(),
		upload.NewCmdUpload(),
		schoolcmd.NewCmdSchool(),
	)
	return cmd
}

type calendarEventOpts struct {
	dateFrom string
	dateTo   string
	page     int
	pageSize int
}

func runCalendarEvents(cmd *cobra.Command, opts calendarEventOpts) error {
	params, err := buildCalendarEventParams(opts)
	if err != nil {
		return err
	}
	client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return err
	}
	var data any
	if opts.page == 0 && opts.pageSize == 0 {
		data, err = youngutil.FetchAllPages(
			cmd.Context(),
			client,
			youngutil.PersonalCalendarEventsPath,
			params,
			"data",
			100,
			"id",
		)
	} else {
		data, err = client.DoJSON(
			cmd.Context(),
			http.MethodGet,
			youngutil.PersonalCalendarEventsPath,
			params,
			nil,
		)
	}
	if err != nil {
		return err
	}
	list := cmdutil.NewListResult(data, "data")
	return output.OutputList(list.Raw, list.Rows, []output.Column{
		{Header: "At", Key: "at"},
		{Header: "End", Key: "endsAt"},
		{Header: "Type", Key: "type"},
		{Header: "Title", Key: "title"},
		{Header: "Location", Key: "location"},
		{Header: "Young ID", Key: "youngId"},
		{Header: "URL", Key: "url"},
	}, list.Total, list.Page)
}

func buildCalendarEventParams(opts calendarEventOpts) (url.Values, error) {
	params, err := youngutil.PageParams(opts.page, opts.pageSize)
	if err != nil {
		return nil, err
	}
	from := strings.TrimSpace(opts.dateFrom)
	to := strings.TrimSpace(opts.dateTo)
	if (from == "") != (to == "") {
		return nil, fmt.Errorf("--date-from and --date-to must be provided together")
	}
	if from != "" {
		params.Set("dateFrom", from)
		params.Set("dateTo", to)
	}
	return params, nil
}

func newCmdOverview() *cobra.Command {
	return &cobra.Command{
		Use:   "overview",
		Short: "Show a compact workspace overview",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := getOverview(cmd)
			if err != nil {
				return err
			}
			if output.IsJSON() {
				return output.JSON(data)
			}
			counts := cmdutil.AsMap(cmdutil.AsMap(data)["counts"])
			todos := cmdutil.AsMap(counts["todos"])
			return output.OutputDetail(map[string]any{
				"todaySchedules":   counts["todaySchedules"],
				"upcomingExams":    counts["upcomingExams"],
				"pendingHomeworks": counts["pendingHomeworks"],
				"incompleteTodos":  todos["incomplete"],
				"overdueTodos":     todos["overdue"],
			}, []output.FieldDef{
				{Key: "todaySchedules", Label: "Today's classes"},
				{Key: "upcomingExams", Label: "Upcoming exams"},
				{Key: "pendingHomeworks", Label: "Pending homeworks"},
				{Key: "incompleteTodos", Label: "Incomplete todos"},
				{Key: "overdueTodos", Label: "Overdue todos"},
			}, "Workspace overview")
		},
	}
}

func newCmdCalendar() *cobra.Command {
	cmd := calendar.NewCmdCalendar()
	var dateFrom, dateTo string
	var page, pageSize int
	events := &cobra.Command{
		Use:   "events",
		Short: "List your complete personal calendar events",
		Long: `List personal calendar events from the date range returned by the
workspace calendar API. With no bounds, the server uses the current Shanghai
date and the following seven days. Use --date-from and --date-to together for
another inclusive range.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCalendarEvents(cmd, calendarEventOpts{
				dateFrom: dateFrom,
				dateTo:   dateTo,
				page:     page,
				pageSize: pageSize,
			})
		},
	}
	events.Flags().StringVar(&dateFrom, "date-from", "", "Inclusive Shanghai date/time range start")
	events.Flags().StringVar(&dateTo, "date-to", "", "Inclusive Shanghai date/time range end")
	events.Flags().IntVarP(&page, "page", "p", 0, "Page number")
	events.Flags().IntVarP(&pageSize, "limit", "L", 0, "Number of calendar events per page")
	cmd.AddCommand(events)
	return cmd
}

func newCmdExam() *cobra.Command {
	return &cobra.Command{
		Use:   "exam",
		Short: "List your upcoming exams",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := getOverview(cmd)
			if err != nil {
				return err
			}
			exams := cmdutil.AsMap(cmdutil.AsMap(data)["exams"])
			rows := cmdutil.RowsFromAny(exams["items"])
			return output.OutputList(exams, rows, []output.Column{
				{Header: "Course", Key: "section.course.namePrimary"},
				{Header: "Date", Key: "examDate"},
				{Header: "Start", Key: "startTime"},
				{Header: "End", Key: "endTime"},
				{Header: "Mode", Key: "examMode"},
			}, len(rows), 1)
		},
	}
}

func getOverview(cmd *cobra.Command) (any, error) {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return nil, err
	}
	return api.ParseResponseRaw(
		c.WorkspaceOverviewGet(api.Ctx(), &openapi.WorkspaceOverviewGetParams{}),
	)
}
