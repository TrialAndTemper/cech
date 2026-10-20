package version

import (
	"regexp"
	"runtime"
	"runtime/debug"
	"testing"
)

func TestResolve(t *testing.T) {
	module := func(v string, settings ...string) *debug.BuildInfo {
		b := &debug.BuildInfo{Main: debug.Module{Path: "github.com/TrialAndTemper/cech/cli", Version: v}}
		for i := 0; i+1 < len(settings); i += 2 {
			b.Settings = append(b.Settings, debug.BuildSetting{Key: settings[i], Value: settings[i+1]})
		}
		return b
	}
	const rev = "0123456789abcdef0123456789abcdef01234567"
	tests := []struct {
		name  string
		stamp string
		build *debug.BuildInfo
		want  string
	}{
		{"stamp wins over the module version", "1.2.3", module("v0.9.0"), "1.2.3"},
		{"stamp loses its v", "v1.2.3", nil, "1.2.3"},
		{"prerelease stamp", "1.2.3-rc.1", nil, "1.2.3-rc.1"},
		{"module version from go install", "", module("v0.1.0"), "0.1.0"},
		{"pseudo-version from go install of a commit", "", module("v0.0.0-20261020120000-0123456789ab"), "0.0.0-20261020120000-0123456789ab"},
		{"devel build", "", module("(devel)"), Dev},
		{"devel build from a clean checkout", "", module("(devel)", "vcs", "git", "vcs.revision", rev, "vcs.modified", "false"), Dev + "+0123456789ab"},
		{"devel build from a changed checkout", "", module("(devel)", "vcs.revision", rev, "vcs.modified", "true"), Dev + "+0123456789ab.dirty"},
		{"short revision", "", module("(devel)", "vcs.revision", "abc"), Dev + "+abc"},
		{"changes but no revision", "", module("(devel)", "vcs.modified", "true"), Dev},
		{"revision that is not SemVer", "", module("(devel)", "vcs", "bzr", "vcs.revision", "dev@example.com-20261020120000-abc"), Dev},
		{"stamp ignores the commit", "1.2.3", module("(devel)", "vcs.revision", rev), "1.2.3"},
		{"empty module version", "", module(""), Dev},
		{"no build info", "", nil, Dev},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolve(tt.stamp, tt.build, "go1.27.2", "linux", "amd64")
			want := Info{Version: tt.want, Go: "go1.27.2", OS: "linux", Arch: "amd64"}
			if got != want {
				t.Errorf("resolve(%q) = %+v, want %+v", tt.stamp, got, want)
			}
		})
	}
}

func TestCurrent(t *testing.T) {
	// A test binary carries no stamp and no module version.
	got := Current()
	if !regexp.MustCompile(`^0\.0\.0-dev(\+[0-9A-Za-z-]{1,12}(\.dirty)?)?$`).MatchString(got.Version) {
		t.Errorf("Current().Version = %q, want 0.0.0-dev with an optional commit", got.Version)
	}
	if got.Go != runtime.Version() || got.OS != runtime.GOOS || got.Arch != runtime.GOARCH {
		t.Errorf("Current() = %+v, want Go %s on %s/%s", got, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	}
}
