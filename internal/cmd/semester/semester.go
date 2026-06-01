package semester

import (
	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
	"github.com/Life-USTC/CLI/internal/timeutil"
)

// semesterDateOnly trims an ISO timestamp to a plain date string (YYYY-MM-DD).
func semesterDateOnly(m map[string]any) {
	for _, k := range []string{"startDate", "endDate"} {
		if s, ok := m[k].(string); ok && len(s) >= 10 {
			m[k] = timeutil.DateOnlyString(s)
		}
	}
}

type semesterListOpts struct {
	page, limit int
}

func NewCmdSemester() *cobra.Command {
	opts := semesterListOpts{}
	cmd := &cobra.Command{
		Use:   "semester [command]",
		Short: "Browse semesters",
		Long:  "List and inspect academic semesters.",
		Example: `  # List all semesters
  life-ustc semester

  # Show the current semester
  life-ustc semester current`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSemesterList(cmd, opts)
		},
	}
	cmdutil.AddListFlags(cmd, &opts.page, &opts.limit)
	cmd.AddCommand(newCmdList())
	cmd.AddCommand(newCmdCurrent())
	return cmd
}

func runSemesterList(cmd *cobra.Command, opts semesterListOpts) error {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	params := &openapi.ListSemestersParams{}
	params.Page = cmdutil.Int64PtrIfPositive(opts.page)
	params.Limit = cmdutil.Int64PtrIfPositive(opts.limit)
	data, err := api.ParseResponseRaw(c.ListSemesters(api.Ctx(), params))
	if err != nil {
		return err
	}
	list := cmdutil.NewListResult(data, "data").FinalizeServerSide(opts.limit)
	for _, row := range list.Rows {
		semesterDateOnly(row)
	}
	return output.OutputList(list.Raw, list.Rows, []output.Column{
		{Header: "Name", Key: "nameCn"},
		{Header: "Code", Key: "code"},
		{Header: "Start", Key: "startDate"},
		{Header: "End", Key: "endDate"},
		{Header: "ID", Key: "id"},
	}, list.Total, list.Page)
}

func newCmdList() *cobra.Command {
	opts := semesterListOpts{}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List semesters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSemesterList(cmd, opts)
		},
	}
	cmdutil.AddListFlags(cmd, &opts.page, &opts.limit)
	return cmd
}

func newCmdCurrent() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show current semester",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}
			data, err := api.ParseResponseRaw(c.GetCurrentSemester(api.Ctx()))
			if err != nil {
				return err
			}
			if m, ok := data.(map[string]any); ok {
				semesterDateOnly(m)
			}
			return output.OutputDetail(data, []output.FieldDef{
				{Key: "id", Label: "ID"},
				{Key: "code", Label: "Code"},
				{Key: "nameCn", Label: "Name"},
				{Key: "startDate", Label: "Start"},
				{Key: "endDate", Label: "End"},
			}, "Current semester")
		},
	}
}
