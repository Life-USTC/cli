package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/authcmd"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/cmd/me"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
)

func NewCmdAccount() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account <command>",
		Short: "Manage identity, sessions, and preferences",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(
		me.NewCmdProfile(),
		authcmd.NewCmdLogin(),
		authcmd.NewCmdLogout(),
		authcmd.NewCmdSession(),
		authcmd.NewCmdToken(),
		newCmdLocale(),
	)
	return cmd
}

func newCmdLocale() *cobra.Command {
	return &cobra.Command{
		Use:   "locale <zh-cn|en-us>",
		Short: "Set your preferred locale",
		Args:  cobra.ExactArgs(1),
		ValidArgs: []string{
			"zh-cn",
			"en-us",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			locale := openapi.LocaleUpdateRequestSchemaLocale(args[0])
			if locale != "zh-cn" && locale != "en-us" {
				return fmt.Errorf("locale must be zh-cn or en-us")
			}
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), true)
			if err != nil {
				return err
			}
			if _, err := api.ParseResponseRaw(c.SetLocale(api.Ctx(), openapi.LocaleUpdateRequestSchema{Locale: locale})); err != nil {
				return err
			}
			output.Success("Locale updated.")
			return nil
		},
	}
}
