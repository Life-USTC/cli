package young_workspace

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/cmd/youngutil"
	"github.com/Life-USTC/CLI/internal/output"
)

// NewCmdYoungEventSubscription manages personal Young event subscriptions.
func NewCmdYoungEventSubscription() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "young-event-subscription <command>",
		Aliases: []string{"young-event-subscriptions"},
		Short:   "Manage Young event subscriptions",
		Args:    cobra.NoArgs,
	}
	cmd.AddCommand(newEventSubscriptionList(), newEventSubscriptionGet(), newEventSubscriptionSet())
	return cmd
}

func newEventSubscriptionList() *cobra.Command {
	var page, limit int
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List your Young event subscriptions",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			params, err := listParams(page, limit)
			if err != nil {
				return err
			}
			client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			data, err := client.DoJSON(cmd.Context(), http.MethodGet, youngutil.YoungEventSubscriptionsPath, params, nil)
			if err != nil {
				return err
			}
			list := cmdutil.NewListResult(data, "data")
			return output.OutputList(list.Raw, list.Rows, []output.Column{
				{Header: "Young ID", Key: "youngId"},
				{Header: "Name", Key: "event.name"},
				{Header: "Start", Key: "event.startAt"},
				{Header: "Signup", Key: "remindSignup"},
				{Header: "Deadline", Key: "remindDeadline"},
				{Header: "Start reminder", Key: "remindStart"},
			}, list.Total, list.Page)
		},
	}
	cmd.Flags().IntVarP(&page, "page", "p", 0, "Page number")
	cmd.Flags().IntVarP(&limit, "limit", "L", 0, "Number of subscriptions per page")
	return cmd
}

func newEventSubscriptionGet() *cobra.Command {
	return &cobra.Command{
		Use:     "get <young-id>",
		Aliases: []string{"show"},
		Short:   "Read one Young event subscription state",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			data, err := client.DoJSON(
				cmd.Context(),
				http.MethodGet,
				youngutil.PathID(youngutil.YoungEventSubscriptionsPath, args[0]),
				nil,
				nil,
			)
			if err != nil {
				return err
			}
			return output.OutputDetail(data, eventSubscriptionFields(), "Young event subscription")
		},
	}
}

func newEventSubscriptionSet() *cobra.Command {
	var subscribed, remindSignup, remindDeadline, remindStart string
	cmd := &cobra.Command{
		Use:   "set <young-id>",
		Short: "Set a Young event subscription and reminder flags",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("subscribed") {
				return fmt.Errorf("--subscribed is required (true or false)")
			}
			subscribedValue, err := parseBoolValue("--subscribed", subscribed)
			if err != nil {
				return err
			}
			body := map[string]any{"subscribed": subscribedValue}
			if cmd.Flags().Changed("remind-signup") {
				value, err := parseBoolValue("--remind-signup", remindSignup)
				if err != nil {
					return err
				}
				body["remindSignup"] = value
			}
			if cmd.Flags().Changed("remind-deadline") {
				value, err := parseBoolValue("--remind-deadline", remindDeadline)
				if err != nil {
					return err
				}
				body["remindDeadline"] = value
			}
			if cmd.Flags().Changed("remind-start") {
				value, err := parseBoolValue("--remind-start", remindStart)
				if err != nil {
					return err
				}
				body["remindStart"] = value
			}
			client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			data, err := client.DoJSON(
				cmd.Context(),
				http.MethodPut,
				youngutil.PathID(youngutil.YoungEventSubscriptionsPath, args[0]),
				nil,
				body,
			)
			if err != nil {
				return err
			}
			return output.OutputDetail(data, eventSubscriptionFields(), "Young event subscription")
		},
	}
	cmd.Flags().StringVar(&subscribed, "subscribed", "", "Subscribe (true) or unsubscribe (false)")
	cmd.Flags().StringVar(&remindSignup, "remind-signup", "", "Notify when signup opens (true or false)")
	cmd.Flags().StringVar(&remindDeadline, "remind-deadline", "", "Notify 24 hours before signup closes (true or false)")
	cmd.Flags().StringVar(&remindStart, "remind-start", "", "Notify one hour before the event starts (true or false)")
	return cmd
}

func eventSubscriptionFields() []output.FieldDef {
	return []output.FieldDef{
		{Key: "youngId", Label: "Young ID"},
		{Key: "subscribed", Label: "Subscribed"},
		{Key: "remindSignup", Label: "Signup reminder"},
		{Key: "remindDeadline", Label: "Deadline reminder"},
		{Key: "remindStart", Label: "Start reminder"},
	}
}

// NewCmdYoungOrganizerSubscription manages personal Young organizer follows.
func NewCmdYoungOrganizerSubscription() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "young-organizer-subscription <command>",
		Aliases: []string{"young-organizer-subscriptions"},
		Short:   "Manage Young organizer subscriptions",
		Args:    cobra.NoArgs,
	}
	cmd.AddCommand(newOrganizerSubscriptionList(), newOrganizerSubscriptionGet(), newOrganizerSubscriptionSet())
	return cmd
}

func newOrganizerSubscriptionList() *cobra.Command {
	var page, limit int
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List your Young organizer subscriptions",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			params, err := listParams(page, limit)
			if err != nil {
				return err
			}
			client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			data, err := client.DoJSON(cmd.Context(), http.MethodGet, youngutil.YoungOrganizerSubscriptionsPath, params, nil)
			if err != nil {
				return err
			}
			list := cmdutil.NewListResult(data, "data")
			return output.OutputList(list.Raw, list.Rows, []output.Column{
				{Header: "Organizer ID", Key: "organizerId"},
				{Header: "Name", Key: "organizer.name"},
				{Header: "Created", Key: "createdAt"},
			}, list.Total, list.Page)
		},
	}
	cmd.Flags().IntVarP(&page, "page", "p", 0, "Page number")
	cmd.Flags().IntVarP(&limit, "limit", "L", 0, "Number of subscriptions per page")
	return cmd
}

func newOrganizerSubscriptionGet() *cobra.Command {
	return &cobra.Command{
		Use:     "get <organizer-id>",
		Aliases: []string{"show"},
		Short:   "Read one Young organizer subscription state",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			data, err := client.DoJSON(
				cmd.Context(),
				http.MethodGet,
				youngutil.PathID(youngutil.YoungOrganizerSubscriptionsPath, args[0]),
				nil,
				nil,
			)
			if err != nil {
				return err
			}
			return output.OutputDetail(data, organizerSubscriptionFields(), "Young organizer subscription")
		},
	}
}

func newOrganizerSubscriptionSet() *cobra.Command {
	var subscribed string
	cmd := &cobra.Command{
		Use:   "set <organizer-id>",
		Short: "Set a Young organizer subscription",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("subscribed") {
				return fmt.Errorf("--subscribed is required (true or false)")
			}
			subscribedValue, err := parseBoolValue("--subscribed", subscribed)
			if err != nil {
				return err
			}
			client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			data, err := client.DoJSON(
				cmd.Context(),
				http.MethodPut,
				youngutil.PathID(youngutil.YoungOrganizerSubscriptionsPath, args[0]),
				nil,
				map[string]any{"subscribed": subscribedValue},
			)
			if err != nil {
				return err
			}
			return output.OutputDetail(data, organizerSubscriptionFields(), "Young organizer subscription")
		},
	}
	cmd.Flags().StringVar(&subscribed, "subscribed", "", "Follow (true) or unfollow (false)")
	return cmd
}

func organizerSubscriptionFields() []output.FieldDef {
	return []output.FieldDef{
		{Key: "organizerId", Label: "Organizer ID"},
		{Key: "subscribed", Label: "Subscribed"},
	}
}

// NewCmdYoungNotification manages personal Young activity notifications.
func NewCmdYoungNotification() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "young-notification <command>",
		Aliases: []string{"young-notifications"},
		Short:   "Read Young activity notifications",
		Args:    cobra.NoArgs,
	}
	cmd.AddCommand(newNotificationList(), newNotificationRead())
	return cmd
}

func newNotificationList() *cobra.Command {
	var page, limit int
	var unread bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List your Young notifications",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			params, err := listParams(page, limit)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("unread") {
				params.Set("unread", strconv.FormatBool(unread))
			}
			client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			data, err := client.DoJSON(cmd.Context(), http.MethodGet, youngutil.YoungNotificationsPath, params, nil)
			if err != nil {
				return err
			}
			list := cmdutil.NewListResult(data, "data")
			return output.OutputList(list.Raw, list.Rows, []output.Column{
				{Header: "Created", Key: "createdAt"},
				{Header: "Title", Key: "title"},
				{Header: "Kind", Key: "kind"},
				{Header: "Read", Key: "readAt"},
				{Header: "Young ID", Key: "youngId"},
				{Header: "Organizer ID", Key: "organizerId"},
			}, list.Total, list.Page)
		},
	}
	cmd.Flags().BoolVar(&unread, "unread", false, "Show only unread notifications (or false for read notifications)")
	cmd.Flags().IntVarP(&page, "page", "p", 0, "Page number")
	cmd.Flags().IntVarP(&limit, "limit", "L", 0, "Number of notifications per page")
	return cmd
}

func newNotificationRead() *cobra.Command {
	return &cobra.Command{
		Use:   "read <notification-id>",
		Short: "Mark a Young notification as read",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			data, err := client.DoJSON(
				cmd.Context(),
				http.MethodPost,
				youngutil.PathID(youngutil.YoungNotificationsPath, args[0])+"/read",
				nil,
				nil,
			)
			if err != nil {
				return err
			}
			return output.OutputDetail(data, []output.FieldDef{
				{Key: "id", Label: "Notification ID"},
				{Key: "success", Label: "Success"},
			}, "Young notification")
		},
	}
}

func listParams(page, limit int) (url.Values, error) {
	return youngutil.PageParams(page, limit)
}

func parseBoolValue(flag, value string) (bool, error) {
	if value != "true" && value != "false" {
		return false, fmt.Errorf("%s must be true or false", flag)
	}
	return value == "true", nil
}
