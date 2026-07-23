package workspace

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/bus"
	"github.com/Life-USTC/CLI/internal/cmd/calendar"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/cmd/homework"
	"github.com/Life-USTC/CLI/internal/cmd/schedule"
	schoolcmd "github.com/Life-USTC/CLI/internal/cmd/school"
	"github.com/Life-USTC/CLI/internal/cmd/todo"
	"github.com/Life-USTC/CLI/internal/cmd/upload"
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
		bus.NewCmdBusPreferences(),
		upload.NewCmdUpload(),
		schoolcmd.NewCmdSchool(),
	)
	return cmd
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
	events := &cobra.Command{
		Use:   "events",
		Short: "Show today's aggregated calendar events",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := getOverview(cmd)
			if err != nil {
				return err
			}
			if output.IsJSON() {
				return output.JSON(data)
			}
			m := cmdutil.AsMap(data)
			for _, group := range []struct {
				key, title string
				cols       []output.Column
			}{
				{"schedules", "Schedules", []output.Column{{Header: "Course", Key: "section.course.namePrimary"}, {Header: "Time", Key: "startTime"}, {Header: "Place", Key: "customPlace"}}},
				{"exams", "Exams", []output.Column{{Header: "Course", Key: "section.course.namePrimary"}, {Header: "Date", Key: "examDate"}, {Header: "Mode", Key: "examMode"}}},
				{"homeworks", "Homeworks", []output.Column{{Header: "Title", Key: "title"}, {Header: "Due", Key: "submissionDueAt"}, {Header: "Course", Key: "section.course.namePrimary"}}},
				{"dueTodos", "Due todos", []output.Column{{Header: "Title", Key: "title"}, {Header: "Due", Key: "dueAt"}, {Header: "Priority", Key: "priority"}}},
			} {
				part := cmdutil.AsMap(m[group.key])
				rows := cmdutil.RowsFromAny(part["items"])
				if len(rows) == 0 {
					continue
				}
				fmt.Println()
				output.Bold("  " + group.title)
				output.Table(rows, group.cols)
			}
			return nil
		},
	}
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
	return api.ParseResponseRaw(c.GetApiMeOverview(api.Ctx(), &openapi.GetApiMeOverviewParams{}))
}
