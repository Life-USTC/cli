package course

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
	"github.com/Life-USTC/CLI/internal/tui"
)

func NewCmdCourse() *cobra.Command {
	var opts courseListOpts
	cmd := &cobra.Command{
		Use:   "course [command]",
		Short: "Browse courses",
		Long:  "List and view courses offered at USTC.",
		Example: `  # List all courses
  life-ustc catalog course

  # Search courses by keyword
  life-ustc catalog course -s "linear algebra"

  # Open course search TUI in an interactive terminal
  life-ustc catalog course

  # View a specific course
  life-ustc catalog course get <jw-id>`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCourseList(cmd, opts)
		},
	}
	addCourseListFlags(cmd, &opts)
	cmd.AddCommand(newCmdList())
	cmd.AddCommand(newCmdView())
	return cmd
}

type courseListOpts struct {
	search           string
	educationLevelID string
	categoryID       string
	classTypeID      string
	interactive      bool
	noInteractive    bool
	page             int
	limit            int
}

func runCourseList(cmd *cobra.Command, opts courseListOpts) error {
	useInteractive, err := cmdutil.ShouldUseInteractive(cmd, opts.interactive, opts.noInteractive, "search", "education-level-id", "category-id", "class-type-id", "page", "limit")
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
				Title:       "Course Search",
				Description: "Search by course name, code, category, or leave blank for recent results.",
				SearchLabel: "Course",
				Search:      opts.search,
				Limit:       opts.limit,
			},
			Columns: courseListColumns(),
			Fetch: func(result tui.SearchResult) (tui.TableResult, error) {
				next := opts
				next.search = result.Search
				next.limit = result.Limit
				next.page = result.Page
				list, err := fetchCourseList(c, next)
				if err != nil {
					return tui.TableResult{}, err
				}
				return tui.TableResult{Rows: list.Rows, Total: list.Total, Page: list.Page}, nil
			},
			OnSelect: func(row map[string]any) error {
				return runCourseView(cmd, fmt.Sprint(row["jwId"]))
			},
			EmptyMessage: "No courses found. Try a broader search.",
		})
	}
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	list, err := fetchCourseList(c, opts)
	if err != nil {
		return err
	}
	return output.OutputList(list.Raw, list.Rows, courseListColumns(), list.Total, list.Page)
}

func fetchCourseList(c *api.TypedClient, opts courseListOpts) (cmdutil.ListResult, error) {
	params := openapi.ListCoursesParams{}
	var err error
	params.Search = cmdutil.StringPtrIfSet(opts.search)
	if params.EducationLevelId, err = cmdutil.Int64PtrIfSet(opts.educationLevelID); err != nil {
		return cmdutil.ListResult{}, err
	}
	if params.CategoryId, err = cmdutil.Int64PtrIfSet(opts.categoryID); err != nil {
		return cmdutil.ListResult{}, err
	}
	if params.ClassTypeId, err = cmdutil.Int64PtrIfSet(opts.classTypeID); err != nil {
		return cmdutil.ListResult{}, err
	}
	params.Page = cmdutil.Int64PtrIfPositive(opts.page)
	params.Limit = cmdutil.Int64PtrIfPositive(opts.limit)
	data, err := api.ParseResponseRaw(c.ListCourses(api.Ctx(), &params))
	if err != nil {
		return cmdutil.ListResult{}, err
	}
	return cmdutil.NewListResult(data, "data").FinalizeServerSide(opts.limit), nil
}

func courseListColumns() []output.Column {
	return []output.Column{
		{Header: "Code", Key: "code"},
		{Header: "Name", Key: "namePrimary"},
		{Header: "Level", Key: "educationLevel.name"},
		{Header: "JW ID", Key: "jwId"},
	}
}

func addCourseListFlags(cmd *cobra.Command, opts *courseListOpts) {
	cmd.Flags().StringVarP(&opts.search, "search", "s", "", "Search query")
	cmd.Flags().StringVar(&opts.educationLevelID, "education-level-id", "", "Education level ID")
	cmd.Flags().StringVar(&opts.categoryID, "category-id", "", "Category ID")
	cmd.Flags().StringVar(&opts.classTypeID, "class-type-id", "", "Class type ID")
	cmd.Flags().BoolVarP(&opts.interactive, "interactive", "i", false, "Open the interactive search form")
	cmd.Flags().BoolVar(&opts.noInteractive, "no-interactive", false, "Skip the interactive search form")
	cmdutil.AddListFlags(cmd, &opts.page, &opts.limit)
}

func newCmdList() *cobra.Command {
	var opts courseListOpts
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List courses",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCourseList(cmd, opts)
		},
	}
	addCourseListFlags(cmd, &opts)
	return cmd
}

func newCmdView() *cobra.Command {
	return &cobra.Command{
		Use:     "get <jw-id>",
		Aliases: []string{"show"},
		Short:   "View course details",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCourseView(cmd, args[0])
		},
	}
}

func runCourseView(cmd *cobra.Command, id string) error {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	jwID, err := cmdutil.Int64PtrIfSet(id)
	if err != nil {
		return err
	}
	data, err := api.ParseResponseRaw(c.GetCourse(api.Ctx(), *jwID, nil))
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
		{Key: "Level", Value: output.Resolve(m, "educationLevel.name")},
		{Key: "Category", Value: output.Resolve(m, "category.name")},
		{Key: "Class type", Value: output.Resolve(m, "classType.name")},
		{Key: "Gradation", Value: output.Resolve(m, "gradation.name")},
		{Key: "Course type", Value: output.Resolve(m, "type.name")},
	}, "Course")

	if sections, ok := m["sections"].([]any); ok && len(sections) > 0 {
		fmt.Println()
		output.Bold("  Sections")
		rows := cmdutil.RowsFromAny(sections)
		output.Table(rows, []output.Column{
			{Header: "ID", Key: "id"},
			{Header: "Code", Key: "code"},
			{Header: "Semester", Key: "semester.name"},
			{Header: "Campus", Key: "campus.name"},
		})
	}
	return nil
}
