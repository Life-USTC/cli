package admin

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
)

func NewCmdAdmin() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin <command>",
		Short: "Admin operations (requires admin privileges)",
	}
	cmd.AddCommand(newCmdUser())
	cmd.AddCommand(newCmdSuspension())
	cmd.AddCommand(newCmdComment())
	cmd.AddCommand(newCmdDescription())
	cmd.AddCommand(newCmdHomework())
	return cmd
}

// defaultListRunE returns a RunE that delegates to the "list" subcommand.
func defaultListRunE(parent *cobra.Command) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		listCmd, _, err := parent.Find([]string{"list"})
		if err != nil {
			return err
		}
		return listCmd.RunE(listCmd, args)
	}
}

// --- User ---

func newCmdUser() *cobra.Command {
	cmd := &cobra.Command{Use: "user <command>", Short: "Manage users"}
	cmd.AddCommand(newCmdUserList())
	cmd.AddCommand(newCmdUserUpdate())
	cmd.RunE = defaultListRunE(cmd)
	return cmd
}

func newCmdUserList() *cobra.Command {
	var (
		search      string
		page, limit int
	)
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			params := &openapi.ListAdminUsersParams{}
			params.Search = cmdutil.StringPtrIfSet(search)
			params.Page = cmdutil.IntStringPtrIfPositive(page)
			params.Limit = cmdutil.IntStringPtrIfPositive(limit)
			data, err := api.ParseResponseRaw(c.ListAdminUsers(api.Ctx(), params))
			if err != nil {
				return err
			}
			list := cmdutil.NewListResult(data, "data").FinalizeServerSide(limit)
			return output.OutputList(list.Raw, list.Rows, []output.Column{
				{Header: "ID", Key: "id"},
				{Header: "Name", Key: "name"},
				{Header: "Email", Key: "email"},
				{Header: "Username", Key: "username"},
				{Header: "Admin", Key: "isAdmin"},
			}, list.Total, list.Page)
		},
	}
	cmd.Flags().StringVar(&search, "search", "", "Search")
	cmdutil.AddListFlags(cmd, &page, &limit)
	return cmd
}

func newCmdUserUpdate() *cobra.Command {
	var (
		name, username string
		admin, noAdmin bool
	)
	cmd := &cobra.Command{
		Use:   "update <user-id>",
		Short: "Update a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			reqBody := openapi.UpdateAdminUserJSONRequestBody{}
			hasUpdate := false
			if name != "" {
				reqBody.Name = &name
				hasUpdate = true
			}
			if username != "" {
				reqBody.Username = &username
				hasUpdate = true
			}
			if admin {
				t := true
				reqBody.IsAdmin = &t
				hasUpdate = true
			}
			if noAdmin {
				f := false
				reqBody.IsAdmin = &f
				hasUpdate = true
			}
			if !hasUpdate {
				return fmt.Errorf("nothing to update — specify at least one flag")
			}
			_, err = api.ParseResponseRaw(c.UpdateAdminUser(api.Ctx(), args[0], reqBody))
			if err != nil {
				return err
			}
			output.Success("User updated.")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Name")
	cmd.Flags().StringVar(&username, "username", "", "Username")
	cmd.Flags().BoolVar(&admin, "admin", false, "Set admin")
	cmd.Flags().BoolVar(&noAdmin, "no-admin", false, "Remove admin")
	return cmd
}

// --- Suspension ---

func newCmdSuspension() *cobra.Command {
	cmd := &cobra.Command{Use: "suspension <command>", Short: "Manage suspensions"}
	cmd.AddCommand(newCmdSuspensionList())
	cmd.AddCommand(newCmdSuspensionCreate())
	cmd.AddCommand(newCmdSuspensionLift())
	cmd.RunE = defaultListRunE(cmd)
	return cmd
}

func newCmdSuspensionList() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List suspensions",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			data, err := api.ParseResponseRaw(c.ListAdminSuspensions(api.Ctx()))
			if err != nil {
				return err
			}
			list := cmdutil.NewListResult(data, "data").FinalizeServerSide(0)
			return output.OutputList(list.Raw, list.Rows, []output.Column{
				{Header: "ID", Key: "id"},
				{Header: "User", Key: "user.name"},
				{Header: "Reason", Key: "reason"},
				{Header: "Expires", Key: "expiresAt"},
				{Header: "Created", Key: "createdAt"},
			}, list.Total, list.Page)
		},
	}
	return cmd
}

func newCmdSuspensionCreate() *cobra.Command {
	var userID, reason, note, expiresAt string
	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Suspend a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			reqBody := openapi.CreateAdminSuspensionJSONRequestBody{
				UserId: userID,
			}
			if reason != "" {
				reqBody.Reason = &reason
			}
			if note != "" {
				reqBody.Note = &note
			}
			if expiresAt != "" {
				reqBody.ExpiresAt = &expiresAt
			}
			_, err = api.ParseResponseRaw(c.CreateAdminSuspension(api.Ctx(), reqBody))
			if err != nil {
				return err
			}
			output.Success("User suspended.")
			return nil
		},
	}
	cmd.Flags().StringVar(&userID, "user-id", "", "User ID (required)")
	_ = cmd.MarkFlagRequired("user-id")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason")
	cmd.Flags().StringVar(&note, "note", "", "Note")
	cmd.Flags().StringVar(&expiresAt, "expires-at", "", "Expiry date (ISO 8601)")
	return cmd
}

func newCmdSuspensionLift() *cobra.Command {
	return &cobra.Command{
		Use:   "lift <suspension-id>",
		Short: "Lift a suspension",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			_, err = api.ParseResponseRaw(c.UpdateAdminSuspension(api.Ctx(), args[0]))
			if err != nil {
				return err
			}
			output.Success("Suspension lifted.")
			return nil
		},
	}
}

// --- Admin Comment ---

func newCmdComment() *cobra.Command {
	cmd := &cobra.Command{Use: "comment <command>", Short: "Moderate comments"}
	cmd.AddCommand(newCmdCommentList())
	cmd.AddCommand(newCmdCommentModerate())
	cmd.RunE = defaultListRunE(cmd)
	return cmd
}

func newCmdCommentList() *cobra.Command {
	var status string
	var limit int
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List comments (admin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			params := &openapi.ListAdminCommentsParams{}
			if status != "" {
				s := openapi.ListAdminCommentsParamsStatus(status)
				params.Status = &s
			}
			params.Limit = cmdutil.IntStringPtrIfPositive(limit)
			data, err := api.ParseResponseRaw(c.ListAdminComments(api.Ctx(), params))
			if err != nil {
				return err
			}
			list := cmdutil.NewListResult(data, "data").FinalizeServerSide(limit)
			return output.OutputList(list.Raw, list.Rows, []output.Column{
				{Header: "ID", Key: "id"},
				{Header: "Body", Key: "body"},
				{Header: "User", Key: "user.name"},
				{Header: "Status", Key: "status"},
				{Header: "Created", Key: "createdAt"},
			}, list.Total, list.Page)
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Status filter (active, softbanned, deleted, suspended)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Limit results")
	return cmd
}

func newCmdCommentModerate() *cobra.Command {
	var status, note string
	cmd := &cobra.Command{
		Use:   "moderate <comment-id>",
		Short: "Moderate a comment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			reqBody := openapi.ModerateAdminCommentJSONRequestBody{
				Status: openapi.AdminModerateCommentRequestSchemaStatus(status),
			}
			if note != "" {
				reqBody.ModerationNote = &note
			}
			_, err = api.ParseResponseRaw(c.ModerateAdminComment(api.Ctx(), args[0], reqBody))
			if err != nil {
				return err
			}
			output.Success("Comment moderated.")
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Status (active, softbanned, deleted)")
	_ = cmd.MarkFlagRequired("status")
	cmd.Flags().StringVar(&note, "note", "", "Note")
	return cmd
}

// --- Admin Description ---

func newCmdDescription() *cobra.Command {
	cmd := &cobra.Command{Use: "description <command>", Short: "Moderate descriptions"}
	cmd.AddCommand(newCmdDescriptionList())
	cmd.RunE = defaultListRunE(cmd)
	return cmd
}

func newCmdDescriptionList() *cobra.Command {
	var targetType, hasContent, search string
	var limit int
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List descriptions (admin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			params := &openapi.ListAdminDescriptionsParams{}
			if targetType != "" {
				t := openapi.ListAdminDescriptionsParamsTargetType(targetType)
				params.TargetType = &t
			}
			if hasContent != "" {
				h := openapi.ListAdminDescriptionsParamsHasContent(hasContent)
				params.HasContent = &h
			}
			params.Search = cmdutil.StringPtrIfSet(search)
			params.Limit = cmdutil.IntStringPtrIfPositive(limit)
			data, err := api.ParseResponseRaw(c.ListAdminDescriptions(api.Ctx(), params))
			if err != nil {
				return err
			}
			list := cmdutil.NewListResult(data, "data").FinalizeServerSide(limit)
			return output.OutputList(list.Raw, list.Rows, []output.Column{
				{Header: "ID", Key: "id"},
				{Header: "Type", Key: "targetType"},
				{Header: "Target", Key: "targetId"},
				{Header: "Content", Key: "content"},
				{Header: "Updated", Key: "updatedAt"},
			}, list.Total, list.Page)
		},
	}
	cmd.Flags().StringVar(&targetType, "target-type", "", "Filter by type")
	cmd.Flags().StringVar(&hasContent, "has-content", "", "Filter: withContent, empty, all")
	cmd.Flags().StringVar(&search, "search", "", "Search")
	cmd.Flags().IntVar(&limit, "limit", 0, "Limit results")
	return cmd
}

// --- Admin Homework ---

func newCmdHomework() *cobra.Command {
	cmd := &cobra.Command{Use: "homework <command>", Short: "Moderate homeworks"}
	cmd.AddCommand(newCmdHomeworkList())
	cmd.AddCommand(newCmdHomeworkDelete())
	cmd.RunE = defaultListRunE(cmd)
	return cmd
}

func newCmdHomeworkList() *cobra.Command {
	var status, search string
	var limit int
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List homeworks (admin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			params := &openapi.ListAdminHomeworksParams{}
			if status != "" {
				s := openapi.ListAdminHomeworksParamsStatus(status)
				params.Status = &s
			}
			params.Search = cmdutil.StringPtrIfSet(search)
			params.Limit = cmdutil.IntStringPtrIfPositive(limit)
			data, err := api.ParseResponseRaw(c.ListAdminHomeworks(api.Ctx(), params))
			if err != nil {
				return err
			}
			list := cmdutil.NewListResult(data, "data").FinalizeServerSide(limit)
			return output.OutputList(list.Raw, list.Rows, []output.Column{
				{Header: "ID", Key: "id"},
				{Header: "Title", Key: "title"},
				{Header: "Section", Key: "section.code"},
				{Header: "Due", Key: "submissionDueAt"},
				{Header: "Status", Key: "status"},
			}, list.Total, list.Page)
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Status (all, active, deleted)")
	cmd.Flags().StringVar(&search, "search", "", "Search")
	cmd.Flags().IntVar(&limit, "limit", 0, "Limit results")
	return cmd
}

func newCmdHomeworkDelete() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "delete <homework-id>",
		Aliases: []string{"rm"},
		Short:   "Delete a homework (admin)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.Confirm("Delete this homework?", yes) {
				return nil
			}
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			_, err = api.ParseResponseRaw(c.DeleteAdminHomework(api.Ctx(), args[0]))
			if err != nil {
				return err
			}
			output.Success("Homework deleted.")
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation")
	return cmd
}
