package description

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
)

func validDescriptionTargetType(targetType string) bool {
	switch targetType {
	case "section", "course", "teacher", "homework":
		return true
	default:
		return false
	}
}

func renderDescription(data any, includeHistoryID bool) error {
	if output.IsJSON() {
		return output.JSON(data)
	}
	m := cmdutil.AsMap(data)
	content := ""
	if c, ok := m["content"].(string); ok {
		content = c
	} else if c, ok := m["description"].(string); ok {
		content = c
	}
	if content != "" {
		fmt.Println()
		fmt.Println(content)
	} else {
		output.Dim("  No description.")
	}

	if history, ok := m["history"].([]any); ok && len(history) > 0 {
		fmt.Println()
		output.Bold("  History")
		rows := cmdutil.RowsFromAny(history)
		cols := []output.Column{
			{Header: "Updated", Key: "updatedAt"},
			{Header: "By", Key: "updatedBy.name"},
		}
		if includeHistoryID {
			cols = append([]output.Column{{Header: "ID", Key: "id"}}, cols...)
		}
		output.Table(rows, cols)
	}
	return nil
}

func runDescriptionGet(cmd *cobra.Command, targetType, targetID string, includeHistoryID bool) error {
	if !validDescriptionTargetType(targetType) {
		return fmt.Errorf("invalid --target-type %q", targetType)
	}
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	params := &openapi.GetDescriptionParams{
		TargetType: openapi.GetDescriptionParamsTargetType(targetType),
		TargetId:   targetID,
	}
	data, err := api.ParseResponseRaw(c.GetDescription(api.Ctx(), params))
	if err != nil {
		return err
	}
	return renderDescription(data, includeHistoryID)
}

func runDescriptionSet(cmd *cobra.Command, targetType, targetID, content string) error {
	if !validDescriptionTargetType(targetType) {
		return fmt.Errorf("invalid --target-type %q", targetType)
	}
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return err
	}
	params := &openapi.UpsertDescriptionParams{
		TargetType: openapi.UpsertDescriptionParamsTargetType(targetType),
		TargetId:   targetID,
	}
	reqBody := openapi.UpsertDescriptionJSONRequestBody{
		TargetType: openapi.DescriptionUpsertRequestSchemaTargetType(targetType),
		TargetId:   targetID,
		Content:    content,
	}
	data, err := api.ParseResponseRaw(c.UpsertDescription(api.Ctx(), params, reqBody))
	if err != nil {
		return err
	}
	m := cmdutil.AsMap(data)
	if updated, _ := m["updated"].(bool); updated {
		output.Success("Description updated.")
	} else {
		output.Success("Description created.")
	}
	return nil
}

func NewCmdDescription() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "description <command>",
		Short: "View and edit resource descriptions",
	}
	cmd.AddCommand(newCmdGet())
	cmd.AddCommand(newCmdSet())
	return cmd
}

// NewCmdDescriptionFor creates a "description" command tree scoped to a target type.
// get and set take the target ID as a positional argument.
func NewCmdDescriptionFor(targetType string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "description <command>",
		Short: fmt.Sprintf("View and edit %s descriptions", targetType),
	}
	cmd.AddCommand(newCmdGetFor(targetType))
	cmd.AddCommand(newCmdSetFor(targetType))
	return cmd
}

func newCmdGetFor(targetType string) *cobra.Command {
	return &cobra.Command{
		Use:     fmt.Sprintf("get <%s-id>", targetType),
		Aliases: []string{"show"},
		Short:   fmt.Sprintf("Get description for a %s", targetType),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDescriptionGet(cmd, targetType, args[0], false)
		},
	}
}

func newCmdSetFor(targetType string) *cobra.Command {
	var content string
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("set <%s-id>", targetType),
		Short: fmt.Sprintf("Create or update description for a %s", targetType),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDescriptionSet(cmd, targetType, args[0], content)
		},
	}
	cmd.Flags().StringVarP(&content, "content", "c", "", "Description content (Markdown)")
	_ = cmd.MarkFlagRequired("content")
	return cmd
}

func newCmdGet() *cobra.Command {
	var targetType, targetID string
	cmd := &cobra.Command{
		Use:     "get",
		Aliases: []string{"show"},
		Short:   "Get description for a resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDescriptionGet(cmd, targetType, targetID, true)
		},
	}
	cmd.Flags().StringVar(&targetType, "target-type", "", "Target type (section, course, teacher, homework)")
	cmd.Flags().StringVar(&targetID, "target-id", "", "Target ID")
	_ = cmd.MarkFlagRequired("target-type")
	_ = cmd.MarkFlagRequired("target-id")
	return cmd
}

func newCmdSet() *cobra.Command {
	var targetType, targetID, content string
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Create or update a description",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDescriptionSet(cmd, targetType, targetID, content)
		},
	}
	cmd.Flags().StringVar(&targetType, "target-type", "", "Target type")
	cmd.Flags().StringVar(&targetID, "target-id", "", "Target ID")
	cmd.Flags().StringVarP(&content, "content", "c", "", "Description content (Markdown)")
	_ = cmd.MarkFlagRequired("target-type")
	_ = cmd.MarkFlagRequired("target-id")
	_ = cmd.MarkFlagRequired("content")
	return cmd
}
