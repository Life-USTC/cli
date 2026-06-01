package comment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
)

var targetTypes = []string{"section", "course", "teacher", "section-teacher", "homework"}

type commentTarget struct {
	targetType string
	targetID   string
	sectionID  string
	teacherID  string
}

func validVisibility(visibility string) bool {
	switch visibility {
	case "public", "logged_in_only", "anonymous":
		return true
	default:
		return false
	}
}

func validCommentTargetType(targetType string) bool {
	for _, candidate := range targetTypes {
		if targetType == candidate {
			return true
		}
	}
	return false
}

func validateTarget(target commentTarget, requireID bool) error {
	if !validCommentTargetType(target.targetType) {
		return fmt.Errorf("invalid --target-type %q", target.targetType)
	}
	if !requireID {
		return nil
	}
	if target.targetType == "section-teacher" {
		if target.sectionID == "" || target.teacherID == "" {
			return fmt.Errorf("--section-id and --teacher-id are required for section-teacher target")
		}
		return nil
	}
	if target.targetID == "" {
		return fmt.Errorf("--target-id is required for this target type")
	}
	return nil
}

func listCommentColumns() []output.Column {
	return []output.Column{
		{Header: "ID", Key: "id"},
		{Header: "Body", Key: "body"},
		{Header: "Visibility", Key: "visibility"},
		{Header: "Created", Key: "createdAt"},
	}
}

func runCommentList(cmd *cobra.Command, target commentTarget) error {
	if err := validateTarget(target, false); err != nil {
		return err
	}
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	params := &openapi.ListCommentsParams{
		TargetType: openapi.ListCommentsParamsTargetType(target.targetType),
	}
	if target.targetID != "" {
		params.TargetId = &target.targetID
	}
	if target.sectionID != "" {
		params.SectionId, err = cmdutil.Int64PtrIfSet(target.sectionID)
		if err != nil {
			return err
		}
	}
	if target.teacherID != "" {
		params.TeacherId, err = cmdutil.Int64PtrIfSet(target.teacherID)
		if err != nil {
			return err
		}
	}
	data, err := api.ParseResponseRaw(c.ListComments(api.Ctx(), params))
	if err != nil {
		return err
	}
	_, rows, total, pg := cmdutil.ExtractList(data, "comments")
	return output.OutputList(data, rows, listCommentColumns(), total, pg)
}

func runCommentCreate(cmd *cobra.Command, target commentTarget, body, visibility, parentID string, anonymous bool) error {
	if !validVisibility(visibility) {
		return fmt.Errorf("invalid --visibility %q (use public, logged_in_only, or anonymous)", visibility)
	}
	if err := validateTarget(target, true); err != nil {
		return err
	}
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return err
	}
	params := &openapi.CreateCommentParams{
		TargetType: openapi.CreateCommentParamsTargetType(target.targetType),
	}
	if target.targetID != "" {
		params.TargetId = &target.targetID
	}
	if target.sectionID != "" {
		params.SectionId, err = cmdutil.Int64PtrIfSet(target.sectionID)
		if err != nil {
			return err
		}
	}
	if target.teacherID != "" {
		params.TeacherId, err = cmdutil.Int64PtrIfSet(target.teacherID)
		if err != nil {
			return err
		}
	}

	vis := openapi.CommentCreateRequestSchemaVisibility(visibility)
	reqBody := openapi.CreateCommentJSONRequestBody{
		TargetType:  openapi.CommentCreateRequestSchemaTargetType(target.targetType),
		Body:        body,
		Visibility:  &vis,
		IsAnonymous: &anonymous,
	}
	if target.targetID != "" {
		reqBody.TargetId = &target.targetID
	}
	if target.sectionID != "" {
		reqBody.SectionId = &target.sectionID
	}
	if target.teacherID != "" {
		reqBody.TeacherId = &target.teacherID
	}
	if parentID != "" {
		reqBody.ParentId = &parentID
	}

	data, err := api.ParseResponseRaw(c.CreateComment(api.Ctx(), params, reqBody))
	if err != nil {
		return err
	}
	m := cmdutil.AsMap(data)
	id, _ := m["id"].(string)
	output.Success(fmt.Sprintf("Comment created: %s", id))
	return nil
}

func NewCmdComment() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment <command>",
		Short: "Read and write comments",
	}
	cmd.AddCommand(newCmdList())
	cmd.AddCommand(newCmdView())
	cmd.AddCommand(newCmdCreate())
	cmd.AddCommand(newCmdUpdate())
	cmd.AddCommand(newCmdDelete())
	cmd.AddCommand(newCmdReact())
	return cmd
}

// NewCmdCommentFor creates a "comment" command tree scoped to a target type.
// list and create take the target ID as a positional argument.
func NewCmdCommentFor(targetType string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment <command>",
		Short: fmt.Sprintf("Comments on this %s", targetType),
	}
	cmd.AddCommand(newCmdListFor(targetType))
	cmd.AddCommand(newCmdView())
	cmd.AddCommand(newCmdCreateFor(targetType))
	cmd.AddCommand(newCmdUpdate())
	cmd.AddCommand(newCmdDelete())
	cmd.AddCommand(newCmdReact())
	return cmd
}

func newCmdListFor(targetType string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("list <%s-id>", targetType),
		Aliases: []string{"ls"},
		Short:   fmt.Sprintf("List comments for a %s", targetType),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommentList(cmd, commentTarget{targetType: targetType, targetID: args[0]})
		},
	}
	return cmd
}

func newCmdCreateFor(targetType string) *cobra.Command {
	var (
		body, visibility, parentID string
		anonymous                  bool
	)
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("create <%s-id>", targetType),
		Aliases: []string{"new"},
		Short:   fmt.Sprintf("Post a comment on a %s", targetType),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if body == "" {
				if !cmdutil.IsInteractive() {
					return fmt.Errorf("--body is required in non-interactive mode")
				}
				body = cmdutil.PromptText("Comment body")
			}
			return runCommentCreate(cmd, commentTarget{targetType: targetType, targetID: args[0]}, body, visibility, parentID, anonymous)
		},
	}
	cmd.Flags().StringVarP(&body, "body", "b", "", "Comment body")
	cmd.Flags().StringVar(&visibility, "visibility", "public", "Visibility (public, logged_in_only, anonymous)")
	cmd.Flags().BoolVar(&anonymous, "anonymous", false, "Post anonymously")
	cmd.Flags().StringVar(&parentID, "parent-id", "", "Reply to comment ID")
	return cmd
}

func newCmdList() *cobra.Command {
	var (
		targetType, targetID, sectionID, teacherID string
	)
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List comments for a target",
		RunE: func(cmd *cobra.Command, args []string) error {
			if targetType == "" {
				return fmt.Errorf("--target-type is required")
			}
			return runCommentList(cmd, commentTarget{
				targetType: targetType,
				targetID:   targetID,
				sectionID:  sectionID,
				teacherID:  teacherID,
			})
		},
	}
	cmd.Flags().StringVar(&targetType, "target-type", "", "Target type (section, course, teacher, section-teacher, homework)")
	cmd.Flags().StringVar(&targetID, "target-id", "", "Target ID")
	cmd.Flags().StringVar(&sectionID, "section-id", "", "Section ID (for section-teacher)")
	cmd.Flags().StringVar(&teacherID, "teacher-id", "", "Teacher ID (for section-teacher)")
	return cmd
}

func newCmdView() *cobra.Command {
	return &cobra.Command{
		Use:     "view <comment-id>",
		Aliases: []string{"show"},
		Short:   "View a comment thread",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}
			data, err := api.ParseResponseRaw(c.GetComment(api.Ctx(), args[0]))
			if err != nil {
				return err
			}
			if output.IsJSON() {
				return output.JSON(data)
			}
			m := cmdutil.AsMap(data)
			output.KVWithTitle([]output.KVPair{
				{Key: "ID", Value: output.Resolve(m, "id")},
				{Key: "Body", Value: output.Resolve(m, "body")},
				{Key: "Visibility", Value: output.Resolve(m, "visibility")},
				{Key: "Anonymous", Value: output.Resolve(m, "isAnonymous")},
				{Key: "Created", Value: output.Resolve(m, "createdAt")},
				{Key: "Updated", Value: output.Resolve(m, "updatedAt")},
			}, "Comment")

			if replies, ok := m["replies"].([]any); ok && len(replies) > 0 {
				fmt.Println()
				output.Bold("  Replies")
				rows := cmdutil.RowsFromAny(replies)
				output.Table(rows, []output.Column{
					{Header: "ID", Key: "id"},
					{Header: "Body", Key: "body"},
					{Header: "Created", Key: "createdAt"},
				})
			}
			return nil
		},
	}
}

func newCmdCreate() *cobra.Command {
	var (
		targetType, targetID, sectionID, teacherID string
		body, visibility, parentID                 string
		anonymous                                  bool
	)
	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Post a comment",
		Long:    "Post a comment. Prompts interactively when --target-type/--body are omitted.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if targetType == "" || body == "" {
				if !cmdutil.IsInteractive() {
					return fmt.Errorf("--target-type and --body are required in non-interactive mode")
				}
				if targetType == "" {
					targetType = cmdutil.PromptSelect("Target type", targetTypes)
				}
				if targetType == "section-teacher" {
					if sectionID == "" {
						sectionID = cmdutil.PromptText("Section ID")
					}
					if teacherID == "" {
						teacherID = cmdutil.PromptText("Teacher ID")
					}
				} else if targetID == "" {
					targetID = cmdutil.PromptText("Target ID")
				}
				if body == "" {
					body = cmdutil.PromptText("Comment body")
				}
			}

			return runCommentCreate(cmd, commentTarget{
				targetType: targetType,
				targetID:   targetID,
				sectionID:  sectionID,
				teacherID:  teacherID,
			}, body, visibility, parentID, anonymous)
		},
	}
	cmd.Flags().StringVar(&targetType, "target-type", "", "Target type")
	cmd.Flags().StringVar(&targetID, "target-id", "", "Target ID")
	cmd.Flags().StringVar(&sectionID, "section-id", "", "Section ID")
	cmd.Flags().StringVar(&teacherID, "teacher-id", "", "Teacher ID")
	cmd.Flags().StringVarP(&body, "body", "b", "", "Comment body")
	cmd.Flags().StringVar(&visibility, "visibility", "public", "Visibility (public, logged_in_only, anonymous)")
	cmd.Flags().BoolVar(&anonymous, "anonymous", false, "Post anonymously")
	cmd.Flags().StringVar(&parentID, "parent-id", "", "Reply to comment ID")
	return cmd
}

func newCmdUpdate() *cobra.Command {
	var body, visibility string
	cmd := &cobra.Command{
		Use:   "update [comment-id]",
		Short: "Edit a comment",
		Long:  "Edit a comment. When run interactively without an ID, shows your recent comments and lets you pick one.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if visibility != "" && !validVisibility(visibility) {
				return fmt.Errorf("invalid --visibility %q (use public, logged_in_only, or anonymous)", visibility)
			}
			id := ""
			if len(args) == 1 {
				id = strings.TrimSpace(args[0])
			}
			if id == "" {
				if !cmdutil.IsInteractive() {
					return fmt.Errorf("comment id is required in non-interactive mode")
				}
				picked, err := promptCommentPick(cmd, "Pick a comment to edit")
				if err != nil {
					return err
				}
				if picked == nil {
					return nil
				}
				id, _ = picked["id"].(string)
				if body == "" {
					body = cmdutil.PromptText("New body")
				}
			}
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			payload := map[string]any{}
			if body != "" {
				payload["body"] = body
			}
			if visibility != "" {
				payload["visibility"] = visibility
			}
			if len(payload) == 0 {
				return fmt.Errorf("nothing to update — specify at least one flag")
			}
			jsonBytes, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			_, err = api.ParseResponseRaw(c.UpdateCommentWithBody(api.Ctx(), id, "application/json", bytes.NewReader(jsonBytes)))
			if err != nil {
				return err
			}
			output.Success("Comment updated.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&body, "body", "b", "", "New body")
	cmd.Flags().StringVar(&visibility, "visibility", "", "Visibility")
	return cmd
}

func newCmdDelete() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "delete [comment-id]",
		Aliases: []string{"rm"},
		Short:   "Delete a comment",
		Long:    "Delete a comment. When run interactively without an ID, shows your recent comments and lets you pick one.",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			var row map[string]any
			if len(args) == 1 {
				id = strings.TrimSpace(args[0])
			}
			if id == "" {
				if !cmdutil.IsInteractive() {
					return fmt.Errorf("comment id is required in non-interactive mode")
				}
				picked, err := promptCommentPick(cmd, "Pick a comment to delete")
				if err != nil {
					return err
				}
				if picked == nil {
					return nil
				}
				id, _ = picked["id"].(string)
				row = picked
			}
			label := commentLabelFromRow(row)
			if !cmdutil.Confirm(fmt.Sprintf("Delete %s?", label), yes) {
				return nil
			}
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			_, err = api.ParseResponseRaw(c.DeleteCommentWithBody(api.Ctx(), id, "application/json", strings.NewReader("{}")))
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

func newCmdReact() *cobra.Command {
	var reactionType string
	var remove bool
	cmd := &cobra.Command{
		Use:   "react <comment-id>",
		Short: "Add or remove a reaction",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			if remove {
				params := &openapi.RemoveCommentReactionParams{
					Type: openapi.RemoveCommentReactionParamsType(reactionType),
				}
				body := openapi.RemoveCommentReactionJSONRequestBody{
					Type: openapi.CommentReactionRequestSchemaType(reactionType),
				}
				_, err = api.ParseResponseRaw(c.RemoveCommentReaction(api.Ctx(), args[0], params, body))
				if err != nil {
					return err
				}
				output.Success("Reaction removed.")
			} else {
				body := openapi.AddCommentReactionJSONRequestBody{
					Type: openapi.CommentReactionRequestSchemaType(reactionType),
				}
				_, err = api.ParseResponseRaw(c.AddCommentReaction(api.Ctx(), args[0], body))
				if err != nil {
					return err
				}
				output.Success("Reaction added.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&reactionType, "type", "", "Reaction type/emoji (required)")
	_ = cmd.MarkFlagRequired("type")
	cmd.Flags().BoolVar(&remove, "remove", false, "Remove reaction")
	return cmd
}

// promptCommentPick loads the user's recent comments and lets them pick one.
func promptCommentPick(cmd *cobra.Command, prompt string) (map[string]any, error) {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return nil, err
	}
	// Fetch the user's own comments — use a general list with a limit
	params := &openapi.ListCommentsParams{}
	data, err := api.ParseResponseRaw(c.ListComments(api.Ctx(), params))
	if err != nil {
		return nil, err
	}
	list := cmdutil.NewListResult(data, "comments").FinalizeServerSide(20)
	if len(list.Rows) == 0 {
		output.Dim("  No comments found.")
		return nil, nil
	}
	return cmdutil.PromptPick(list.Rows, listCommentColumns(), "id", prompt)
}

// commentLabelFromRow returns a short summary for success messages.
func commentLabelFromRow(row map[string]any) string {
	if row == nil {
		return "this comment"
	}
	body, _ := row["body"].(string)
	if body != "" {
		// Truncate long bodies for readable confirm messages
		if len(body) > 40 {
			return body[:37] + "..."
		}
		return body
	}
	id, _ := row["id"].(string)
	if id != "" {
		return "comment " + id
	}
	return "this comment"
}
