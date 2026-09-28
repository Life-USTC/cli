package root

import (
	"github.com/Life-USTC/CLI/internal/specification"
	"testing"
)

func TestSpecGeneratedBusinessClients(t *testing.T) {
	t.Run("openapi.cli-generated-business-client", func(t *testing.T) { specification.Run(t, semanticAdapter) })
}
func TestSpecAdminSessionBoundary(t *testing.T) {
	t.Run("cli.admin-session-boundary", func(t *testing.T) { specification.Run(t, semanticAdapter) })
}
func TestSpecBusPreferencePartialUpdate(t *testing.T) {
	t.Run("cli.bus-preference-partial-update", func(t *testing.T) { specification.Run(t, semanticAdapter) })
}
func TestSpecBusPreferenceJSONInput(t *testing.T) {
	t.Run("cli.bus-preference-json-input", func(t *testing.T) { specification.Run(t, semanticAdapter) })
}
func TestSpecExamMonitorIdentityOutput(t *testing.T) {
	t.Run("exam.cli-monitor-identity", func(t *testing.T) { specification.Run(t, semanticAdapter) })
}
func TestSpecYoungParticipationOutput(t *testing.T) {
	t.Run("young.cli-participation-fields", func(t *testing.T) { specification.Run(t, semanticAdapter) })
}
func TestSpecDescriptionContentOutput(t *testing.T) {
	t.Run("description.cli-content-output", func(t *testing.T) { specification.Run(t, semanticAdapter) })
}
func TestSpecDescriptionHistoryOutput(t *testing.T) {
	t.Run("description.cli-history-output", func(t *testing.T) { specification.Run(t, semanticAdapter) })
}
func semanticAdapter(t *testing.T, in specification.Input) specification.Observation {
	if in.Action == "architecture" {
		calls, issues := auditGeneratedBusinessCalls()
		for _, issue := range issues {
			t.Error(issue)
		}
		return specification.Observation{Calls: calls, Violations: len(issues)}
	}
	prefixes := map[string][]string{"admin": {}, "bus-partial": {"workspace", "bus-preferences", "set"}, "bus-json": {"workspace", "bus-preferences", "set"}, "exam": {"workspace", "exam"}, "young": {"catalog", "young-event", "get", "event-1"}, "publication": {"catalog", "publication"}, "description": {"community", "description", "get"}}
	prefix, ok := prefixes[in.Action]
	if !ok {
		t.Fatalf("unsupported native action %s", in.Action)
	}
	args := []string{"--server", in.ServerURL, "--format", in.Format, "--no-color"}
	args = append(args, prefix...)
	args = append(args, in.Arguments...)
	cmd := NewCmdRoot()
	commands := []string{}
	for _, c := range cmd.Commands() {
		commands = append(commands, c.Name())
	}
	in.Arguments = args
	result := specification.Execute(t, cmd, in)
	result.Commands = commands
	return result
}
