package root

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/config"
)

var (
	commentTargetTypeCompletions = []string{
		"section\tSection comments",
		"course\tCourse comments",
		"teacher\tTeacher comments",
		"section-teacher\tSection-teacher comments",
		"homework\tHomework comments",
	}
	descriptionTargetTypeCompletions = []string{
		"section\tSection descriptions",
		"course\tCourse descriptions",
		"teacher\tTeacher descriptions",
		"homework\tHomework descriptions",
	}
)

func registerCompletionMetadata(root *cobra.Command) {
	_ = root.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return filterCompletions([]string{
			"table\tHuman-readable terminal output",
			"json\tMachine-readable JSON output",
		}, toComplete, ""), cobra.ShellCompDirectiveNoFileComp
	})

	_ = root.RegisterFlagCompletionFunc("server", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		candidates := []string{
			config.DefaultServer + "\tBuilt-in default server",
		}
		if server := config.GetDefaultServer(); server != "" && server != config.DefaultServer {
			candidates = append([]string{server + "\tConfigured default server"}, candidates...)
		}
		return filterCompletions(candidates, toComplete, ""), cobra.ShellCompDirectiveNoFileComp
	})

	registerFlagCompletion(root, []string{"todo"}, "priority", []string{
		"low\tLow priority",
		"medium\tMedium priority",
		"high\tHigh priority",
	})

	registerFlagCompletion(root, []string{"comment", "list"}, "target-type", commentTargetTypeCompletions)
	registerFlagCompletion(root, []string{"comment", "create"}, "target-type", commentTargetTypeCompletions)
	registerFlagCompletion(root, []string{"description", "get"}, "target-type", descriptionTargetTypeCompletions)
	registerFlagCompletion(root, []string{"description", "set"}, "target-type", descriptionTargetTypeCompletions)
	registerFlagCompletion(root, []string{"api"}, "method", []string{
		"GET\tFetch a resource",
		"POST\tCreate or invoke an action",
		"PUT\tReplace a resource",
		"PATCH\tPartially update a resource",
		"DELETE\tDelete a resource",
	})
	_ = markFlagFilename(root, []string{"api"}, "input")
	_ = markFlagFilename(root, []string{"upload", "download"}, "output")
}

func registerFlagCompletion(root *cobra.Command, path []string, flag string, completions []string) {
	cmd := findCommand(root, path...)
	if cmd == nil {
		return
	}
	_ = cmd.RegisterFlagCompletionFunc(flag, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return filterCompletions(completions, toComplete, ""), cobra.ShellCompDirectiveNoFileComp
	})
}

func markFlagFilename(root *cobra.Command, path []string, flag string) error {
	cmd := findCommand(root, path...)
	if cmd == nil {
		return nil
	}
	return cmd.MarkFlagFilename(flag)
}

func findCommand(root *cobra.Command, path ...string) *cobra.Command {
	cmd := root
	for _, segment := range path {
		var next *cobra.Command
		for _, child := range cmd.Commands() {
			if child.Name() == segment || hasAlias(child, segment) {
				next = child
				break
			}
		}
		if next == nil {
			return nil
		}
		cmd = next
	}
	return cmd
}

func hasAlias(cmd *cobra.Command, value string) bool {
	for _, alias := range cmd.Aliases {
		if alias == value {
			return true
		}
	}
	return false
}

func filterCompletions(completions []string, toComplete, noMatchHelp string) []string {
	if toComplete == "" {
		return completions
	}

	var filtered []string
	for _, candidate := range completions {
		if strings.HasPrefix(candidate, toComplete) {
			filtered = append(filtered, candidate)
		}
	}
	if len(filtered) == 0 {
		if noMatchHelp == "" {
			noMatchHelp = "Press TAB again after entering a longer prefix."
		}
		filtered = cobra.AppendActiveHelp(filtered, noMatchHelp)
	}
	return filtered
}
