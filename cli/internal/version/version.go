// Package version tells which version of cech is running (ADR 14).
package version

import (
	"runtime"
	"runtime/debug"
	"strings"
)

// Dev is the version of a build that carries none: one with no stamp,
// built from a checkout rather than installed from a module version.
// When the build info names the commit, resolve adds it as SemVer build
// metadata, as in 0.0.0-dev+0123456789ab.dirty.
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
// from the build info, else Dev with the commit, if known. go install
// records a module version, or a pseudo-version such as
// v0.0.0-20261020120000-0123456789ab for a commit. go build in a
// checkout of cli/ records "(devel)", even at a cli/vX.Y.Z tag: Go
// stamps a version from git only for a module at the repository root
// (checked with Go 1.27.2), so releases need the stamp.
func resolve(stamp string, build *debug.BuildInfo, goVersion, goos, goarch string) Info {
	v := stamp
	if v == "" && build != nil && build.Main.Version != "(devel)" {
		v = build.Main.Version
	}
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		v = Dev + commit(build)
	}
	return Info{Version: v, Go: goVersion, OS: goos, Arch: goarch}
}

// commit returns "+<revision>" for a build that go recorded from a
// checkout, its first 12 characters, with ".dirty" when the checkout had
// changes; otherwise "". A revision that is not valid SemVer build
// metadata, as a Bazaar one with "@" in it, is left out.
func commit(build *debug.BuildInfo) string {
	if build == nil {
		return ""
	}
	var rev, dirty string
	for _, s := range build.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = ".dirty"
			}
		}
	}
	rev = rev[:min(len(rev), 12)]
	if rev == "" || strings.ContainsFunc(rev, notIdentifier) {
		return ""
	}
	return "+" + rev + dirty
}

// notIdentifier reports whether r cannot appear in a SemVer identifier,
// which holds only ASCII letters, digits and hyphens.
func notIdentifier(r rune) bool {
	switch {
	case r == '-', '0' <= r && r <= '9', 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z':
		return false
	default:
		return true
	}
}
