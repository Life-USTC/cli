package schedule

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
)

type scheduleListOpts struct {
	sectionID, teacherID, roomID, dateFrom, dateTo string
	weekday                                        int
	page, limit                                    int
}

func NewCmdSchedule() *cobra.Command {
	opts := scheduleListOpts{}
	cmd := &cobra.Command{
		Use:   "schedule [command]",
		Short: "Browse schedules",
		Long:  "List class schedules with optional filters for section, teacher, date range, and weekday.",
		Example: `  # List all schedules
  life-ustc schedule

  # Filter by weekday
  life-ustc schedule list --weekday 3`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScheduleList(cmd, opts)
		},
	}
	addScheduleListFlags(cmd, &opts)
	cmd.AddCommand(newCmdList())
	return cmd
}

func addScheduleListFlags(cmd *cobra.Command, opts *scheduleListOpts) {
	cmd.Flags().StringVar(&opts.sectionID, "section-id", "", "Section ID")
	cmd.Flags().StringVar(&opts.teacherID, "teacher-id", "", "Teacher ID")
	cmd.Flags().StringVar(&opts.roomID, "room-id", "", "Room ID")
	cmd.Flags().StringVar(&opts.dateFrom, "date-from", "", "Start date")
	cmd.Flags().StringVar(&opts.dateTo, "date-to", "", "End date")
	cmd.Flags().IntVar(&opts.weekday, "weekday", 0, "Weekday (1=Mon, 7=Sun)")
	cmdutil.AddListFlags(cmd, &opts.page, &opts.limit)
}

func runScheduleList(cmd *cobra.Command, opts scheduleListOpts) error {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	params := openapi.ListSchedulesParams{}
	params.SectionId = cmdutil.StringPtrIfSet(opts.sectionID)
	params.TeacherId = cmdutil.StringPtrIfSet(opts.teacherID)
	params.RoomId = cmdutil.StringPtrIfSet(opts.roomID)
	if opts.dateFrom != "" {
		t, err := time.Parse(time.DateOnly, opts.dateFrom)
		if err != nil {
			return fmt.Errorf("invalid --date-from: %w", err)
		}
		params.DateFrom = &t
	}
	if opts.dateTo != "" {
		t, err := time.Parse(time.DateOnly, opts.dateTo)
		if err != nil {
			return fmt.Errorf("invalid --date-to: %w", err)
		}
		params.DateTo = &t
	}
	if opts.weekday > 0 {
		params.Weekday = cmdutil.IntStringPtrIfPositive(opts.weekday)
	}
	params.Page = cmdutil.IntStringPtrIfPositive(opts.page)
	params.Limit = cmdutil.IntStringPtrIfPositive(opts.limit)
	data, err := api.ParseResponseRaw(c.ListSchedules(api.Ctx(), &params))
	if err != nil {
		return err
	}
	list := cmdutil.NewListResult(data, "data").FinalizeServerSide(opts.limit)

	for _, row := range list.Rows {
		normalizeScheduleListRow(row)
	}

	return output.OutputList(list.Raw, list.Rows, []output.Column{
		{Header: "Course", Key: "section.course.namePrimary"},
		{Header: "Section", Key: "section.code"},
		{Header: "Day", Key: "weekday"},
		{Header: "Time", Key: "timeRange"},
		{Header: "Place", Key: "customPlace"},
		{Header: "ID", Key: "id"},
	}, list.Total, list.Page)
}

func normalizeScheduleListRow(row map[string]any) {
	start, startOK := cmdutil.FormatHHMM(row["startTime"])
	end, endOK := cmdutil.FormatHHMM(row["endTime"])
	if startOK || endOK {
		row["timeRange"] = strings.Trim(start+"-"+end, "-")
	}
	if name, ok := cmdutil.FormatWeekday(row["weekday"]); ok {
		row["weekday"] = name
	}
	if row["customPlace"] == nil {
		if room := cmdutil.AsMap(row["room"]); room != nil {
			if name, _ := room["namePrimary"].(string); name != "" {
				row["customPlace"] = name
			}
		}
	}
}

func newCmdList() *cobra.Command {
	opts := scheduleListOpts{}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List schedules",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScheduleList(cmd, opts)
		},
	}
	addScheduleListFlags(cmd, &opts)
	return cmd
}
