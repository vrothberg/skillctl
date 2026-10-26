package version

import (
	"runtime/debug"
	"testing"
)

func TestCommitFromBuildInfo(t *testing.T) {
	tests := []struct {
		name     string
		info     *debug.BuildInfo
		expected string
	}{
		{
			name:     "pseudo-version from go install",
			info:     &debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20261026090134-a0b1b4f10e22"}},
			expected: "a0b1b4f10e22",
		},
		{
			name:     "pseudo-version of a dirty local build",
			info:     &debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20261026090134-a0b1b4f10e22+dirty"}},
			expected: "a0b1b4f10e22+dirty",
		},
		{
			name:     "tagged module version carries no hash",
			info:     &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}},
			expected: "v0.1.0",
		},
		{
			name: "devel falls back to vcs.revision",
			info: &debug.BuildInfo{
				Main:     debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "a0b1b4f10e22c5a1ba979cbc46c5d80e0694a4d5"}},
			},
			expected: "a0b1b4f10e22",
		},
		{
			name:     "no version and no vcs info",
			info:     &debug.BuildInfo{},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commitFromBuildInfo(tt.info); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
