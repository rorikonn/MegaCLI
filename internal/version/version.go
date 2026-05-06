package version

import (
	"runtime/debug"
	"strings"
)

// Build-time parameters set via -ldflags.

var (
	Version    = "devel"
	Commit     = "unknown"
	ReleaseBuild = "false"
)

// IsRelease returns true only when built by CI with
// -ldflags "-X ...ReleaseBuild=true".
func IsRelease() bool {
	return ReleaseBuild == "true"
}

// A user may install crush using `go install github.com/megacli/megacli@latest`.
// without -ldflags, in which case the version above is unset. As a workaround
// we use the embedded build version that *is* set when using `go install` (and
// is only set for `go install` and not for `go build`).
func init() {
	if Version != "devel" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	mainVersion := info.Main.Version
	if mainVersion != "" && mainVersion != "(devel)" {
		Version = strings.TrimSuffix(mainVersion, "+dirty")
	}
}
