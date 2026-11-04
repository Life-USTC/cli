package root

import (
	"runtime/debug"
	"testing"
)

func TestVersionForInstalledModule(t *testing.T) {
	for _, tc := range []struct {
		name, injected, module, want string
		available                    bool
	}{
		{"installed release", "dev", "v0.2.0", "0.2.0", true},
		{"explicit release wins", "0.3.0", "v0.2.0", "0.3.0", true},
		{"local checkout", "dev", "(devel)", "dev", true},
		{"missing metadata", "dev", "", "dev", false},
		{"empty version", "dev", "", "dev", true},
		{"installed prerelease", "dev", "v0.3.0-rc.1", "0.3.0-rc.1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := executableVersion(tc.injected, func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{Main: debug.Module{Path: "github.com/Life-USTC/CLI", Version: tc.module}}, tc.available
			})
			if got != tc.want {
				t.Fatalf("version=%q, want%q", got, tc.want)
			}
		})
	}
}

func TestVersionKeepsVCSCheckoutDevelopmentLabel(t *testing.T) {
	got := executableVersion("dev", func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v0.2.0"}, Settings: []debug.BuildSetting{{Key: "vcs", Value: "git"}}}, true
	})
	if got != "dev" {
		t.Fatalf("checkout version=%q", got)
	}
}
