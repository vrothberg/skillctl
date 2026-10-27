package store

import (
	"strings"
	"testing"
)

func TestNormalizeImageReference(t *testing.T) {
	const digestSuffix = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Mirror libimage's TestNormalizeTaggedDigestedString cases: normalize
	// combined references without qualifying names or adding default tags.
	for _, tc := range []struct {
		input   string
		want    string
		wantErr string
	}{
		{"$$garbage", "", "invalid reference format"},
		{"", "", "repository name must have at least one component"},
		{"fedora", "fedora", ""},
		{"fedora:tag", "fedora:tag", ""},
		{digestSuffix, "", "invalid reference format"},
		{"docker://fedora:latest", "", "invalid reference format"},
		{"docker://fedora:latest" + digestSuffix, "", "invalid reference format"},
		{"fedora" + digestSuffix, "fedora" + digestSuffix, ""},
		{"fedora:latest" + digestSuffix, "fedora" + digestSuffix, ""},
		{"repo/fedora:123456" + digestSuffix, "repo/fedora" + digestSuffix, ""},
		{"quay.io/repo/fedora:tag" + digestSuffix, "quay.io/repo/fedora" + digestSuffix, ""},
		{"localhost/fedora:anothertag" + digestSuffix, "localhost/fedora" + digestSuffix, ""},
		{"localhost:5000/fedora:v1.2.3.4.5" + digestSuffix, "localhost:5000/fedora" + digestSuffix, ""},
		{"quay.io/repo/fedora", "quay.io/repo/fedora", ""},
		{"quay.io/repo/fedora:v1", "quay.io/repo/fedora:v1", ""},
		{"quay.io/repo/fedora" + digestSuffix, "quay.io/repo/fedora" + digestSuffix, ""},
		{"quay.io/repo/fedora:tag@sha256:invalid", "", "invalid reference format"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := normalizeImageReference(tc.input)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("got error %v, want error containing %q", err, tc.wantErr)
				}
				if got != "" {
					t.Errorf("got %q for invalid reference, want empty result", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
