package comment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/cmd/youngutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
)

var targetTypes = []string{"section", "course", "teacher", "section-teacher", "homework", "young-event"}

const commentsPath = "/api/community/comments"

type commentTarget struct {
	targetType string
	targetID   string
	youngID    string
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
	target = normalizeTarget(target)
	if !validCommentTargetType(target.targetType) {
		return fmt.Errorf("invalid --target-type %q", target.targetType)
	}
	if target.targetType == "young-event" {
		target = normalizeTarget(target)
		if _, err := youngutil.RequireID(target.youngID, "--young-id"); err != nil {
			return err
		}
		return nil
	}
	if !requireID {
		return nil
	}
	if target.targetType == "section-teacher" {
		if _, err := youngutil.RequireID(target.sectionID, "--section-id"); err != nil {
			return err
		}
		if _, err := youngutil.RequireID(target.teacherID, "--teacher-id"); err != nil {
			return err
		}
		return nil
	}
	if _, err := youngutil.RequireID(target.targetID, "--target-id"); err != nil {
		return err
	}
	return nil
}

func normalizeTarget(target commentTarget) commentTarget {
	target.targetType = strings.TrimSpace(target.targetType)
	target.targetID = strings.TrimSpace(target.targetID)
	target.youngID = strings.TrimSpace(target.youngID)
	target.sectionID = strings.TrimSpace(target.sectionID)
	target.teacherID = strings.TrimSpace(target.teacherID)
	if target.targetType == "young-event" && target.youngID == "" {
		target.youngID = target.targetID
	}
	return target
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
	target = normalizeTarget(target)
	if err := validateTarget(target, false); err != nil {
		return err
	}
	params, err := commentListParams(cmd, target)
	if err != nil {
		return err
	}
	client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), false)
	if err != nil {
		return err
	}
	data, err := youngutil.FetchAllIfUnpaged(
		cmd.Context(),
		client,
		commentsPath,
		params,
		"data",
		100,
		"id",
	)
	if err != nil {
		return err
	}
	list := cmdutil.NewListResult(data, "data")
	return output.OutputList(list.Raw, list.Rows, listCommentColumns(), list.Total, list.Page)
}

func commentListParams(cmd *cobra.Command, target commentTarget) (url.Values, error) {
	params, err := youngutil.PageParams(commandIntFlag(cmd, "page"), commandIntFlag(cmd, "limit"))
	if err != nil {
		return nil, err
	}
	params.Set("targetType", target.targetType)
	if target.targetID != "" && target.targetType != "young-event" {
		params.Set("targetId", target.targetID)
	}
	if target.youngID != "" {
		params.Set("youngId", target.youngID)
	}
	if target.sectionID != "" {
		if _, err := cmdutil.Int64PtrIfSet(target.sectionID); err != nil {
			return nil, err
		}
		params.Set("sectionId", target.sectionID)
	}
	if target.teacherID != "" {
		if _, err := cmdutil.Int64PtrIfSet(target.teacherID); err != nil {
			return nil, err
		}
		params.Set("teacherId", target.teacherID)
	}
	return params, nil
}

func commandIntFlag(cmd *cobra.Command, name string) int {
	if cmd.Flags().Lookup(name) == nil {
		return 0
	}
	value, err := cmd.Flags().GetInt(name)
	if err != nil {
		return 0
	}
	return value
}

func runCommentCreate(cmd *cobra.Command, target commentTarget, body, visibility, parentID string, anonymous bool) error {
	target = normalizeTarget(target)
	if !validVisibility(visibility) {
		return fmt.Errorf("invalid --visibility %q (use public, logged_in_only, or anonymous)", visibility)
	}
	if err := validateTarget(target, true); err != nil {
		return err
	}
	if target.targetType == "young-event" {
		client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), true)
		if err != nil {
			return err
		}
		request := map[string]any{
			"targetType":  "young-event",
			"youngId":     target.youngID,
			"body":        body,
			"visibility":  visibility,
			"isAnonymous": anonymous,
		}
		if parentID != "" {
			request["parentId"] = parentID
		}
		data, err := client.DoJSON(cmd.Context(), http.MethodPost, commentsPath, nil, request)
		if err != nil {
			return err
		}
		return reportCommentCreated(data)
	}
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return err
	}
	vis := openapi.CommentCreateRequestSchemaVisibility(visibility)
	reqBody := openapi.CreateCommentJSONRequestBody{
		TargetType:  openapi.CommentCreateRequestSchemaTargetType(target.targetType),
		Body:        body,
		Visibility:  &vis,
		IsAnonymous: &anonymous,
	}
	if target.targetID != "" {
		targetId := openapi.CommentCreateRequestSchema_TargetId{}
		_ = targetId.FromCommentCreateRequestSchemaTargetId0(target.targetID)
		reqBody.TargetId = &targetId
	}
	if target.sectionID != "" {
		sectionId := openapi.CommentCreateRequestSchema_SectionId{}
		_ = sectionId.FromCommentCreateRequestSchemaSectionId0(target.sectionID)
		reqBody.SectionId = &sectionId
	}
	if target.teacherID != "" {
		teacherId := openapi.CommentCreateRequestSchema_TeacherId{}
		_ = teacherId.FromCommentCreateRequestSchemaTeacherId0(target.teacherID)
		reqBody.TeacherId = &teacherId
	}
	if parentID != "" {
		reqBody.ParentId = &parentID
	}

	data, err := api.ParseResponseRaw(c.CreateComment(api.Ctx(), reqBody))
	if err != nil {
		return err
	}
	return reportCommentCreated(data)
}

func reportCommentCreated(data any) error {
	if output.IsJSON() {
		return output.JSON(data)
	}
	m := cmdutil.AsMap(data)
	if m == nil {
		return fmt.Errorf("unexpected comment response format")
	}
	id, _ := m["id"].(string)
	if id == "" {
		return fmt.Errorf("comment response has no id")
	}
	output.Success(fmt.Sprintf("Comment created: %s", id))
	return nil
}

func NewCmdComment() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment <command>",
		Short: "Read and write comments",
		Args:  cobra.NoArgs,
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
		Args:  cobra.NoArgs,
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
			targetID, err := youngutil.RequireID(args[0], "<target-id>")
			if err != nil {
				return err
			}
			return runCommentList(cmd, commentTarget{targetType: targetType, targetID: targetID})
		},
	}
	var page, limit int
	cmd.Flags().IntVarP(&page, "page", "p", 0, "Page number")
	cmd.Flags().IntVarP(&limit, "limit", "L", 0, "Number of comments per page")
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
			targetID, err := youngutil.RequireID(args[0], "<target-id>")
			if err != nil {
				return err
			}
			if body == "" {
				if !cmdutil.IsInteractive() {
					return fmt.Errorf("--body is required in non-interactive mode")
				}
				body = cmdutil.PromptText("Comment body")
			}
			return runCommentCreate(cmd, commentTarget{targetType: targetType, targetID: targetID}, body, visibility, parentID, anonymous)
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
		targetType, targetID, youngID, sectionID, teacherID string
	)
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List comments for a target",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if targetType == "" {
				return fmt.Errorf("--target-type is required")
			}
			return runCommentList(cmd, commentTarget{
				targetType: targetType,
				targetID:   targetID,
				youngID:    youngID,
				sectionID:  sectionID,
				teacherID:  teacherID,
			})
		},
	}
	cmd.Flags().StringVar(&targetType, "target-type", "", "Target type (section, course, teacher, section-teacher, homework, young-event)")
	cmd.Flags().StringVar(&targetID, "target-id", "", "Target ID")
	cmd.Flags().StringVar(&youngID, "young-id", "", "Young event ID (for --target-type young-event)")
	cmd.Flags().StringVar(&sectionID, "section-id", "", "Section ID (for section-teacher)")
	cmd.Flags().StringVar(&teacherID, "teacher-id", "", "Teacher ID (for section-teacher)")
	var page, limit int
	cmd.Flags().IntVarP(&page, "page", "p", 0, "Page number")
	cmd.Flags().IntVarP(&limit, "limit", "L", 0, "Number of comments per page")
	return cmd
}

func newCmdView() *cobra.Command {
	return &cobra.Command{
		Use:     "get <comment-id>",
		Aliases: []string{"show"},
		Short:   "View a comment thread",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commentID, err := youngutil.RequireID(args[0], "<comment-id>")
			if err != nil {
				return err
			}
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}
			data, err := api.ParseResponseRaw(c.GetComment(api.Ctx(), commentID))
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
		targetType, targetID, youngID, sectionID, teacherID string
		body, visibility, parentID                          string
		anonymous                                           bool
	)
	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Post a comment",
		Long:    "Post a comment. Prompts interactively when --target-type/--body are omitted.",
		Args:    cobra.NoArgs,
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
				} else if targetType == "young-event" {
					if youngID == "" && targetID != "" {
						youngID = targetID
					}
					if youngID == "" {
						youngID = cmdutil.PromptText("Young event ID")
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
				youngID:    youngID,
				sectionID:  sectionID,
				teacherID:  teacherID,
			}, body, visibility, parentID, anonymous)
		},
	}
	cmd.Flags().StringVar(&targetType, "target-type", "", "Target type")
	cmd.Flags().StringVar(&targetID, "target-id", "", "Target ID")
	cmd.Flags().StringVar(&youngID, "young-id", "", "Young event ID (for young-event target)")
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
			data, err := api.ParseResponseRaw(c.UpdateCommentWithBody(api.Ctx(), id, "application/json", bytes.NewReader(jsonBytes)))
			if err != nil {
				return err
			}
			return reportCommentMutation(data, "Comment updated.")
		},
	}
	cmd.Flags().StringVarP(&body, "body", "b", "", "New body")
	cmd.Flags().StringVar(&visibility, "visibility", "", "Visibility")
	return cmd
}

func newCmdDelete() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "delete [comment-id]...",
		Aliases: []string{"rm"},
		Short:   "Delete comment(s)",
		Long:    "Delete one or more comments. When run interactively without IDs, shows your recent comments and lets you pick one.",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			var rows []map[string]any
			if len(args) > 0 {
				ids = make([]string, len(args))
				for i, arg := range args {
					id, err := youngutil.RequireID(arg, "<comment-id>")
					if err != nil {
						return err
					}
					ids[i] = id
				}
			} else {
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
				id, _ := picked["id"].(string)
				ids = []string{id}
				rows = []map[string]any{picked}
			}

			label := commentBatchLabel(rows, len(ids))
			if !cmdutil.Confirm(fmt.Sprintf("Delete %s?", label), yes) {
				return nil
			}
			return deleteComments(cmd, ids, rows)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation")
	return cmd
}

func commentBatchLabel(rows []map[string]any, idCount int) string {
	if len(rows) == 1 {
		return commentLabelFromRow(rows[0])
	}
	if len(rows) > 1 {
		return fmt.Sprintf("%d comments", len(rows))
	}
	if idCount == 1 {
		return "this comment"
	}
	return fmt.Sprintf("%d comments", idCount)
}

func deleteComments(cmd *cobra.Command, ids []string, rows []map[string]any) error {
	c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
	if err != nil {
		return err
	}
	body := openapi.DeleteApiCommentsBatchJSONRequestBody{Ids: ids}
	data, err := api.ParseResponseRaw(c.DeleteApiCommentsBatch(api.Ctx(), body))
	if err != nil {
		return err
	}
	return reportCommentBatchResults(data, rows)
}

func reportCommentBatchResults(data any, rows []map[string]any) error {
	if output.IsJSON() {
		return output.JSON(data)
	}
	labels := make(map[string]string, len(rows))
	for _, row := range rows {
		id, _ := row["id"].(string)
		if id != "" {
			labels[id] = commentLabelFromRow(row)
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
		id, _ := result["id"].(string)
		success, _ := result["success"].(bool)
		if success {
			label := labels[id]
			if label == "" {
				label = "comment " + id
			}
			output.Success(fmt.Sprintf("Deleted: %s", label))
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
		return fmt.Errorf("failed to delete %d comment(s):\n%s", len(failures), strings.Join(failures, "\n"))
	}
	return nil
}

func newCmdReact() *cobra.Command {
	var reactionType string
	var remove bool
	cmd := &cobra.Command{
		Use:   "react <comment-id>",
		Short: "Add or remove a reaction",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commentID, err := youngutil.RequireID(args[0], "<comment-id>")
			if err != nil {
				return err
			}
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			if remove {
				params := &openapi.RemoveCommentReactionParams{
					Type: openapi.RemoveCommentReactionParamsType(reactionType),
				}
				data, err := api.ParseResponseRaw(c.RemoveCommentReaction(api.Ctx(), commentID, params))
				if err != nil {
					return err
				}
				return reportCommentMutation(data, "Reaction removed.")
			} else {
				body := openapi.AddCommentReactionJSONRequestBody{
					Type: openapi.CommentReactionRequestSchemaType(reactionType),
				}
				data, err := api.ParseResponseRaw(c.AddCommentReaction(api.Ctx(), commentID, body))
				if err != nil {
					return err
				}
				return reportCommentMutation(data, "Reaction added.")
			}
		},
	}
	cmd.Flags().StringVar(&reactionType, "type", "", "Reaction type/emoji (required)")
	_ = cmd.MarkFlagRequired("type")
	cmd.Flags().BoolVar(&remove, "remove", false, "Remove reaction")
	return cmd
}

func reportCommentMutation(data any, message string) error {
	if output.IsJSON() {
		return output.JSON(data)
	}
	output.Success(message)
	return nil
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
	list := cmdutil.NewListResult(data, "data").FinalizeServerSide(20)
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
