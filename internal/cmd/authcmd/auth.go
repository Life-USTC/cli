package authcmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/auth"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/config"
	"github.com/Life-USTC/CLI/internal/output"
)

func NewCmdAuth() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth [command]",
		Short: "Authenticate with a Life@USTC server",
		Long:  "Log in, log out, and inspect authentication status for a Life@USTC server.",
		Example: `  # Check auth status
  life-ustc auth

  # Log in via browser
  life-ustc auth login

  # Print the current access token
  life-ustc auth token`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthStatus(cmd)
		},
	}
	cmd.AddCommand(newCmdLogin())
	cmd.AddCommand(newCmdLogout())
	cmd.AddCommand(newCmdStatus())
	cmd.AddCommand(newCmdToken())
	return cmd
}

func runAuthStatus(cmd *cobra.Command) error {
	server := cmdutil.ServerFromCmd(cmd)
	cred, err := config.LoadCredentials(server)
	if err != nil {
		return err
	}
	if output.IsJSON() {
		data := map[string]any{"server": server, "authenticated": cred != nil}
		if cred != nil {
			data["expired"] = config.IsTokenExpired(cred)
			data["scope"] = cred.Scope
			data["hasRefreshToken"] = cred.RefreshToken != ""
		}
		return output.JSON(data)
	}
	if cred == nil {
		output.Warning(fmt.Sprintf("Not logged in to %s", server))
		return nil
	}
	status := "active"
	if config.IsTokenExpired(cred) {
		status = "expired"
	}
	output.KVWithTitle([]output.KVPair{
		{Key: "Server", Value: server},
		{Key: "Status", Value: status},
		{Key: "Scope", Value: cred.Scope},
		{Key: "Refresh token", Value: cred.RefreshToken != ""},
	}, "Auth status")
	return nil
}

func newCmdLogin() *cobra.Command {
	var useDeviceCode bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in via browser (OAuth2 + PKCE) or device code flow",
		RunE: func(cmd *cobra.Command, args []string) error {
			server := cmdutil.ServerFromCmd(cmd)
			var cred *config.Credential
			var err error
			if useDeviceCode {
				cred, err = auth.LoginDeviceCode(server)
			} else {
				cred, err = auth.Login(server)
			}
			if err != nil {
				return err
			}
			if err := config.SaveCredentials(server, cred); err != nil {
				return err
			}
			output.Success(fmt.Sprintf("Logged in to %s", server))
			return nil
		},
	}

	cmd.Flags().BoolVar(&useDeviceCode, "device", false, "Use device code flow (no browser redirect needed)")
	cmd.Flags().BoolVar(&useDeviceCode, "device-code", false, "Use device code flow (alias for --device)")

	return cmd
}

func newCmdLogout() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out and remove stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			server := cmdutil.ServerFromCmd(cmd)
			removed, err := config.RemoveCredentials(server)
			if err != nil {
				return err
			}
			if removed {
				output.Success(fmt.Sprintf("Logged out from %s", server))
			} else {
				output.Warning("No credentials found for this server.")
			}
			return nil
		},
	}
}

func newCmdStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthStatus(cmd)
		},
	}
}

func newCmdToken() *cobra.Command {
	return &cobra.Command{
		Use:   "token",
		Short: "Print the current access token",
		RunE: func(cmd *cobra.Command, args []string) error {
			server := cmdutil.ServerFromCmd(cmd)
			cred, err := config.LoadCredentials(server)
			if err != nil {
				return err
			}
			if cred == nil {
				return fmt.Errorf("not logged in. Run `life-ustc auth login` first")
			}
			if config.IsTokenExpired(cred) {
				newCred, err := auth.RefreshToken(server, cred)
				if err != nil {
					return fmt.Errorf("token expired and refresh failed: %w", err)
				}
				cred = newCred
				_ = config.SaveCredentials(server, cred)
			}
			fmt.Println(cred.AccessToken)
			return nil
		},
	}
}
