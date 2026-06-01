package configcmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/config"
	"github.com/Life-USTC/CLI/internal/output"
)

func NewCmdConfig() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config [command]",
		Short: "Manage CLI configuration",
		Long:  "View and update CLI configuration such as the default server URL.",
		Example: `  # Show the current server
  life-ustc config

  # Set a new default server
  life-ustc config set-server https://example.com`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(config.GetDefaultServer())
			return nil
		},
	}
	cmd.AddCommand(newCmdSetServer())
	cmd.AddCommand(newCmdGetServer())
	cmd.AddCommand(newCmdSetSchoolPrograms())
	cmd.AddCommand(newCmdGetSchoolPrograms())
	return cmd
}

func newCmdSetServer() *cobra.Command {
	return &cobra.Command{
		Use:   "set-server <url>",
		Short: "Set the default server URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.SetDefaultServer(args[0]); err != nil {
				return err
			}
			output.Success(fmt.Sprintf("Default server set to %s", args[0]))
			return nil
		},
	}
}

func newCmdGetServer() *cobra.Command {
	return &cobra.Command{
		Use:   "get-server",
		Short: "Show the default server URL",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(config.GetDefaultServer())
		},
	}
}

func newCmdSetSchoolPrograms() *cobra.Command {
	return &cobra.Command{
		Use:   "set-school-programs <undergraduate|graduate|undergraduate,graduate>",
		Short: "Set default school program selection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			programs, err := parseSchoolPrograms(args[0])
			if err != nil {
				return err
			}
			if err := config.SetSchoolPrograms(programs); err != nil {
				return err
			}
			output.Success(fmt.Sprintf("Default school programs set to %s", strings.Join(programs, ",")))
			return nil
		},
	}
}

func newCmdGetSchoolPrograms() *cobra.Command {
	return &cobra.Command{
		Use:   "get-school-programs",
		Short: "Show default school program selection",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(strings.Join(config.GetSchoolPrograms(), ","))
		},
	}
}

func parseSchoolPrograms(value string) ([]string, error) {
	seen := map[string]struct{}{}
	var programs []string
	for _, part := range strings.Split(value, ",") {
		program := strings.ToLower(strings.TrimSpace(part))
		switch program {
		case "undergrad", "undergraduate":
			program = "undergraduate"
		case "grad", "graduate":
			program = "graduate"
		default:
			return nil, fmt.Errorf("invalid school program %q; use undergraduate, graduate, or undergraduate,graduate", strings.TrimSpace(part))
		}
		if _, ok := seen[program]; ok {
			continue
		}
		seen[program] = struct{}{}
		programs = append(programs, program)
	}
	if len(programs) == 0 {
		return nil, fmt.Errorf("at least one school program is required")
	}
	return programs, nil
}
