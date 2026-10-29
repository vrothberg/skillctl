package cli

import (
	"strings"
	"testing"
)

func TestSkillName(t *testing.T) {
	const digest = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	for _, tc := range []struct {
		input string
		want  string
	}{
		{"skill", "skill"},
		{"skill:latest", "skill"},
		{"team/skill:v1", "skill"},
		{"quay.io/team/skill", "skill"},
		{"quay.io/team/skill:v1", "skill"},
		{"quay.io/team/skill" + digest, "skill"},
		{"quay.io/team/skill:v1" + digest, "skill"},
		{"localhost:5000/skill", "skill"},
		{"localhost:5000/skill:v1", "skill"},
		{"localhost:5000/team/skill" + digest, "skill"},
		{"localhost:5000/team/skill:v1" + digest, "skill"},
		{"quay.io/team/skill.config:v1", "skill.config"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := skillName(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSkillNameRejectsInvalidReferences(t *testing.T) {
	// IPv6 registry literals are also rejected by the vendored parser.
	for _, input := range []string{":::invalid:::", "quay.io/team/skill@sha256:invalid", "docker://quay.io/team/skill", "[::1]:5000/team/skill"} {
		t.Run(input, func(t *testing.T) {
			name, err := skillName(input)
			if err == nil || !strings.Contains(err.Error(), "invalid reference format") {
				t.Fatalf("expected invalid-reference error, got %v", err)
			}
			if name != "" {
				t.Errorf("invalid reference produced name %q", name)
			}
		})
	}
}
