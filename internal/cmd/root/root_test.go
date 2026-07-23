package root

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandsUseUnifiedDomainHierarchy(t *testing.T) {
	cmd := NewCmdRoot()

	todo := findCommand(cmd, "workspace", "todo")
	if todo == nil {
		t.Fatal("workspace todo command missing")
	}
	if findCommand(todo, "complete") == nil {
		t.Fatal("todo complete command missing")
	}
	if findCommand(todo, "reopen") == nil {
		t.Fatal("todo reopen command missing")
	}
	homework := findCommand(cmd, "workspace", "homework")
	if homework == nil {
		t.Fatal("workspace homework command missing")
	}
	if findCommand(homework, "complete") == nil {
		t.Fatal("homework complete command missing")
	}
	if findCommand(homework, "reopen") == nil {
		t.Fatal("homework reopen command missing")
	}
	for _, stale := range []string{"me", "auth", "todo", "homework", "calendar", "upload", "course", "section", "bus", "comment"} {
		if findCommand(cmd, stale) != nil {
			t.Fatalf("stale top-level %q command still registered", stale)
		}
	}
}

func TestUnifiedDomainContents(t *testing.T) {
	cmd := NewCmdRoot()
	expected := map[string][]string{
		"catalog": {
			"metadata", "semester", "course", "section", "teacher", "schedule", "bus",
		},
		"workspace": {
			"overview", "calendar", "schedule", "exam", "todo", "homework",
			"subscription", "bus-preferences", "upload", "school",
		},
		"community": {
			"comment", "description", "section-homework",
		},
		"account": {
			"profile", "login", "logout", "session", "token", "locale",
		},
	}
	for scope, children := range expected {
		for _, child := range children {
			if findCommand(cmd, scope, child) == nil {
				t.Errorf("expected %s %s command is missing", scope, child)
			}
		}
	}

	if findCommand(cmd, "catalog", "bus", "preferences") != nil {
		t.Error("personal bus preferences leaked into catalog")
	}
	if findCommand(cmd, "workspace", "homework", "create") != nil {
		t.Error("shared homework creation leaked into workspace")
	}
	if findCommand(cmd, "workspace", "calendar", "set") != nil {
		t.Error("section subscriptions leaked into calendar")
	}
}

func TestCompletionInstallIsExplicitSubcommand(t *testing.T) {
	cmd := NewCmdRoot()
	completion := findCommand(cmd, "completion")
	if completion == nil {
		t.Fatal("completion command missing")
	}
	if findCommand(completion, "install") == nil {
		t.Fatal("completion install command missing")
	}
}

func TestCompleteShellsFiltersByPrefix(t *testing.T) {
	got, directive := completeShells(nil, nil, "ba")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v", directive)
	}
	if len(got) != 1 || got[0] != "bash\tGNU Bash" {
		t.Fatalf("shell completions = %v", got)
	}
}

func TestCompleteShellsNoMatchActiveHelp(t *testing.T) {
	got, _ := completeShells(nil, nil, "nope")
	if len(got) != 1 || !strings.Contains(got[0], "Use --shell with bash, zsh, fish, or powershell.") {
		t.Fatalf("shell active help = %v", got)
	}
}

func TestBashCompletionIncludesDescriptionPatch(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCompletion(NewCmdRoot(), "bash", &buf); err != nil {
		t.Fatal(err)
	}
	script := buf.String()
	for _, want := range []string{
		"# bash completion for life-ustc",
		"__life-ustc_request_completion()",
		"__life-ustc_print_completion_descriptions()",
		"__life-ustc_handle_go_custom_completion()",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("bash completion missing %q", want)
		}
	}
}

// TestAllExpectedCommandsPresent checks every documented top-level command exists.
func TestAllExpectedCommandsPresent(t *testing.T) {
	cmd := NewCmdRoot()
	for _, name := range []string{
		"catalog", "workspace", "community", "account", "admin",
		"config", "completion", "api",
	} {
		if findCommand(cmd, name) == nil {
			t.Errorf("expected top-level command %q is missing", name)
		}
	}
}

// TestCommandGroupAssignments checks that each command has the expected GroupID.
func TestCommandGroupAssignments(t *testing.T) {
	cmd := NewCmdRoot()
	checks := map[string]string{
		"catalog":    groupMain,
		"workspace":  groupMain,
		"community":  groupMain,
		"account":    groupMain,
		"admin":      groupMain,
		"config":     groupPlumbing,
		"completion": groupPlumbing,
		"api":        groupPlumbing,
	}
	for name, wantGroup := range checks {
		child := findCommand(cmd, name)
		if child == nil {
			t.Errorf("command %q not found", name)
			continue
		}
		if child.GroupID != wantGroup {
			t.Errorf("command %q: GroupID = %q, want %q", name, child.GroupID, wantGroup)
		}
	}
}

// TestShortDescriptionsNotParenWrapped ensures that Short strings are not wrapped
// in parentheses (the paren style is applied only in the rendered command list,
// not stored on the command itself).
func TestShortDescriptionsNotParenWrapped(t *testing.T) {
	cmd := NewCmdRoot()
	for _, child := range cmd.Commands() {
		if strings.HasPrefix(child.Short, "(") && strings.HasSuffix(child.Short, ")") {
			t.Errorf("command %q Short is wrapped in parens: %q", child.Name(), child.Short)
		}
	}
}

// TestParenthesizeHelper verifies the parenthesize function used in list rendering.
func TestParenthesizeHelper(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"hello", "(hello)"},
		{"(already)", "(already)"},
		{"  trim  ", "(trim)"},
		{"", ""},
	}
	for _, tc := range cases {
		got := parenthesize(tc.in)
		if got != tc.want {
			t.Errorf("parenthesize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestVersionFlag checks the version template.
func TestVersionFlag(t *testing.T) {
	cmd := NewCmdRoot()
	if cmd.Version != version {
		t.Errorf("Version = %q, want %q", cmd.Version, version)
	}
}

// TestGlobalFlagsExist verifies that all expected persistent flags are registered.
func TestGlobalFlagsExist(t *testing.T) {
	cmd := NewCmdRoot()
	for _, flag := range []string{"server", "format", "no-color", "jq", "verbose", "json"} {
		if cmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("persistent flag --%s is missing", flag)
		}
	}
}

// TestJSONFlagEquivalence checks that --json is registered as a shorthand for --format json.
func TestJSONFlagEquivalence(t *testing.T) {
	cmd := NewCmdRoot()
	jsonFlag := cmd.PersistentFlags().Lookup("json")
	if jsonFlag == nil {
		t.Fatal("--json flag missing")
	}
	if jsonFlag.Usage != "Output as JSON (shorthand for --format json)" {
		t.Errorf("--json flag usage = %q", jsonFlag.Usage)
	}
}
