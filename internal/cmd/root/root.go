package root

import (
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/cmd/admin"
	"github.com/Life-USTC/CLI/internal/cmd/apicmd"
	"github.com/Life-USTC/CLI/internal/cmd/authcmd"
	"github.com/Life-USTC/CLI/internal/cmd/bus"
	"github.com/Life-USTC/CLI/internal/cmd/calendar"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/cmd/comment"
	"github.com/Life-USTC/CLI/internal/cmd/configcmd"
	"github.com/Life-USTC/CLI/internal/cmd/course"
	"github.com/Life-USTC/CLI/internal/cmd/description"
	"github.com/Life-USTC/CLI/internal/cmd/homework"
	"github.com/Life-USTC/CLI/internal/cmd/me"
	"github.com/Life-USTC/CLI/internal/cmd/metadata"
	"github.com/Life-USTC/CLI/internal/cmd/schedule"
	schoolcmd "github.com/Life-USTC/CLI/internal/cmd/school"
	"github.com/Life-USTC/CLI/internal/cmd/section"
	"github.com/Life-USTC/CLI/internal/cmd/semester"
	"github.com/Life-USTC/CLI/internal/cmd/teacher"
	"github.com/Life-USTC/CLI/internal/cmd/todo"
	"github.com/Life-USTC/CLI/internal/cmd/upload"
	"github.com/Life-USTC/CLI/internal/output"
)

var version = "dev"

// Command group IDs
const (
	groupStart     = "start"
	groupPersonal  = "personal"
	groupBrowse    = "browse"
	groupCommunity = "community"
	groupRef       = "reference"
	groupAdmin     = "admin"
	groupPlumbing  = "plumbing"
)

func grouped(groupID string, cmd *cobra.Command) *cobra.Command {
	cmd.GroupID = groupID
	return cmd
}

func NewCmdRoot() *cobra.Command {
	var (
		server  string
		format  string
		noColor bool
		jq      string
		verbose bool
		jsonOut bool
	)

	cobra.EnableCommandSorting = false

	cmd := &cobra.Command{
		Use:   "life-ustc <command> [flags]",
		Short: "Life@USTC in your terminal",
		Long: `Work seamlessly with the USTC campus platform from the command line.

Browse courses, sections, and teachers. Manage your todos, homework,
calendar, uploads, comments, and descriptions. Use --json or --jq for
scripting, or drop down to 'life-ustc api' for raw endpoint access.`,
		Example: `  # Show your profile
  life-ustc me

  # Check your pending todos
  life-ustc todo --pending

  # Browse sections and filter with jq
  life-ustc section list --limit 5 --jq '.data[].code'

  # View a course and its sections
  life-ustc course view <course-id>

  # Call a raw API endpoint
  life-ustc api semesters/current --jq '.currentSemester.id'

  # Install shell completion into your current shell
  life-ustc completion install`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if jsonOut {
				format = "json"
			}
			output.Current.Format = format
			output.Current.NoColor = noColor
			output.Current.JQ = jq
			output.Current.Verbose = verbose
			if noColor {
				color.NoColor = true
			}
			if cmd.Parent() == nil && len(args) == 0 && !isCompletionCommand(cmd) {
				maybeHintCompletion(cmd)
			}
		},
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}

	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.SetVersionTemplate("life-ustc version {{.Version}}\n")

	// Global flags
	cmd.PersistentFlags().StringVar(&server, "server", "", "Server URL (default: life-ustc.tiankaima.dev, env: LIFE_USTC_SERVER)")
	cmd.PersistentFlags().StringVar(&format, "format", "table", "Output format: table, json")
	cmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	cmd.PersistentFlags().StringVar(&jq, "jq", "", "Filter JSON output with a jq expression")
	cmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Show verbose output (API requests, timing)")
	cmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output as JSON (shorthand for --format json)")

	// Command groups
	cmd.AddGroup(
		&cobra.Group{ID: groupStart, Title: "Start here:"},
		&cobra.Group{ID: groupPersonal, Title: "Personal:"},
		&cobra.Group{ID: groupBrowse, Title: "Browse:"},
		&cobra.Group{ID: groupCommunity, Title: "Community:"},
		&cobra.Group{ID: groupRef, Title: "Reference:"},
		&cobra.Group{ID: groupAdmin, Title: "Administration:"},
		&cobra.Group{ID: groupPlumbing, Title: "Setup and automation:"},
	)

	cmd.AddCommand(
		grouped(groupStart, authcmd.NewCmdAuth()),
		grouped(groupStart, me.NewCmdMe()),

		grouped(groupPersonal, todo.NewCmdTodo()),
		grouped(groupPersonal, homework.NewCmdMyHomework()),
		grouped(groupPersonal, calendar.NewCmdCalendar()),
		grouped(groupPersonal, upload.NewCmdUpload()),

		grouped(groupBrowse, course.NewCmdCourse()),
		grouped(groupBrowse, section.NewCmdSection()),
		grouped(groupBrowse, teacher.NewCmdTeacher()),
		grouped(groupBrowse, semester.NewCmdSemester()),
		grouped(groupBrowse, schedule.NewCmdSchedule()),
		grouped(groupBrowse, bus.NewCmdBus()),
		grouped(groupBrowse, schoolcmd.NewCmdSchool()),

		grouped(groupCommunity, comment.NewCmdComment()),
		grouped(groupCommunity, description.NewCmdDescription()),

		grouped(groupRef, metadata.NewCmdMetadata()),
		grouped(groupAdmin, admin.NewCmdAdmin()),

		grouped(groupPlumbing, configcmd.NewCmdConfig()),
		grouped(groupPlumbing, newCmdCompletion()),
		grouped(groupPlumbing, apicmd.NewCmdAPI()),
	)

	cmd.InitDefaultHelpCmd()
	registerCompletionMetadata(cmd)
	configureHelp(cmd)

	return cmd
}

func isCompletionCommand(cmd *cobra.Command) bool {
	if cmd.Name() == "__complete" {
		return true
	}
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "completion" {
			return true
		}
	}
	return false
}

func maybeHintCompletion(cmd *cobra.Command) {
	if !cmdutil.IsInteractive() || output.IsJSON() {
		return
	}
	shell, ok := detectShell()
	if !ok {
		return
	}
	if completionIsInstalled(shell) {
		return
	}
	output.Hint("tab completions not set up — run: life-ustc completion install")
}

func completionIsInstalled(shell string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	switch shell {
	case "bash":
		for _, p := range []string{
			"/usr/share/bash-completion/completions/life-ustc",
			filepath.Join(home, ".local", "share", "bash-completion", "completions", "life-ustc"),
		} {
			if _, err := os.Stat(p); err == nil {
				return true
			}
		}
	case "zsh":
		for _, p := range []string{
			"/usr/share/zsh/site-functions/_life-ustc",
			"/usr/local/share/zsh/site-functions/_life-ustc",
			filepath.Join(home, ".zsh", "completions", "_life-ustc"),
		} {
			if _, err := os.Stat(p); err == nil {
				return true
			}
		}
	case "fish":
		for _, p := range []string{
			filepath.Join(home, ".config", "fish", "completions", "life-ustc.fish"),
		} {
			if _, err := os.Stat(p); err == nil {
				return true
			}
		}
	}
	return false
}
