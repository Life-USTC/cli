package root

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/Life-USTC/CLI/internal/output"
)

func configureHelp(root *cobra.Command) {
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		renderHelp(cmd.OutOrStdout(), cmd)
	})
}

func renderHelp(w io.Writer, cmd *cobra.Command) {
	if long := strings.TrimSpace(cmd.Long); long != "" {
		_, _ = fmt.Fprintln(w, long)
		_, _ = fmt.Fprintln(w)
	} else if short := strings.TrimSpace(cmd.Short); short != "" {
		_, _ = fmt.Fprintln(w, short)
		_, _ = fmt.Fprintln(w)
	}

	printSectionTitle(w, "USAGE")
	_, _ = fmt.Fprintf(w, "  %s\n", cmd.UseLine())

	if groups := groupedCommands(cmd); len(groups) > 0 {
		for _, group := range groups {
			_, _ = fmt.Fprintln(w)
			printSectionTitle(w, strings.ToUpper(strings.TrimSuffix(group.title, ":")))
			printCommandList(w, group.commands)
		}
	} else if commands := visibleChildCommands(cmd); len(commands) > 0 {
		_, _ = fmt.Fprintln(w)
		printSectionTitle(w, "COMMANDS")
		printCommandList(w, commands)
	}

	if flags := visibleFlags(cmd); len(flags) > 0 {
		_, _ = fmt.Fprintln(w)
		printSectionTitle(w, "FLAGS")
		printFlagList(w, flags)
	}

	if persistent := inheritedFlags(cmd); len(persistent) > 0 {
		_, _ = fmt.Fprintln(w)
		printSectionTitle(w, "GLOBAL FLAGS")
		printFlagList(w, persistent)
	}

	if examples := strings.TrimSpace(cmd.Example); examples != "" {
		_, _ = fmt.Fprintln(w)
		printSectionTitle(w, "EXAMPLES")
		for _, line := range strings.Split(examples, "\n") {
			line = strings.TrimRight(line, " ")
			if line == "" {
				_, _ = fmt.Fprintln(w)
				continue
			}
			if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t") {
				_, _ = fmt.Fprintf(w, "%s\n", line)
				continue
			}
			_, _ = fmt.Fprintf(w, "  %s\n", line)
		}
	}

	_, _ = fmt.Fprintln(w)
	printSectionTitle(w, "LEARN MORE")
	learnPath := cmd.CommandPath()
	if cmd.HasSubCommands() {
		learnPath += " <subcommand>"
	}
	_, _ = fmt.Fprintf(w, "  Use `%s --help` for more information about a command.\n", learnPath)
}

type commandGroup struct {
	title    string
	commands []*cobra.Command
}

func groupedCommands(cmd *cobra.Command) []commandGroup {
	if len(cmd.Groups()) == 0 {
		return nil
	}

	byID := make(map[string][]*cobra.Command)
	for _, child := range visibleChildCommands(cmd) {
		byID[child.GroupID] = append(byID[child.GroupID], child)
	}

	var groups []commandGroup
	for _, group := range cmd.Groups() {
		commands := byID[group.ID]
		if len(commands) == 0 {
			continue
		}
		groups = append(groups, commandGroup{title: group.Title, commands: commands})
		delete(byID, group.ID)
	}

	if leftovers := byID[""]; len(leftovers) > 0 {
		groups = append(groups, commandGroup{title: "Additional Commands:", commands: leftovers})
		delete(byID, "")
	}

	if len(byID) > 0 {
		keys := make([]string, 0, len(byID))
		for key := range byID {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			groups = append(groups, commandGroup{title: strings.ToUpper(key), commands: byID[key]})
		}
	}

	return groups
}

func visibleChildCommands(cmd *cobra.Command) []*cobra.Command {
	commands := cmd.Commands()
	out := make([]*cobra.Command, 0, len(commands))
	for _, child := range commands {
		if child.Hidden {
			continue
		}
		out = append(out, child)
	}
	return out
}

func printSectionTitle(w io.Writer, title string) {
	if output.IsTTY() {
		_, _ = fmt.Fprintln(w, color.New(color.Bold).Sprint(title))
		return
	}
	_, _ = fmt.Fprintln(w, title)
}

func printCommandList(w io.Writer, commands []*cobra.Command) {
	maxName := 0
	for _, cmd := range commands {
		if n := len(cmd.Name()); n > maxName {
			maxName = n
		}
	}

	for _, cmd := range commands {
		plainName := cmd.Name()
		name := plainName
		desc := parenthesize(cmd.Short)
		if output.IsTTY() {
			name = color.New(color.Bold).Sprint(name)
			desc = color.New(color.Faint).Sprint(desc)
		}
		_, _ = fmt.Fprintf(w, "  %s%s  %s\n", name, strings.Repeat(" ", maxName-len(plainName)), desc)
	}
}

func parenthesize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		return s
	}
	return "(" + s + ")"
}

func visibleFlags(cmd *cobra.Command) []*pflag.Flag {
	var flags []*pflag.Flag
	cmd.NonInheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		flags = append(flags, flag)
	})
	return flags
}

func inheritedFlags(cmd *cobra.Command) []*pflag.Flag {
	var flags []*pflag.Flag
	cmd.InheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		flags = append(flags, flag)
	})
	return flags
}

func printFlagList(w io.Writer, flags []*pflag.Flag) {
	maxName := 0
	names := make([]string, len(flags))
	for i, flag := range flags {
		name := formatFlagName(flag)
		names[i] = name
		if len(name) > maxName {
			maxName = len(name)
		}
	}

	for i, flag := range flags {
		plainName := names[i]
		name := plainName
		desc := parenthesize(flag.Usage)
		if value := flag.DefValue; value != "" && value != "false" && value != "[]" {
			desc = strings.TrimSuffix(desc, ")") + fmt.Sprintf(" Default: %q)", value)
		}
		if output.IsTTY() {
			name = color.New(color.Bold).Sprint(name)
			desc = color.New(color.Faint).Sprint(desc)
		}
		_, _ = fmt.Fprintf(w, "  %s%s  %s\n", name, strings.Repeat(" ", maxName-len(plainName)), desc)
	}
}

func formatFlagName(flag *pflag.Flag) string {
	var parts []string
	if flag.Shorthand != "" && flag.ShorthandDeprecated == "" {
		parts = append(parts, "-"+flag.Shorthand)
	}
	long := "--" + flag.Name
	if flag.Value.Type() != "bool" {
		long += " <" + flag.Value.Type() + ">"
	}
	parts = append(parts, long)
	return strings.Join(parts, ", ")
}
