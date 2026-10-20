// Package version tells which version of cech is running (ADR 14).
package version

import (
	"runtime"
	"runtime/debug"
	"strings"
)

// Dev is the version of a build that carries none: one with no stamp,
// built from a checkout rather than installed from a module version.
// go version -m shows the commit such a build came from, when go
// recorded one.
const Dev = "0.0.0-dev"

// stamped is set by a release build:
//
//	go build -ldflags "-X github.com/TrialAndTemper/cech/cli/internal/version.stamped=1.2.3" ./cmd/cech
var stamped string

// Info describes the running binary.
type Info struct {
	// Version is a SemVer version without a leading "v".
	Version string `json:"version"`
	// Go is the Go release that built the binary, such as "go1.27.2".
	Go string `json:"go"`
	// OS and Arch are the target the binary was built for.
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// Current describes the running binary.
func Current() Info {
	// Without build info, as in a binary built with no module support,
	// ReadBuildInfo returns nil.
	build, _ := debug.ReadBuildInfo()
	return resolve(stamped, build, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// resolve picks the version: the stamp, else the main module's version
// from the build info, else Dev. go install records a module version,
// or a pseudo-version such as v0.0.0-20261020120000-0123456789ab for a
// commit. go build in a checkout of cli/ records "(devel)", even at a
// cli/vX.Y.Z tag: Go stamps a version from git only for a module at the
// repository root (checked with Go 1.27.2), so releases need the stamp.
func resolve(stamp string, build *debug.BuildInfo, goVersion, goos, goarch string) Info {
	v := stamp
	if v == "" && build != nil && build.Main.Version != "(devel)" {
		v = build.Main.Version
	}
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		v = Dev
	}
	return Info{Version: v, Go: goVersion, OS: goos, Arch: goarch}
}
