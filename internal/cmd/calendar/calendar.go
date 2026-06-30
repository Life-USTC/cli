package calendar

import (
	"fmt"
	"strconv"

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
	return &cobra.Command{
		Use:   "set <section-id>...",
		Short: "Set calendar section subscriptions",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			sectionIds := make([]int, len(args))
			for i, arg := range args {
				id, err := strconv.Atoi(arg)
				if err != nil {
					return fmt.Errorf("invalid section ID %q: %w", arg, err)
				}
				sectionIds[i] = id
			}
			body := openapi.SetCalendarSubscriptionJSONRequestBody{
				SectionIds: &sectionIds,
			}
			_, err = api.ParseResponseRaw(c.SetCalendarSubscription(api.Ctx(), body))
			if err != nil {
				return err
			}
			output.Success("Calendar subscriptions updated.")
			return nil
		},
	}
}

func newCmdImportCodes() *cobra.Command {
	var semesterID string
	cmd := &cobra.Command{
		Use:   "import-codes <code>...",
		Short: "Import section codes into calendar subscriptions",
		Long:  "Resolve section codes to Life@USTC sections and add them to your calendar subscriptions.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			body := openapi.PostApiCalendarSubscriptionsImportCodesJSONRequestBody{
				Codes: args,
			}
			if semesterID != "" {
				semester := openapi.MatchSectionCodesRequestSchema_SemesterId{}
				_ = semester.FromMatchSectionCodesRequestSchemaSemesterId0(semesterID)
				body.SemesterId = &semester
			}
			data, err := api.ParseResponseRaw(c.PostApiCalendarSubscriptionsImportCodes(api.Ctx(), body))
			if err != nil {
				return err
			}
			if output.IsJSON() {
				return output.JSON(data)
			}
			m := cmdutil.AsMap(data)
			matchedCodes, _ := m["matchedCodes"].([]any)
			sectionRows := cmdutil.RowsFromAny(m["sections"])
			output.Success(fmt.Sprintf("Matched %d section code(s)", len(matchedCodes)))
			if len(sectionRows) > 0 {
				output.Table(sectionRows, []output.Column{
					{Header: "ID", Key: "id"},
					{Header: "Code", Key: "code"},
					{Header: "Course", Key: "course.namePrimary"},
					{Header: "Semester", Key: "semester.name"},
				})
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&semesterID, "semester-id", "", "Semester ID to narrow the match")
	return cmd
}
