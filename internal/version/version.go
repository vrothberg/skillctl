package version

import (
	"runtime/debug"
	"strings"
)

// Version is the single source of truth for skillctl's version and is
// bumped manually. Release tags are cut from it (e.g. "0.1.0" -> v0.1.0).
var Version = "0.1.0-dev"

// Commit identifies the exact source the binary was built from. It is
// derived from the Go build info, so it works for all build paths:
//
//   - `go install m@version` binaries carry the resolved module version;
//     pseudo-versions end in the commit hash, e.g.
//     v0.0.0-20261026090134-a0b1b4f10e22.
//   - binaries built from a checkout carry the vcs.revision setting.
var Commit = "unknown"

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		Commit = commitFromBuildInfo(info)
	}
}

func commitFromBuildInfo(info *debug.BuildInfo) string {
	if v := info.Main.Version; v != "" && v != "(devel)" {
		if i := strings.LastIndexByte(v, '-'); i != -1 {
			return v[i+1:]
		}
		// Tagged module versions carry no commit hash.
		return v
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 12 {
			return s.Value[:12]
		}
	}
	return "unknown"
}
