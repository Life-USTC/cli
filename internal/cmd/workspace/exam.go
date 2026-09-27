package workspace

import (
	"context"
	"fmt"
	"github.com/Life-USTC/CLI/internal/openapi"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/cmd/youngutil"
	"github.com/Life-USTC/CLI/internal/output"
)

type examOpts struct {
	dateFrom, dateTo           string
	semesterID, page, pageSize int
	includeDateUnknown         bool
}

func newCmdExam() *cobra.Command {
	opts := examOpts{includeDateUnknown: true}
	cmd := &cobra.Command{
		Use:   "exam",
		Short: "List exams from your subscribed sections",
		Long:  "List all exams from your subscribed sections across semesters, including past and unknown-date exams. By default every page is fetched. Use --page or --limit to request one page.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("semester-id") && opts.semesterID == 0 {
				return fmt.Errorf("--semester-id must be positive")
			}
			if cmd.Flags().Changed("page") && opts.page == 0 {
				return fmt.Errorf("--page must be positive")
			}
			if cmd.Flags().Changed("limit") && opts.pageSize == 0 {
				return fmt.Errorf("--limit must be positive")
			}
			params, err := buildExamParams(opts)
			if err != nil {
				return err
			}
			client, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			data, err := youngutil.FetchAllIfUnpaged(cmd.Context(), func(ctx context.Context, query url.Values) (any, error) { return fetchExamPage(ctx, client, query) }, "/api/workspace/exams", params, "data", 100, "id")
			if err != nil {
				return err
			}
			list := cmdutil.NewListResult(data, "data")
			return output.OutputList(list.Raw, list.Rows, []output.Column{
				{Header: "Course", Key: "section.course.namePrimary"},
				{Header: "Semester", Key: "section.semester.nameCn"},
				{Header: "Date", Key: "examDate"},
				{Header: "Start", Key: "startTime"},
				{Header: "End", Key: "endTime"},
				{Header: "Mode", Key: "examMode"},
			}, list.Total, list.Page)
		},
	}
	cmd.Flags().StringVar(&opts.dateFrom, "date-from", "", "Inclusive Shanghai calendar-date start")
	cmd.Flags().StringVar(&opts.dateTo, "date-to", "", "Inclusive Shanghai calendar-date end")
	cmd.Flags().IntVar(&opts.semesterID, "semester-id", 0, "Filter by semester ID")
	cmd.Flags().BoolVar(&opts.includeDateUnknown, "include-date-unknown", true, "Include exams whose dates are unknown")
	cmd.Flags().IntVarP(&opts.page, "page", "p", 0, "Fetch only this page")
	cmd.Flags().IntVarP(&opts.pageSize, "limit", "L", 0, "Fetch one page with this many exams (maximum 100)")
	return cmd
}

func buildExamParams(opts examOpts) (url.Values, error) {
	params, err := youngutil.PageParams(opts.page, opts.pageSize)
	if err != nil {
		return nil, err
	}
	if opts.pageSize > 100 {
		return nil, fmt.Errorf("--limit must not exceed 100")
	}
	if opts.semesterID < 0 {
		return nil, fmt.Errorf("--semester-id must be positive")
	}
	if opts.semesterID > 0 {
		params.Set("semesterId", strconv.Itoa(opts.semesterID))
	}
	if value := strings.TrimSpace(opts.dateFrom); value != "" {
		params.Set("dateFrom", value)
	}
	if value := strings.TrimSpace(opts.dateTo); value != "" {
		params.Set("dateTo", value)
	}
	params.Set("includeDateUnknown", strconv.FormatBool(opts.includeDateUnknown))
	return params, nil
}

func fetchExamPage(ctx context.Context, client *api.TypedClient, query url.Values) (any, error) {
	var paramIncludeDateUnknown *openapi.WorkspaceExamListParamsIncludeDateUnknown
	if value := query.Get("includeDateUnknown"); value != "" {
		converted := openapi.WorkspaceExamListParamsIncludeDateUnknown(value)
		paramIncludeDateUnknown = &converted
	}
	paramSemesterId, err := cmdutil.Int64PtrIfSet(query.Get("semesterId"))
	if err != nil {
		return nil, err
	}
	paramPage, err := cmdutil.Int64PtrIfSet(query.Get("page"))
	if err != nil {
		return nil, err
	}
	paramPageSize, err := cmdutil.Int64PtrIfSet(query.Get("pageSize"))
	if err != nil {
		return nil, err
	}
	var paramLocale *openapi.WorkspaceExamListParamsLocale
	if value := query.Get("locale"); value != "" {
		converted := openapi.WorkspaceExamListParamsLocale(value)
		paramLocale = &converted
	}
	params := openapi.WorkspaceExamListParams{DateFrom: cmdutil.StringPtrIfSet(query.Get("dateFrom")),
		DateTo:             cmdutil.StringPtrIfSet(query.Get("dateTo")),
		IncludeDateUnknown: paramIncludeDateUnknown,
		SemesterId:         paramSemesterId,
		Page:               paramPage,
		PageSize:           paramPageSize,
		Locale:             paramLocale}
	return api.ParseResponse[openapi.SubscribedExamsResponseSchema](client.WorkspaceExamList(ctx, &params))
}
