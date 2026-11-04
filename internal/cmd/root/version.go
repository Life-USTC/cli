package root

import (
	"runtime/debug"
	"strings"
)

// Explicit release ldflags win; go install embeds its tagged module version.
func executableVersion(injected string, readBuildInfo func() (*debug.BuildInfo, bool)) string {
	if injected != "" && injected != "dev" {
		return injected
	}
	if info, ok := readBuildInfo(); ok && info != nil && info.Main.Version != "" && info.Main.Version != "(devel)" {
		for _, setting := range info.Settings {
			if setting.Key == "vcs" {
				return "dev"
			}
		}
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return "dev"
}
