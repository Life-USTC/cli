package teacher

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
	"github.com/Life-USTC/CLI/internal/tui"
)

func NewCmdTeacher() *cobra.Command {
	var opts teacherListOpts
	cmd := &cobra.Command{
		Use:   "teacher [command]",
		Short: "Browse teachers",
		Long:  "List and view teacher profiles and their associated sections.",
		Example: `  # List all teachers
  life-ustc catalog teacher

  # Search teachers by name
  life-ustc catalog teacher -s "zhang"

  # Open teacher search TUI in an interactive terminal
  life-ustc catalog teacher

  # View a specific teacher
  life-ustc catalog teacher get <teacher-id>`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeacherList(cmd, opts)
		},
	}
	addTeacherListFlags(cmd, &opts)
	cmd.AddCommand(newCmdList())
	cmd.AddCommand(newCmdView())
	return cmd
}

type teacherListOpts struct {
	departmentID  string
	search        string
	interactive   bool
	noInteractive bool
	page          int
	limit         int
}

func runTeacherList(cmd *cobra.Command, opts teacherListOpts) error {
	useInteractive, err := cmdutil.ShouldUseInteractive(cmd, opts.interactive, opts.noInteractive, "department-id", "search", "page", "limit")
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
				Title:       "Teacher Search",
				Description: "Search by teacher name, code, department, or leave blank for recent results.",
				SearchLabel: "Teacher",
				Search:      opts.search,
				Limit:       opts.limit,
			},
			Columns: teacherListColumns(),
			Fetch: func(result tui.SearchResult) (tui.TableResult, error) {
				next := opts
				next.search = result.Search
				next.limit = result.Limit
				next.page = result.Page
				list, err := fetchTeacherList(c, next)
				if err != nil {
					return tui.TableResult{}, err
				}
				return tui.TableResult{Rows: list.Rows, Total: list.Total, Page: list.Page}, nil
			},
			OnSelect: func(row map[string]any) error {
				return runTeacherView(cmd, fmt.Sprint(row["id"]))
			},
			EmptyMessage: "No teachers found. Try a broader search.",
		})
	}
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	list, err := fetchTeacherList(c, opts)
	if err != nil {
		return err
	}
	return output.OutputList(list.Raw, list.Rows, teacherListColumns(), list.Total, list.Page)
}

func fetchTeacherList(c *api.TypedClient, opts teacherListOpts) (cmdutil.ListResult, error) {
	params := openapi.ListTeachersParams{}
	var err error
	if params.DepartmentId, err = cmdutil.Int64PtrIfSet(opts.departmentID); err != nil {
		return cmdutil.ListResult{}, err
	}
	params.Search = cmdutil.StringPtrIfSet(opts.search)
	params.Page = cmdutil.Int64PtrIfPositive(opts.page)
	params.Limit = cmdutil.Int64PtrIfPositive(opts.limit)
	data, err := api.ParseResponseRaw(c.ListTeachers(api.Ctx(), &params))
	if err != nil {
		return cmdutil.ListResult{}, err
	}
	return cmdutil.NewListResult(data, "data").FinalizeServerSide(opts.limit), nil
}

func teacherListColumns() []output.Column {
	return []output.Column{
		{Header: "Name", Key: "namePrimary"},
		{Header: "Department", Key: "department.name"},
		{Header: "Code", Key: "code"},
		{Header: "ID", Key: "id"},
	}
}

func addTeacherListFlags(cmd *cobra.Command, opts *teacherListOpts) {
	cmd.Flags().StringVar(&opts.departmentID, "department-id", "", "Department ID")
	cmd.Flags().StringVarP(&opts.search, "search", "s", "", "Search query")
	cmd.Flags().BoolVarP(&opts.interactive, "interactive", "i", false, "Open the interactive search form")
	cmd.Flags().BoolVar(&opts.noInteractive, "no-interactive", false, "Skip the interactive search form")
	cmdutil.AddListFlags(cmd, &opts.page, &opts.limit)
}

func newCmdList() *cobra.Command {
	var opts teacherListOpts
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List teachers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeacherList(cmd, opts)
		},
	}
	addTeacherListFlags(cmd, &opts)
	return cmd
}

func newCmdView() *cobra.Command {
	return &cobra.Command{
		Use:     "get <teacher-id>",
		Aliases: []string{"show"},
		Short:   "View teacher details",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeacherView(cmd, args[0])
		},
	}
}

func runTeacherView(cmd *cobra.Command, id string) error {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	teacherID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid teacher id %q: %w", id, err)
	}
	data, err := api.ParseResponseRaw(c.GetTeacher(api.Ctx(), teacherID, nil))
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
		{Key: "Name", Value: output.Resolve(m, "namePrimary")},
		{Key: "Name (EN)", Value: output.Resolve(m, "nameSecondary")},
		{Key: "Department", Value: output.Resolve(m, "department.name")},
		{Key: "Title", Value: output.Resolve(m, "title")},
	}, "Teacher")

	if sections, ok := m["sections"].([]any); ok && len(sections) > 0 {
		fmt.Println()
		output.Bold("  Sections")
		rows := cmdutil.RowsFromAny(sections)
		output.Table(rows, []output.Column{
			{Header: "ID", Key: "id"},
			{Header: "Code", Key: "code"},
			{Header: "Course", Key: "course.namePrimary"},
			{Header: "Semester", Key: "semester.name"},
		})
	}
	return nil
}
