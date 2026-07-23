package section

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
	"github.com/Life-USTC/CLI/internal/tui"
)

// normalizeScheduleRow converts raw integer fields to display-friendly strings.
func normalizeScheduleRow(row map[string]any) {
	if name, ok := cmdutil.FormatWeekday(row["weekday"]); ok {
		row["weekday"] = name
	}
	for _, k := range []string{"startTime", "endTime"} {
		if formatted, ok := cmdutil.FormatHHMM(row[k]); ok {
			row[k] = formatted
		}
	}
}

func NewCmdSection() *cobra.Command {
	var opts sectionListOpts
	cmd := &cobra.Command{
		Use:   "section [command]",
		Short: "Browse class sections",
		Long:  "List, view, and manage class sections including schedules and calendars.",
		Example: `  # List all sections
  life-ustc catalog section

  # Search sections by keyword
  life-ustc catalog section -s "calculus"

  # Open section search TUI in an interactive terminal
  life-ustc catalog section

  # View a specific section
  life-ustc catalog section get <jw-id>`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSectionList(cmd, opts)
		},
	}
	addSectionListFlags(cmd, &opts)
	cmd.AddCommand(newCmdList())
	cmd.AddCommand(newCmdView())
	cmd.AddCommand(newCmdSchedules())
	cmd.AddCommand(newCmdCalendar())
	cmd.AddCommand(newCmdMatchCodes())
	return cmd
}

type sectionListOpts struct {
	courseID      string
	semesterID    string
	campusID      string
	departmentID  string
	teacherID     string
	search        string
	ids           string
	interactive   bool
	noInteractive bool
	page          int
	limit         int
}

func runSectionList(cmd *cobra.Command, opts sectionListOpts) error {
	useInteractive, err := cmdutil.ShouldUseInteractive(cmd, opts.interactive, opts.noInteractive, "course-id", "semester-id", "campus-id", "department-id", "teacher-id", "search", "ids", "page", "limit")
	if err != nil {
		return err
	}
	if useInteractive {
		c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
		if err != nil {
			return err
		}
		return tui.RunSearchTable(tui.SearchTable{
			Form: tui.SearchForm{
				Title:       "Section Search",
				Description: "Search by course name, section code, teacher, or leave blank for recent results.",
				SearchLabel: "Section",
				Search:      opts.search,
				Limit:       opts.limit,
			},
			Columns: sectionListColumns(),
			Fetch: func(result tui.SearchResult) (tui.TableResult, error) {
				next := opts
				next.search = result.Search
				next.limit = result.Limit
				next.page = result.Page
				list, err := fetchSectionList(c, next)
				if err != nil {
					return tui.TableResult{}, err
				}
				return tui.TableResult{Rows: list.Rows, Total: list.Total, Page: list.Page}, nil
			},
			OnSelect: func(row map[string]any) error {
				return runSectionView(cmd, fmt.Sprint(row["jwId"]))
			},
			EmptyMessage: "No sections found. Try a broader search.",
		})
	}
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	list, err := fetchSectionList(c, opts)
	if err != nil {
		return err
	}
	return output.OutputList(list.Raw, list.Rows, sectionListColumns(), list.Total, list.Page)
}

func fetchSectionList(c *api.TypedClient, opts sectionListOpts) (cmdutil.ListResult, error) {
	params := openapi.ListSectionsParams{}
	var err error
	if params.CourseId, err = cmdutil.Int64PtrIfSet(opts.courseID); err != nil {
		return cmdutil.ListResult{}, err
	}
	if params.SemesterId, err = cmdutil.Int64PtrIfSet(opts.semesterID); err != nil {
		return cmdutil.ListResult{}, err
	}
	if params.CampusId, err = cmdutil.Int64PtrIfSet(opts.campusID); err != nil {
		return cmdutil.ListResult{}, err
	}
	if params.DepartmentId, err = cmdutil.Int64PtrIfSet(opts.departmentID); err != nil {
		return cmdutil.ListResult{}, err
	}
	if params.TeacherId, err = cmdutil.Int64PtrIfSet(opts.teacherID); err != nil {
		return cmdutil.ListResult{}, err
	}
	params.Search = cmdutil.StringPtrIfSet(opts.search)
	params.Ids = cmdutil.StringPtrIfSet(opts.ids)
	params.Page = cmdutil.Int64PtrIfPositive(opts.page)
	params.Limit = cmdutil.Int64PtrIfPositive(opts.limit)
	data, err := api.ParseResponseRaw(c.ListSections(api.Ctx(), &params))
	if err != nil {
		return cmdutil.ListResult{}, err
	}
	return cmdutil.NewListResult(data, "data").FinalizeServerSide(opts.limit), nil
}

func sectionListColumns() []output.Column {
	return []output.Column{
		{Header: "Code", Key: "code"},
		{Header: "Course", Key: "course.namePrimary"},
		{Header: "Semester", Key: "semester.name"},
		{Header: "Campus", Key: "campus.name"},
		{Header: "JW ID", Key: "jwId"},
	}
}

func addSectionListFlags(cmd *cobra.Command, opts *sectionListOpts) {
	cmd.Flags().StringVar(&opts.courseID, "course-id", "", "Course ID")
	cmd.Flags().StringVar(&opts.semesterID, "semester-id", "", "Semester ID")
	cmd.Flags().StringVar(&opts.campusID, "campus-id", "", "Campus ID")
	cmd.Flags().StringVar(&opts.departmentID, "department-id", "", "Department ID")
	cmd.Flags().StringVar(&opts.teacherID, "teacher-id", "", "Teacher ID")
	cmd.Flags().StringVarP(&opts.search, "search", "s", "", "Search query")
	cmd.Flags().StringVar(&opts.ids, "ids", "", "Comma-separated section IDs")
	cmd.Flags().BoolVarP(&opts.interactive, "interactive", "i", false, "Open the interactive search form")
	cmd.Flags().BoolVar(&opts.noInteractive, "no-interactive", false, "Skip the interactive search form")
	cmdutil.AddListFlags(cmd, &opts.page, &opts.limit)
}

func newCmdList() *cobra.Command {
	var opts sectionListOpts
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List sections",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSectionList(cmd, opts)
		},
	}
	addSectionListFlags(cmd, &opts)
	return cmd
}

func newCmdView() *cobra.Command {
	return &cobra.Command{
		Use:     "get <jw-id>",
		Aliases: []string{"show"},
		Short:   "View section details",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSectionView(cmd, args[0])
		},
	}
}

func runSectionView(cmd *cobra.Command, id string) error {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	jwID, err := cmdutil.Int64PtrIfSet(id)
	if err != nil {
		return err
	}
	data, err := api.ParseResponseRaw(c.GetSection(api.Ctx(), *jwID, nil))
	if err != nil {
		return err
	}
	if output.IsJSON() {
		return output.JSON(data)
	}
	m := cmdutil.AsMap(data)
	output.KVWithTitle([]output.KVPair{
		{Key: "ID", Value: output.Resolve(m, "id")},
		{Key: "Code", Value: output.Resolve(m, "code")},
		{Key: "Course", Value: output.Resolve(m, "course.namePrimary")},
		{Key: "Semester", Value: output.Resolve(m, "semester.name")},
		{Key: "Campus", Value: output.Resolve(m, "campus.name")},
	}, "Section")

	if teachers, ok := m["teachers"].([]any); ok && len(teachers) > 0 {
		fmt.Println()
		output.Bold("  Teachers")
		rows := cmdutil.RowsFromAny(teachers)
		output.Table(rows, []output.Column{
			{Header: "ID", Key: "id"},
			{Header: "Name", Key: "namePrimary"},
			{Header: "Name (EN)", Key: "nameSecondary"},
			{Header: "Department", Key: "department.name"},
		})
	}

	if schedules, ok := m["schedules"].([]any); ok && len(schedules) > 0 {
		fmt.Println()
		output.Bold("  Schedules")
		rows := cmdutil.RowsFromAny(schedules)
		for _, row := range rows {
			normalizeScheduleRow(row)
		}
		output.Table(rows, []output.Column{
			{Header: "ID", Key: "id"},
			{Header: "Day", Key: "weekday"},
			{Header: "Start", Key: "startTime"},
			{Header: "End", Key: "endTime"},
			{Header: "Place", Key: "customPlace"},
		})
	}
	return nil
}

func newCmdSchedules() *cobra.Command {
	return &cobra.Command{
		Use:   "schedules <jw-id>",
		Short: "List schedules for a section",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}
			jwID, err := cmdutil.Int64PtrIfSet(args[0])
			if err != nil {
				return err
			}
			data, err := api.ParseResponseRaw(c.GetSectionSchedules(api.Ctx(), *jwID, &openapi.GetSectionSchedulesParams{}))
			if err != nil {
				return err
			}
			_, rows, total, pg := cmdutil.ExtractList(data)
			for _, row := range rows {
				normalizeScheduleRow(row)
			}
			return output.OutputList(data, rows, []output.Column{
				{Header: "ID", Key: "id"},
				{Header: "Day", Key: "weekday"},
				{Header: "Start", Key: "startTime"},
				{Header: "End", Key: "endTime"},
				{Header: "Place", Key: "customPlace"},
			}, total, pg)
		},
	}
}

func newCmdCalendar() *cobra.Command {
	var outFile string
	cmd := &cobra.Command{
		Use:   "calendar <jw-id>",
		Short: "Download ICS calendar for a section",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}
			jwID, err := cmdutil.Int64PtrIfSet(args[0])
			if err != nil {
				return err
			}
			resp, err := c.GetSectionCalendar(api.Ctx(), *jwID)
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if outFile != "" {
				if err := os.WriteFile(outFile, body, 0o644); err != nil {
					return err
				}
				output.Success(fmt.Sprintf("Saved to %s", outFile))
			} else {
				fmt.Print(string(body))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&outFile, "output", "o", "", "Save to file")
	return cmd
}

func newCmdMatchCodes() *cobra.Command {
	var semesterID string
	cmd := &cobra.Command{
		Use:   "match <code>...",
		Short: "Match section codes",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}
			body := openapi.MatchSectionCodesJSONRequestBody{
				Codes: args,
			}
			if semesterID != "" {
				semesterIDUnion := openapi.MatchSectionCodesRequestSchema_SemesterId{}
				_ = semesterIDUnion.FromMatchSectionCodesRequestSchemaSemesterId0(semesterID)
				body.SemesterId = &semesterIDUnion
			}
			data, err := api.ParseResponseRaw(c.MatchSectionCodes(api.Ctx(), body))
			if err != nil {
				return err
			}
			if output.IsJSON() {
				return output.JSON(data)
			}
			_, rows, total, pg := cmdutil.ExtractList(data, "sections")
			if err := output.OutputList(data, rows, []output.Column{
				{Header: "ID", Key: "id"},
				{Header: "Code", Key: "code"},
				{Header: "Course", Key: "course.nameCn"},
				{Header: "Semester", Key: "semester.nameCn"},
			}, total, pg); err != nil {
				return err
			}
			m := cmdutil.AsMap(data)
			if unmatched, ok := m["unmatchedCodes"].([]any); ok && len(unmatched) > 0 {
				fmt.Println()
				output.Warning(fmt.Sprintf("%d code(s) not found: %v", len(unmatched), unmatched))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&semesterID, "semester-id", "", "Semester ID filter")
	return cmd
}
