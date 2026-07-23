package todo

import (
	"fmt"
	"sort"
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

func NewCmdTodo() *cobra.Command {
	var opts todoListOpts
	cmd := &cobra.Command{
		Use:   "todo [command]",
		Short: "Manage personal todos",
		Long:  "Create, list, update, and delete personal todo items.",
		Example: `  # List all pending todos
  life-ustc workspace todo --pending

  # Create a new todo
  life-ustc workspace todo create --title "Review notes" --priority high --due 2025-06-01

  # Mark a todo as done (omit ID to pick interactively)
  life-ustc workspace todo complete
  life-ustc workspace todo complete <id>

  # Delete a todo (omit ID to pick interactively)
  life-ustc workspace todo delete
  life-ustc workspace todo delete <id>

  # Get todo IDs for scripting
  life-ustc workspace todo --jq '.todos[].id'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTodoList(cmd, opts)
		},
	}
	addTodoListFlags(cmd, &opts)
	cmd.AddCommand(newCmdList())
	cmd.AddCommand(newCmdCreate())
	cmd.AddCommand(newCmdCompletion(true))
	cmd.AddCommand(newCmdCompletion(false))
	cmd.AddCommand(newCmdUpdate())
	cmd.AddCommand(newCmdDelete())
	return cmd
}

type todoListOpts struct {
	done     bool
	pending  bool
	priority string
	before   string
	after    string
	sort     string
	page     int
	limit    int
}

func runTodoList(cmd *cobra.Command, opts todoListOpts) error {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return err
	}

	// Build server-side filter params
	params := &openapi.ListTodosParams{}
	if opts.done && opts.pending {
		return fmt.Errorf("--done and --pending cannot be used together")
	}
	if opts.done {
		v := openapi.True
		params.Completed = &v
	}
	if opts.pending {
		v := openapi.False
		params.Completed = &v
	}
	if opts.priority != "" {
		if !validTodoPriority(opts.priority) {
			return fmt.Errorf("invalid --priority %q (use low, medium, or high)", opts.priority)
		}
		v := openapi.ListTodosParamsPriority(opts.priority)
		params.Priority = &v
	}
	if opts.before != "" {
		t, err := timeutil.ParseUserDateTime(opts.before, true)
		if err != nil {
			return fmt.Errorf("invalid --before date: %w", err)
		}
		s := t.Format(time.RFC3339)
		params.DueBefore = &s
	}
	if opts.after != "" {
		t, err := timeutil.ParseUserDateTime(opts.after, false)
		if err != nil {
			return fmt.Errorf("invalid --after date: %w", err)
		}
		s := t.Format(time.RFC3339)
		params.DueAfter = &s
	}

	data, err := api.ParseResponseRaw(c.ListTodos(api.Ctx(), params))
	if err != nil {
		return err
	}
	list := cmdutil.NewListResult(data, "todos")
	list, err = applyTodoListOptions(list, opts)
	if err != nil {
		return err
	}
	if output.IsJSON() {
		return output.OutputList(list.Raw, list.Rows, nil, list.Total, list.Page)
	}

	annotateTodoRows(list.Rows)
	return output.OutputList(list.Raw, list.Rows, []output.Column{
		{Header: "Done", Key: "_done"},
		{Header: "Title", Key: "title"},
		{Header: "Priority", Key: "_priority"},
		{Header: "Due", Key: "_due"},
		{Header: "Left", Key: "_timeLeft"},
		{Header: "ID", Key: "id"},
	}, list.Total, list.Page)
}

func applyTodoListOptions(list cmdutil.ListResult, opts todoListOpts) (cmdutil.ListResult, error) {
	rows, err := sortTodoRows(list.Rows, opts.sort)
	if err != nil {
		return cmdutil.ListResult{}, err
	}
	return list.WithRows(rows, list.Total, list.Page).FinalizeClientSide(opts.page, opts.limit)
}

func annotateTodoRows(rows []map[string]any) {
	faint := color.New(color.Faint)
	for _, row := range rows {
		completed, _ := row["completed"].(bool)
		dueStr, _ := row["dueAt"].(string)
		priority, _ := row["priority"].(string)

		// _done: ✓/✗ colored by completion + urgency
		dueTime, hasDue := todoDueTime(row)
		if completed {
			row["_done"] = faint.Sprint("✓")
		} else if hasDue && time.Until(dueTime) < 0 {
			row["_done"] = color.RedString("✗")
		} else if hasDue && time.Until(dueTime) < 24*time.Hour {
			row["_done"] = color.YellowString("✗")
		} else {
			row["_done"] = faint.Sprint("✗")
		}

		// _priority: colored by level
		switch priority {
		case "high":
			row["_priority"] = color.RedString("high")
		case "medium":
			row["_priority"] = color.YellowString("medium")
		case "low":
			row["_priority"] = faint.Sprint("low")
		default:
			row["_priority"] = faint.Sprint("-")
		}

		// _due and _timeLeft: colored by urgency
		if hasDue {
			dueFormatted := dueTime.Local().Format("2006-01-02 15:04")
			timeLeft := output.FormatRelativeTime(dueStr)
			switch {
			case completed:
				row["_due"] = faint.Sprint(dueFormatted)
				row["_timeLeft"] = faint.Sprint("-")
			case time.Until(dueTime) < 0:
				row["_due"] = color.RedString(dueFormatted)
				row["_timeLeft"] = color.RedString(timeLeft)
			case time.Until(dueTime) < 24*time.Hour:
				row["_due"] = color.YellowString(dueFormatted)
				row["_timeLeft"] = color.YellowString(timeLeft)
			default:
				row["_due"] = dueFormatted
				row["_timeLeft"] = timeLeft
			}
		} else {
			row["_due"] = faint.Sprint("-")
			row["_timeLeft"] = faint.Sprint("-")
		}
	}
}

func sortTodoRows(rows []map[string]any, sortKey string) ([]map[string]any, error) {
	switch sortKey {
	case "":
		return rows, nil
	case "created":
		sort.SliceStable(rows, func(i, j int) bool { return stringValue(rows[i], "createdAt") < stringValue(rows[j], "createdAt") })
	case "due":
		sort.SliceStable(rows, func(i, j int) bool {
			left, leftOK := todoDueTime(rows[i])
			right, rightOK := todoDueTime(rows[j])
			switch {
			case leftOK && rightOK:
				return left.Before(right)
			case leftOK != rightOK:
				return leftOK
			default:
				return false
			}
		})
	case "priority":
		sort.SliceStable(rows, func(i, j int) bool {
			return priorityRank(stringValue(rows[i], "priority")) > priorityRank(stringValue(rows[j], "priority"))
		})
	default:
		return nil, fmt.Errorf("invalid --sort %q (use created, due, or priority)", sortKey)
	}
	return rows, nil
}

func todoDueTime(row map[string]any) (time.Time, bool) {
	dueStr, _ := row["dueAt"].(string)
	if dueStr == "" {
		return time.Time{}, false
	}
	dueTime, ok := timeutil.ParseAPI(dueStr)
	if !ok {
		return time.Time{}, false
	}
	return dueTime, true
}

func validTodoPriority(priority string) bool {
	switch priority {
	case "low", "medium", "high":
		return true
	default:
		return false
	}
}

func priorityRank(priority string) int {
	switch priority {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func stringValue(row map[string]any, key string) string {
	if v, ok := row[key].(string); ok {
		return v
	}
	return ""
}

func addTodoListFlags(cmd *cobra.Command, opts *todoListOpts) {
	cmd.Flags().BoolVar(&opts.done, "done", false, "Show only completed todos")
	cmd.Flags().BoolVar(&opts.pending, "pending", false, "Show only pending todos")
	cmd.Flags().StringVar(&opts.priority, "priority", "", "Filter by priority (low, medium, high)")
	cmd.Flags().StringVar(&opts.before, "before", "", "Show todos due before this date (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().StringVar(&opts.after, "after", "", "Show todos due after this date (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().StringVar(&opts.sort, "sort", "", "Sort by field (created, due, priority)")
	cmdutil.AddListFlags(cmd, &opts.page, &opts.limit)
}

// todoPickColumns returns the columns shown when picking a todo interactively.
func todoPickColumns() []output.Column {
	return []output.Column{
		{Header: "Done", Key: "_done"},
		{Header: "Title", Key: "title"},
		{Header: "Priority", Key: "_priority"},
		{Header: "Due", Key: "_due"},
		{Header: "ID", Key: "id"},
	}
}

// fetchTodoPickList loads a filtered todo list for interactive picking.
func fetchTodoPickList(cmd *cobra.Command, opts todoListOpts) ([]map[string]any, error) {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return nil, err
	}
	params := &openapi.ListTodosParams{}
	if opts.done {
		v := openapi.True
		params.Completed = &v
	}
	if opts.pending {
		v := openapi.False
		params.Completed = &v
	}
	data, err := api.ParseResponseRaw(c.ListTodos(api.Ctx(), params))
	if err != nil {
		return nil, err
	}
	list := cmdutil.NewListResult(data, "todos")
	list, err = applyTodoListOptions(list, opts)
	if err != nil {
		return nil, err
	}
	annotateTodoRows(list.Rows)
	return list.Rows, nil
}

// promptTodoPick lets the user pick a todo from a filtered list.
// Returns the selected row (which contains the "id" key), or nil if cancelled.
func promptTodoPick(cmd *cobra.Command, opts todoListOpts, prompt string) (map[string]any, error) {
	rows, err := fetchTodoPickList(cmd, opts)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		if opts.done {
			output.Dim("  No completed todos found.")
		} else if opts.pending {
			output.Dim("  No pending todos found.")
		} else {
			output.Dim("  No todos found.")
		}
		return nil, nil
	}
	return cmdutil.PromptPick(rows, todoPickColumns(), "id", prompt)
}

// todoTitleFromRow returns a short summary string for success messages.
func todoTitleFromRow(row map[string]any) string {
	title, _ := row["title"].(string)
	if title != "" {
		return title
	}
	id, _ := row["id"].(string)
	return id
}

func newCmdList() *cobra.Command {
	var opts todoListOpts
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List your todos",
		Example: `  life-ustc workspace todo list --pending --priority high
  life-ustc workspace todo list --done --sort due
  life-ustc workspace todo list --before 2025-06-01 --limit 10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTodoList(cmd, opts)
		},
	}
	addTodoListFlags(cmd, &opts)
	return cmd
}

func newCmdCreate() *cobra.Command {
	var (
		title, content, priority, due string
	)
	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Create a todo",
		Long:    "Create a todo. When run interactively without --title, prompts for input.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				if !cmdutil.IsInteractive() {
					return fmt.Errorf("--title is required in non-interactive mode")
				}
				output.Panel("New todo", "Add the title first; optional details can be left blank.")
				title = cmdutil.PromptText("Title")
				if title == "" {
					return fmt.Errorf("title is required")
				}
				if content == "" {
					content = cmdutil.PromptText("Content (optional)")
				}
				if priority == "" {
					priority = cmdutil.PromptSelect("Priority", []string{"low", "medium", "high"})
				}
				if due == "" {
					due = cmdutil.PromptText("Due date (optional, YYYY-MM-DD or RFC3339)")
				}
			}

			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			body := openapi.CreateTodoJSONRequestBody{Title: title}
			if content != "" {
				body.Content = &content
			}
			if priority != "" {
				if !validTodoPriority(priority) {
					return fmt.Errorf("invalid --priority %q (use low, medium, or high)", priority)
				}
				p := openapi.TodoCreateRequestSchemaPriority(priority)
				body.Priority = &p
			}
			if due != "" {
				dueAt := openapi.TodoCreateRequestSchema_DueAt{}
				_ = dueAt.FromTodoCreateRequestSchemaDueAt1(openapi.TodoCreateRequestSchemaDueAt1(due))
				body.DueAt = &dueAt
			}
			data, err := api.ParseResponseRaw(c.CreateTodo(api.Ctx(), body))
			if err != nil {
				return err
			}
			m := cmdutil.AsMap(data)
			id, _ := m["id"].(string)
			output.Success(fmt.Sprintf("Created todo %s: %s", id, title))
			return nil
		},
	}
	cmd.Flags().StringVarP(&title, "title", "t", "", "Todo title")
	cmd.Flags().StringVar(&content, "content", "", "Content")
	cmd.Flags().StringVar(&priority, "priority", "", "Priority (low, medium, high)")
	cmd.Flags().StringVar(&due, "due", "", "Due date (YYYY-MM-DD or RFC3339)")
	return cmd
}

func newCmdCompletion(completed bool) *cobra.Command {
	action := "complete"
	short := "Mark todo(s) as complete"
	if !completed {
		action = "reopen"
		short = "Mark todo(s) as incomplete"
	}
	cmd := &cobra.Command{
		Use:   action + " [todo-id]...",
		Short: short,
		Long:  short + ". When run interactively without IDs, shows matching todos and lets you pick one.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			var rows []map[string]any
			if len(args) > 0 {
				ids = make([]string, len(args))
				for i, arg := range args {
					ids[i] = strings.TrimSpace(arg)
				}
			} else {
				if !cmdutil.IsInteractive() {
					return fmt.Errorf("todo id is required in non-interactive mode")
				}
				opts := todoListOpts{sort: "due", limit: 20}
				if !completed {
					opts.done = true
				} else {
					opts.pending = true
				}
				picked, err := promptTodoPick(cmd, opts, "Pick a todo to "+action)
				if err != nil {
					return err
				}
				if picked == nil {
					return nil
				}
				id, _ := picked["id"].(string)
				ids = []string{id}
				rows = []map[string]any{picked}
			}
			return setTodosCompleted(cmd, ids, rows, completed)
		},
	}
	return cmd
}

func setTodosCompleted(cmd *cobra.Command, ids []string, rows []map[string]any, completed bool) error {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return err
	}
	items := make([]struct {
		Completed bool   `json:"completed"`
		TodoId    string `json:"todoId"`
	}, len(ids))
	for i, id := range ids {
		items[i].Completed = completed
		items[i].TodoId = id
	}
	body := openapi.PatchApiTodosBatchJSONRequestBody{Items: items}
	data, err := api.ParseResponseRaw(c.PatchApiTodosBatch(api.Ctx(), body))
	if err != nil {
		return err
	}
	return reportTodoBatchResults(data, rows, completed)
}

func reportTodoBatchResults(data any, rows []map[string]any, completed bool) error {
	labels := make(map[string]string, len(rows))
	for _, row := range rows {
		id, _ := row["id"].(string)
		if id != "" {
			labels[id] = todoTitleFromRow(row)
		}
	}

	m, ok := data.(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected batch response format")
	}
	results, _ := m["results"].([]any)

	var failures []string
	for _, r := range results {
		result, ok := r.(map[string]any)
		if !ok {
			continue
		}
		id, _ := result["todoId"].(string)
		success, _ := result["success"].(bool)
		if success {
			label := labels[id]
			if label == "" {
				label = id
			}
			if completed {
				output.Success(fmt.Sprintf("Completed: %s", label))
			} else {
				output.Success(fmt.Sprintf("Reopened: %s", label))
			}
			continue
		}
		if errMap, ok := result["error"].(map[string]any); ok {
			msg, _ := errMap["message"].(string)
			failures = append(failures, fmt.Sprintf("%s: %s", id, msg))
		} else {
			failures = append(failures, id)
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("failed to update %d todo(s):\n%s", len(failures), strings.Join(failures, "\n"))
	}
	return nil
}

func newCmdUpdate() *cobra.Command {
	var (
		title, content, priority, due string
		completed                     bool
		notCompleted                  bool
	)
	cmd := &cobra.Command{
		Use:   "update [todo-id]",
		Short: "Update a todo",
		Long:  "Update a todo. When run interactively without an ID, shows your todos and lets you pick one.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			if len(args) == 1 {
				id = strings.TrimSpace(args[0])
			}
			if id == "" {
				if !cmdutil.IsInteractive() {
					return fmt.Errorf("todo id is required in non-interactive mode")
				}
				picked, err := promptTodoPick(cmd, todoListOpts{sort: "due", limit: 20}, "Pick a todo to update")
				if err != nil {
					return err
				}
				if picked == nil {
					return nil
				}
				id, _ = picked["id"].(string)
			}
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			body := openapi.UpdateTodoJSONRequestBody{}
			hasUpdate := false
			if title != "" {
				body.Title = &title
				hasUpdate = true
			}
			if content != "" {
				body.Content = &content
				hasUpdate = true
			}
			if priority != "" {
				if !validTodoPriority(priority) {
					return fmt.Errorf("invalid --priority %q (use low, medium, or high)", priority)
				}
				p := openapi.TodoUpdateRequestSchemaPriority(priority)
				body.Priority = &p
				hasUpdate = true
			}
			if due != "" {
				dueAt := openapi.TodoUpdateRequestSchema_DueAt{}
				_ = dueAt.FromTodoUpdateRequestSchemaDueAt1(openapi.TodoUpdateRequestSchemaDueAt1(due))
				body.DueAt = &dueAt
				hasUpdate = true
			}
			if completed && notCompleted {
				return fmt.Errorf("--completed and --not-completed cannot be used together")
			}
			if completed {
				t := true
				body.Completed = &t
				hasUpdate = true
			}
			if notCompleted {
				f := false
				body.Completed = &f
				hasUpdate = true
			}
			if !hasUpdate {
				return fmt.Errorf("nothing to update — specify at least one flag (e.g. --title, --completed)")
			}
			_, err = api.ParseResponseRaw(c.UpdateTodo(api.Ctx(), id, body))
			if err != nil {
				return err
			}
			output.Success("Todo updated.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&title, "title", "t", "", "Title")
	cmd.Flags().StringVar(&content, "content", "", "Content")
	cmd.Flags().StringVar(&priority, "priority", "", "Priority")
	cmd.Flags().StringVar(&due, "due", "", "Due date")
	cmd.Flags().BoolVar(&completed, "completed", false, "Mark completed")
	cmd.Flags().BoolVar(&notCompleted, "not-completed", false, "Mark not completed")
	return cmd
}

func newCmdDelete() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "delete [todo-id]",
		Aliases: []string{"rm"},
		Short:   "Delete a todo",
		Long:    "Delete a todo. When run interactively without an ID, shows your todos and lets you pick one.",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			var row map[string]any
			if len(args) == 1 {
				id = strings.TrimSpace(args[0])
			}
			if id == "" {
				if !cmdutil.IsInteractive() {
					return fmt.Errorf("todo id is required in non-interactive mode")
				}
				picked, err := promptTodoPick(cmd, todoListOpts{sort: "due", limit: 20}, "Pick a todo to delete")
				if err != nil {
					return err
				}
				if picked == nil {
					return nil
				}
				id, _ = picked["id"].(string)
				row = picked
			} else {
				row = map[string]any{"id": id}
			}
			label := todoTitleFromRow(row)
			if !cmdutil.Confirm(fmt.Sprintf("Delete %s?", label), yes) {
				return nil
			}
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			_, err = api.ParseResponseRaw(c.DeleteTodo(api.Ctx(), id))
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
