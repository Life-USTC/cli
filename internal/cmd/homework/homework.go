package homework

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
	"github.com/Life-USTC/CLI/internal/timeutil"
)

// NewCmdSectionHomework returns homework commands scoped to a section.
// list and create take section-id as a positional argument.
func NewCmdSectionHomework() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "homework <command>",
		Short: "Manage section homeworks",
		Long:  "List, create, update, and delete homeworks for a course section.",
		Example: `  # List homeworks for a section
  life-ustc section homework list <section-id>

  # Create a homework
  life-ustc section homework create <section-id> --title "Problem Set 1"

  # Delete a homework (omit ID to pick interactively)
  life-ustc section homework delete
  life-ustc section homework delete <homework-id>`,
	}
	cmd.AddCommand(newCmdSectionList())
	cmd.AddCommand(newCmdSectionCreate())
	cmd.AddCommand(newCmdUpdate())
	cmd.AddCommand(newCmdDelete())
	return cmd
}

func newCmdSectionList() *cobra.Command {
	var (
		includeDeleted bool
		page, limit    int
	)
	cmd := &cobra.Command{
		Use:     "list <section-id>",
		Aliases: []string{"ls"},
		Short:   "List homeworks for a section",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}
			inclDel := openapi.ListHomeworksParamsIncludeDeleted("false")
			if includeDeleted {
				inclDel = openapi.ListHomeworksParamsIncludeDeletedTrue
			}
			sectionID, err := cmdutil.Int64PtrIfSet(args[0])
			if err != nil {
				return err
			}
			params := &openapi.ListHomeworksParams{
				SectionId:      sectionID,
				IncludeDeleted: &inclDel,
			}
			data, err := api.ParseResponseRaw(c.ListHomeworks(api.Ctx(), params))
			if err != nil {
				return err
			}
			list, err := cmdutil.NewListResult(data, "homeworks").FinalizeClientSide(page, limit)
			if err != nil {
				return err
			}
			return output.OutputList(list.Raw, list.Rows, []output.Column{
				{Header: "Title", Key: "title"},
				{Header: "Due", Key: "submissionDueAt"},
				{Header: "Major", Key: "isMajor"},
				{Header: "Action ID", Key: "id"},
			}, list.Total, list.Page)
		},
	}
	cmd.Flags().BoolVar(&includeDeleted, "include-deleted", false, "Include deleted")
	cmdutil.AddListFlags(cmd, &page, &limit)
	return cmd
}

func newCmdSectionCreate() *cobra.Command {
	var (
		title, desc, publishedAt, submissionStart, submissionDue string
		major, requiresTeam                                      bool
	)
	cmd := &cobra.Command{
		Use:     "create [section-id]",
		Aliases: []string{"new"},
		Short:   "Create a homework for a section",
		Long:    "Create a homework for a section. When run interactively without a section ID, lets you choose from your subscribed sections.",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sectionID := ""
			if len(args) == 1 {
				sectionID = args[0]
			} else {
				if !cmdutil.IsInteractive() {
					return fmt.Errorf("section id is required in non-interactive mode")
				}
				var err error
				sectionID, err = promptSubscribedSectionID(cmd)
				if err != nil {
					return err
				}
				if sectionID == "" {
					return nil
				}
			}
			if title == "" {
				if !cmdutil.IsInteractive() {
					return fmt.Errorf("--title is required in non-interactive mode")
				}
				output.Panel("New homework", "Add homework to section "+sectionID+".", "Optional details can be left blank.")
				title = cmdutil.PromptText("Homework title")
				if desc == "" {
					desc = cmdutil.PromptText("Description (optional)")
				}
				if submissionDue == "" {
					submissionDue = cmdutil.PromptText("Submission due (optional, YYYY-MM-DD or RFC3339)")
				}
			}
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			sectionId := openapi.HomeworkCreateRequestSchema_0_SectionId{}
			_ = sectionId.FromHomeworkCreateRequestSchema0SectionId0(sectionID)
			schemaBody := openapi.HomeworkCreateRequestSchema0{
				SectionId: sectionId,
				Title:     title,
			}
			if desc != "" {
				schemaBody.Description = &desc
			}
			if publishedAt != "" {
				publishedAtUnion := openapi.HomeworkCreateRequestSchema_0_PublishedAt{}
				_ = publishedAtUnion.FromHomeworkCreateRequestSchema0PublishedAt0(publishedAt)
				schemaBody.PublishedAt = &publishedAtUnion
			}
			if submissionStart != "" {
				submissionStartUnion := openapi.HomeworkCreateRequestSchema_0_SubmissionStartAt{}
				_ = submissionStartUnion.FromHomeworkCreateRequestSchema0SubmissionStartAt0(submissionStart)
				schemaBody.SubmissionStartAt = &submissionStartUnion
			}
			if submissionDue != "" {
				submissionDueUnion := openapi.HomeworkCreateRequestSchema_0_SubmissionDueAt{}
				_ = submissionDueUnion.FromHomeworkCreateRequestSchema0SubmissionDueAt0(submissionDue)
				schemaBody.SubmissionDueAt = &submissionDueUnion
			}
			if major {
				schemaBody.IsMajor = &major
			}
			if requiresTeam {
				schemaBody.RequiresTeam = &requiresTeam
			}
			body := openapi.CreateHomeworkJSONRequestBody{}
			_ = body.FromHomeworkCreateRequestSchema0(schemaBody)
			data, err := api.ParseResponseRaw(c.CreateHomework(api.Ctx(), body))
			if err != nil {
				return err
			}
			m := cmdutil.AsMap(data)
			id, _ := m["id"].(string)
			output.Success(fmt.Sprintf("Created homework %s: %s", id, title))
			return nil
		},
	}
	cmd.Flags().StringVarP(&title, "title", "t", "", "Title")
	cmd.Flags().StringVar(&desc, "description", "", "Description")
	cmd.Flags().StringVar(&publishedAt, "published-at", "", "Publish date (ISO 8601)")
	cmd.Flags().StringVar(&submissionStart, "submission-start", "", "Submission start date")
	cmd.Flags().StringVar(&submissionDue, "submission-due", "", "Submission due date")
	cmd.Flags().BoolVar(&major, "major", false, "Major assignment")
	cmd.Flags().BoolVar(&requiresTeam, "requires-team", false, "Requires a team submission")
	return cmd
}

func promptSubscribedSectionID(cmd *cobra.Command) (string, error) {
	sections, err := loadSubscribedSections(cmd)
	if err != nil {
		return "", err
	}
	if len(sections) == 0 {
		output.Panel("Select section", "No subscribed sections were found.", "Enter a section ID manually.")
		id := cmdutil.PromptText("Section ID")
		if id == "" {
			output.Warning("Cancelled.")
		}
		return id, nil
	}

	options := make([]string, 0, len(sections))
	byLabel := make(map[string]string, len(sections))
	for _, section := range sections {
		id := fmt.Sprint(output.Resolve(section, "id"))
		label := sectionChoiceLabel(section)
		options = append(options, label)
		byLabel[label] = id
	}
	choice := cmdutil.PromptSelect("Choose a subscribed section", options)
	if id := byLabel[choice]; id != "" {
		return id, nil
	}
	return choice, nil
}

func loadSubscribedSections(cmd *cobra.Command) ([]map[string]any, error) {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return nil, err
	}
	data, err := api.ParseResponseRaw(c.GetCurrentCalendarSubscription(api.Ctx()))
	if err == nil {
		m := cmdutil.AsMap(data)
		sub, _ := m["subscription"].(map[string]any)
		if sections, ok := sub["sections"].([]any); ok {
			rows := cmdutil.RowsFromAny(sections)
			if len(rows) > 0 {
				return rows, nil
			}
		}
	}

	data, err = api.ParseResponseRaw(c.GetSubscribedHomeworks(api.Ctx()))
	if err != nil {
		return nil, fmt.Errorf("could not load subscribed sections: %w", err)
	}
	rows := cmdutil.NewListResult(data, "homeworks").Rows
	seen := map[string]bool{}
	sections := make([]map[string]any, 0)
	for _, row := range rows {
		section, _ := row["section"].(map[string]any)
		if section == nil {
			continue
		}
		id := fmt.Sprint(output.Resolve(section, "id"))
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		sections = append(sections, section)
	}
	return sections, nil
}

func sectionChoiceLabel(section map[string]any) string {
	resolveStr := func(key string) string {
		v := output.Resolve(section, key)
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
	id := resolveStr("id")
	code := resolveStr("code")
	course := resolveStr("course.namePrimary")
	semester := resolveStr("semester.name")
	parts := []string{}
	if code != "" {
		parts = append(parts, code)
	}
	if course != "" {
		parts = append(parts, course)
	}
	if semester != "" {
		parts = append(parts, semester)
	}
	if id != "" {
		parts = append(parts, "#"+id)
	}
	if len(parts) == 0 {
		return "Section #" + id
	}
	return strings.Join(parts, " · ")
}

type myHomeworkListOpts struct {
	sectionID string
	done      bool
	pending   bool
	before    string
	after     string
	page      int
	limit     int
}

// NewCmdMyHomework returns personal homework commands (list + complete).
func NewCmdMyHomework() *cobra.Command {
	var opts myHomeworkListOpts
	cmd := &cobra.Command{
		Use:   "homework [command]",
		Short: "View and manage your homeworks",
		Long:  "List your assigned homeworks and mark them as complete.\nWhen no --section-id is given, aggregates homework from all your subscribed sections.",
		Example: `  # List all your homeworks (from subscribed sections)
  life-ustc homework

  # Show only pending homeworks
  life-ustc homework --pending

  # Filter to a specific section
  life-ustc homework list --section-id <id>

  # Add homework to a section
  life-ustc homework create <section-id> --title "Problem Set 1"

  # Mark a homework as done (omit ID to pick interactively)
  life-ustc homework done
  life-ustc homework done <homework-id>`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMyHomeworkList(cmd, opts)
		},
	}
	addMyHomeworkListFlags(cmd, &opts)
	cmd.AddCommand(newCmdMyList())
	cmd.AddCommand(newCmdSectionCreate())
	cmd.AddCommand(newCmdComplete())
	return cmd
}

func newCmdMyList() *cobra.Command {
	var opts myHomeworkListOpts
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List your homeworks",
		Example: `  life-ustc homework list
  life-ustc homework list --section-id <id>
  life-ustc homework list --pending
  life-ustc homework list --before 2025-06-01`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMyHomeworkList(cmd, opts)
		},
	}
	addMyHomeworkListFlags(cmd, &opts)
	return cmd
}

func addMyHomeworkListFlags(cmd *cobra.Command, opts *myHomeworkListOpts) {
	cmd.Flags().StringVar(&opts.sectionID, "section-id", "", "Section ID")
	cmd.Flags().BoolVar(&opts.done, "done", false, "Show only completed homeworks")
	cmd.Flags().BoolVar(&opts.pending, "pending", false, "Show only pending homeworks")
	cmd.Flags().StringVar(&opts.before, "before", "", "Show homeworks due before this date (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().StringVar(&opts.after, "after", "", "Show homeworks due after this date (RFC3339 or YYYY-MM-DD)")
	cmdutil.AddListFlags(cmd, &opts.page, &opts.limit)
}

func runMyHomeworkList(cmd *cobra.Command, opts myHomeworkListOpts) error {
	if opts.done && opts.pending {
		return fmt.Errorf("--done and --pending cannot be used together")
	}
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return err
	}

	var data any

	if opts.sectionID != "" {
		// Single section — use /api/homeworks with sectionId filter
		sectionID, err := cmdutil.Int64PtrIfSet(opts.sectionID)
		if err != nil {
			return err
		}
		params := &openapi.ListHomeworksParams{
			SectionId: sectionID,
		}
		data, err = api.ParseResponseRaw(c.ListHomeworks(api.Ctx(), params))
		if err != nil {
			return err
		}
	} else {
		// All subscribed sections — use the combined endpoint
		data, err = api.ParseResponseRaw(c.GetSubscribedHomeworks(api.Ctx()))
		if err != nil {
			return err
		}
	}

	list := cmdutil.NewListResult(data, "homeworks")
	rows, err := filterHomeworkRows(list.Rows, opts)
	if err != nil {
		return err
	}
	list, err = list.WithRows(rows, len(rows), 0).FinalizeClientSide(opts.page, opts.limit)
	if err != nil {
		return err
	}
	data = list.Raw
	rows = list.Rows

	if output.IsJSON() {
		return output.JSON(data)
	}

	annotateHomeworkRows(rows)

	if len(rows) == 0 {
		output.Dim("  No homeworks found.")
		if opts.done || opts.pending || opts.before != "" || opts.after != "" {
			output.Hint("try adjusting your filters, or run without filters to see all items")
		}
		return nil
	}

	output.Dim(fmt.Sprintf("  %d homework(s)", len(rows)))
	output.Table(rows, []output.Column{
		{Header: "Done", Key: "_done"},
		{Header: "Course", Key: "section.course.namePrimary"},
		{Header: "Title", Key: "title"},
		{Header: "Due", Key: "_due"},
		{Header: "Left", Key: "_timeLeft"},
		{Header: "ID", Key: "id"},
	})
	return nil
}

func annotateHomeworkRows(rows []map[string]any) {
	for _, row := range rows {
		completed := homeworkCompleted(row)

		dueStr, _ := row["submissionDueAt"].(string)
		due, hasDue := timeutil.ParseAPI(dueStr)
		due = due.In(time.Local)

		dueFmt := color.New(color.Faint).Sprint("-")
		timeLeft := color.New(color.Faint).Sprint("-")
		if hasDue {
			dueFmt = due.Format("2006-01-02 15:04")
		}

		doneStr := color.New(color.Faint).Sprint("✗")
		if completed {
			doneStr = color.GreenString("✓")
			if hasDue {
				dueFmt = color.New(color.Faint).Sprint(dueFmt)
			}
		} else if hasDue {
			remaining := time.Until(due)
			rel := output.FormatRelativeTime(dueStr)
			switch {
			case remaining < 0:
				doneStr = color.RedString("✗")
				dueFmt = color.RedString(dueFmt)
				timeLeft = color.RedString(rel)
			case remaining < 24*time.Hour:
				doneStr = color.YellowString("✗")
				dueFmt = color.YellowString(dueFmt)
				timeLeft = color.YellowString(rel)
			default:
				timeLeft = rel
			}
		}

		row["_done"] = doneStr
		row["_due"] = dueFmt
		row["_timeLeft"] = timeLeft
	}
}

func homeworkCompleted(row map[string]any) bool {
	if v, ok := row["isCompleted"].(bool); ok {
		return v
	}
	completed := row["completion"] != nil
	row["isCompleted"] = completed
	return completed
}

func filterHomeworkRows(rows []map[string]any, opts myHomeworkListOpts) ([]map[string]any, error) {
	var beforeTime, afterTime *time.Time
	if opts.before != "" {
		t, err := timeutil.ParseUserDateTime(opts.before, true)
		if err != nil {
			return nil, fmt.Errorf("invalid --before date: %w", err)
		}
		beforeTime = &t
	}
	if opts.after != "" {
		t, err := timeutil.ParseUserDateTime(opts.after, false)
		if err != nil {
			return nil, fmt.Errorf("invalid --after date: %w", err)
		}
		afterTime = &t
	}

	if !opts.done && !opts.pending && beforeTime == nil && afterTime == nil {
		return rows, nil
	}

	filtered := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		completed := homeworkCompleted(row)
		if opts.done && !completed {
			continue
		}
		if opts.pending && completed {
			continue
		}
		if beforeTime != nil || afterTime != nil {
			dueRaw, _ := row["submissionDueAt"].(string)
			if dueRaw == "" {
				continue
			}
			due, ok := timeutil.ParseAPI(dueRaw)
			if !ok {
				continue
			}
			if beforeTime != nil && due.After(*beforeTime) {
				continue
			}
			if afterTime != nil && due.Before(*afterTime) {
				continue
			}
		}
		filtered = append(filtered, row)
	}
	return filtered, nil
}

// homeworkPickColumns returns the columns shown when picking a homework interactively.
func homeworkPickColumns() []output.Column {
	return []output.Column{
		{Header: "Done", Key: "_done"},
		{Header: "Course", Key: "section.course.namePrimary"},
		{Header: "Title", Key: "title"},
		{Header: "Due", Key: "_due"},
		{Header: "ID", Key: "id"},
	}
}

// fetchHomeworkPickList loads a filtered homework list for interactive picking.
func fetchHomeworkPickList(cmd *cobra.Command, opts myHomeworkListOpts) ([]map[string]any, error) {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return nil, err
	}
	data, err := api.ParseResponseRaw(c.GetSubscribedHomeworks(api.Ctx()))
	if err != nil {
		return nil, err
	}
	list := cmdutil.NewListResult(data, "homeworks")
	rows, err := filterHomeworkRows(list.Rows, opts)
	if err != nil {
		return nil, err
	}
	list, err = list.WithRows(rows, len(rows), 0).FinalizeClientSide(0, opts.limit)
	if err != nil {
		return nil, err
	}
	annotateHomeworkRows(list.Rows)
	return list.Rows, nil
}

// promptHomeworkPick lets the user pick a homework from a filtered list.
func promptHomeworkPick(cmd *cobra.Command, opts myHomeworkListOpts, prompt string) (map[string]any, error) {
	rows, err := fetchHomeworkPickList(cmd, opts)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		if opts.done {
			output.Dim("  No completed homeworks found.")
		} else if opts.pending {
			output.Dim("  No pending homeworks found.")
		} else {
			output.Dim("  No homeworks found.")
		}
		return nil, nil
	}
	return cmdutil.PromptPick(rows, homeworkPickColumns(), "id", prompt)
}

// homeworkTitleFromRow returns a short summary for success messages.
func homeworkTitleFromRow(row map[string]any) string {
	title, _ := row["title"].(string)
	if title != "" {
		return title
	}
	id, _ := row["id"].(string)
	return id
}

func newCmdUpdate() *cobra.Command {
	var (
		title, publishedAt, submissionStart, submissionDue string
		major, notMajor, requiresTeam, noTeam              bool
	)
	cmd := &cobra.Command{
		Use:   "update [homework-id]",
		Short: "Update a homework",
		Long:  "Update a homework. When run interactively without an ID, shows homeworks and lets you pick one.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			if len(args) == 1 {
				id = strings.TrimSpace(args[0])
			}
			if id == "" {
				if !cmdutil.IsInteractive() {
					return fmt.Errorf("homework id is required in non-interactive mode")
				}
				picked, err := promptHomeworkPick(cmd, myHomeworkListOpts{limit: 20}, "Pick a homework to update")
				if err != nil {
					return err
				}
				if picked == nil {
					return nil
				}
				id, _ = picked["id"].(string)
			}
			if major && notMajor {
				return fmt.Errorf("--major and --not-major cannot be used together")
			}
			if requiresTeam && noTeam {
				return fmt.Errorf("--requires-team and --no-team cannot be used together")
			}
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			body := openapi.UpdateHomeworkJSONRequestBody{}
			hasUpdate := false
			if title != "" {
				body.Title = &title
				hasUpdate = true
			}
			if publishedAt != "" {
				publishedAtUnion := openapi.HomeworkUpdateRequestSchema_PublishedAt{}
				_ = publishedAtUnion.FromHomeworkUpdateRequestSchemaPublishedAt0(publishedAt)
				body.PublishedAt = &publishedAtUnion
				hasUpdate = true
			}
			if submissionStart != "" {
				submissionStartUnion := openapi.HomeworkUpdateRequestSchema_SubmissionStartAt{}
				_ = submissionStartUnion.FromHomeworkUpdateRequestSchemaSubmissionStartAt0(submissionStart)
				body.SubmissionStartAt = &submissionStartUnion
				hasUpdate = true
			}
			if submissionDue != "" {
				submissionDueUnion := openapi.HomeworkUpdateRequestSchema_SubmissionDueAt{}
				_ = submissionDueUnion.FromHomeworkUpdateRequestSchemaSubmissionDueAt0(submissionDue)
				body.SubmissionDueAt = &submissionDueUnion
				hasUpdate = true
			}
			if major {
				t := true
				body.IsMajor = &t
				hasUpdate = true
			}
			if notMajor {
				f := false
				body.IsMajor = &f
				hasUpdate = true
			}
			if requiresTeam {
				t := true
				body.RequiresTeam = &t
				hasUpdate = true
			}
			if noTeam {
				f := false
				body.RequiresTeam = &f
				hasUpdate = true
			}
			if !hasUpdate {
				return fmt.Errorf("nothing to update — specify at least one flag")
			}
			_, err = api.ParseResponseRaw(c.UpdateHomework(api.Ctx(), id, body))
			if err != nil {
				return err
			}
			output.Success("Homework updated.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&title, "title", "t", "", "Title")
	cmd.Flags().BoolVar(&major, "major", false, "Mark as major")
	cmd.Flags().BoolVar(&notMajor, "not-major", false, "Mark as not major")
	cmd.Flags().StringVar(&publishedAt, "published-at", "", "Publish date")
	cmd.Flags().StringVar(&submissionStart, "submission-start", "", "Submission start")
	cmd.Flags().StringVar(&submissionDue, "submission-due", "", "Submission due")
	cmd.Flags().BoolVar(&requiresTeam, "requires-team", false, "Mark as requiring team")
	cmd.Flags().BoolVar(&noTeam, "no-team", false, "Mark as not requiring team")
	return cmd
}

func newCmdDelete() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "delete [homework-id]",
		Aliases: []string{"rm"},
		Short:   "Delete a homework (soft delete)",
		Long:    "Delete a homework. When run interactively without an ID, shows homeworks and lets you pick one.",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			var row map[string]any
			if len(args) == 1 {
				id = strings.TrimSpace(args[0])
			}
			if id == "" {
				if !cmdutil.IsInteractive() {
					return fmt.Errorf("homework id is required in non-interactive mode")
				}
				picked, err := promptHomeworkPick(cmd, myHomeworkListOpts{limit: 20}, "Pick a homework to delete")
				if err != nil {
					return err
				}
				if picked == nil {
					return nil
				}
				id, _ = picked["id"].(string)
				row = picked
			}
			label := homeworkTitleFromRow(row)
			if !cmdutil.Confirm(fmt.Sprintf("Delete %s?", label), yes) {
				return nil
			}
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			_, err = api.ParseResponseRaw(c.DeleteHomework(api.Ctx(), id))
			if err != nil {
				return err
			}
			output.Success(fmt.Sprintf("Deleted: %s", label))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation")
	return cmd
}

func newCmdComplete() *cobra.Command {
	var undo bool
	cmd := &cobra.Command{
		Use:     "done [homework-id]",
		Aliases: []string{"complete", "finish"},
		Short:   "Mark homework as complete",
		Long:    "Mark homework as complete. When run interactively without an ID, shows matching homeworks and lets you pick one.",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			var row map[string]any
			if len(args) == 1 {
				id = strings.TrimSpace(args[0])
			}
			if id == "" {
				if !cmdutil.IsInteractive() {
					return fmt.Errorf("homework id is required in non-interactive mode")
				}
				opts := myHomeworkListOpts{limit: 20}
				if undo {
					opts.done = true
				} else {
					opts.pending = true
				}
				picked, err := promptHomeworkPick(cmd, opts, "Pick a homework to "+cmdutil.DoneVerb(undo))
				if err != nil {
					return err
				}
				if picked == nil {
					return nil
				}
				id, _ = picked["id"].(string)
				row = picked
			}
			return setHomeworkCompleted(cmd, id, row, !undo)
		},
	}
	cmd.Flags().BoolVar(&undo, "undo", false, "Mark as not completed")
	return cmd
}

func setHomeworkCompleted(cmd *cobra.Command, id string, row map[string]any, completed bool) error {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return err
	}
	body := openapi.SetHomeworkCompletionJSONRequestBody{Completed: completed}
	_, err = api.ParseResponseRaw(c.SetHomeworkCompletion(api.Ctx(), id, body))
	if err != nil {
		return err
	}
	label := homeworkTitleFromRow(row)
	if completed {
		output.Success(fmt.Sprintf("Completed: %s", label))
	} else {
		output.Success(fmt.Sprintf("Reopened: %s", label))
	}
	return nil
}
