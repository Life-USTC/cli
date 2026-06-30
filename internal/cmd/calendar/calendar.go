package calendar

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
)

func NewCmdCalendar() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "calendar [command]",
		Short: "Manage calendar subscriptions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCalendarGet(cmd)
		},
	}
	cmd.AddCommand(newCmdGet())
	cmd.AddCommand(newCmdQuery())
	cmd.AddCommand(newCmdAdd())
	cmd.AddCommand(newCmdRemove())
	cmd.AddCommand(newCmdSet())
	cmd.AddCommand(newCmdImportCodes())
	return cmd
}

func runCalendarGet(cmd *cobra.Command) error {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return err
	}
	data, err := api.ParseResponseRaw(c.GetCurrentCalendarSubscription(api.Ctx()))
	if err != nil {
		return err
	}
	if output.IsJSON() {
		return output.JSON(data)
	}

	// API returns {"subscription": {...}} — unwrap
	m := cmdutil.AsMap(data)
	sub, _ := m["subscription"].(map[string]any)
	if sub == nil {
		output.Dim("  No calendar subscription found.")
		return nil
	}

	calURL, _ := sub["calendarUrl"].(string)
	note, _ := sub["note"].(string)
	output.KVWithTitle([]output.KVPair{
		{Key: "URL", Value: output.Hyperlink(calURL, calURL)},
		{Key: "Note", Value: note},
	}, "Calendar subscription")

	if sections, ok := sub["sections"].([]any); ok && len(sections) > 0 {
		fmt.Println()
		output.Bold("  Sections")
		rows := cmdutil.RowsFromAny(sections)
		output.Table(rows, []output.Column{
			{Header: "ID", Key: "id"},
			{Header: "Code", Key: "code"},
			{Header: "Course", Key: "course.name"},
			{Header: "Semester", Key: "semester.name"},
		})
	}
	return nil
}

func newCmdGet() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Show your calendar subscription",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCalendarGet(cmd)
		},
	}
}

func newCmdSet() *cobra.Command {
	var semesterID string
	cmd := &cobra.Command{
		Use:   "set <section-id-or-code>...",
		Short: "Replace calendar section subscriptions",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCalendarBatch(cmd, args, openapi.CalendarSubscriptionBatchRequestSchemaActionSet, semesterID)
		},
	}
	cmd.Flags().StringVar(&semesterID, "semester-id", "", "Semester ID to narrow code matches")
	return cmd
}

func newCmdAdd() *cobra.Command {
	var semesterID string
	cmd := &cobra.Command{
		Use:   "add <section-id-or-code>...",
		Short: "Add calendar section subscriptions",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCalendarBatch(cmd, args, openapi.CalendarSubscriptionBatchRequestSchemaActionAdd, semesterID)
		},
	}
	cmd.Flags().StringVar(&semesterID, "semester-id", "", "Semester ID to narrow code matches")
	return cmd
}

func newCmdRemove() *cobra.Command {
	var semesterID string
	cmd := &cobra.Command{
		Use:   "remove <section-id-or-code>...",
		Short: "Remove calendar section subscriptions",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCalendarBatch(cmd, args, openapi.CalendarSubscriptionBatchRequestSchemaActionRemove, semesterID)
		},
	}
	cmd.Flags().StringVar(&semesterID, "semester-id", "", "Semester ID to narrow code matches")
	return cmd
}

func newCmdQuery() *cobra.Command {
	var semesterID string
	cmd := &cobra.Command{
		Use:   "query <section-id-or-code>...",
		Short: "Search sections for calendar subscription changes",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			sectionIds, codes, err := splitCalendarRefs(args)
			if err != nil {
				return err
			}
			body := openapi.QueryCalendarSubscriptionSectionsJSONRequestBody{}
			if len(sectionIds) > 0 {
				body.SectionIds = &sectionIds
			}
			if len(codes) > 0 {
				body.Codes = &codes
			}
			if semesterID != "" {
				semester := openapi.CalendarSubscriptionQueryRequestSchema_SemesterId{}
				_ = semester.FromCalendarSubscriptionQueryRequestSchemaSemesterId0(semesterID)
				body.SemesterId = &semester
			}
			data, err := api.ParseResponseRaw(c.QueryCalendarSubscriptionSections(api.Ctx(), body))
			if err != nil {
				return err
			}
			return outputCalendarMatchResult(data, "Matched")
		},
	}
	cmd.Flags().StringVar(&semesterID, "semester-id", "", "Semester ID to narrow code matches")
	return cmd
}

func newCmdImportCodes() *cobra.Command {
	var semesterID string
	cmd := &cobra.Command{
		Use:   "import-codes <code>...",
		Short: "Import section or course codes into calendar subscriptions",
		Long:  "Resolve section or course codes to Life@USTC sections and add them to your calendar subscriptions.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCalendarBatch(cmd, args, openapi.CalendarSubscriptionBatchRequestSchemaActionAdd, semesterID)
		},
	}
	cmd.Flags().StringVar(&semesterID, "semester-id", "", "Semester ID to narrow the match")
	return cmd
}

func runCalendarBatch(cmd *cobra.Command, args []string, action openapi.CalendarSubscriptionBatchRequestSchemaAction, semesterID string) error {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return err
	}
	sectionIds, codes, err := splitCalendarRefs(args)
	if err != nil {
		return err
	}
	body := openapi.BatchUpdateCalendarSubscriptionJSONRequestBody{
		Action: action,
	}
	if len(sectionIds) > 0 {
		body.SectionIds = &sectionIds
	}
	if len(codes) > 0 {
		body.Codes = &codes
	}
	if semesterID != "" {
		semester := openapi.CalendarSubscriptionBatchRequestSchema_SemesterId{}
		_ = semester.FromCalendarSubscriptionBatchRequestSchemaSemesterId0(semesterID)
		body.SemesterId = &semester
	}
	data, err := api.ParseResponseRaw(c.BatchUpdateCalendarSubscription(api.Ctx(), body))
	if err != nil {
		return err
	}
	return outputCalendarBatchResult(data)
}

func splitCalendarRefs(args []string) ([]int, []string, error) {
	sectionIds := make([]int, 0, len(args))
	codes := make([]string, 0, len(args))
	for _, arg := range args {
		value := strings.TrimSpace(arg)
		if value == "" {
			continue
		}
		id, err := strconv.Atoi(value)
		if err == nil {
			if id <= 0 {
				return nil, nil, fmt.Errorf("invalid section ID %q", arg)
			}
			sectionIds = append(sectionIds, id)
			continue
		}
		codes = append(codes, value)
	}
	return sectionIds, codes, nil
}

func outputCalendarBatchResult(data any) error {
	if output.IsJSON() {
		return output.JSON(data)
	}
	m := cmdutil.AsMap(data)
	output.KVWithTitle([]output.KVPair{
		{Key: "Action", Value: m["action"]},
		{Key: "Added", Value: m["addedCount"]},
		{Key: "Removed", Value: m["removedCount"]},
		{Key: "Unchanged", Value: m["unchangedCount"]},
	}, "Calendar subscriptions updated")
	fmt.Println()
	return outputCalendarMatchResult(data, "Resolved")
}

func outputCalendarMatchResult(data any, verb string) error {
	if output.IsJSON() {
		return output.JSON(data)
	}
	m := cmdutil.AsMap(data)
	matchedCodes, _ := m["matchedCodes"].([]any)
	matchedSectionIds, _ := m["matchedSectionIds"].([]any)
	unmatchedCodes, _ := m["unmatchedCodes"].([]any)
	unmatchedSectionIds, _ := m["unmatchedSectionIds"].([]any)
	output.Success(fmt.Sprintf("%s %d code(s) and %d section ID(s)", verb, len(matchedCodes), len(matchedSectionIds)))
	if len(unmatchedCodes) > 0 || len(unmatchedSectionIds) > 0 {
		output.Dim(fmt.Sprintf("  Unmatched: %d code(s), %d section ID(s)", len(unmatchedCodes), len(unmatchedSectionIds)))
	}
	sectionRows := cmdutil.RowsFromAny(m["sections"])
	if len(sectionRows) > 0 {
		output.Table(sectionRows, []output.Column{
			{Header: "ID", Key: "id"},
			{Header: "Code", Key: "code"},
			{Header: "Course", Key: "course.namePrimary"},
			{Header: "Semester", Key: "semester.nameCn"},
		})
	}
	return nil
}
