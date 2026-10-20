package version

import (
	"runtime"
	"runtime/debug"
	"testing"
)

func TestResolve(t *testing.T) {
	module := func(v string) *debug.BuildInfo {
		return &debug.BuildInfo{Main: debug.Module{Path: "github.com/TrialAndTemper/cech/cli", Version: v}}
	}
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
		{"empty module version", "", module(""), Dev},
		{"no build info", "", nil, Dev},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolve(tt.stamp, tt.build, "go1.27.2", "linux", "amd64")
			want := Info{Version: tt.want, Go: "go1.27.2", OS: "linux", Arch: "amd64"}
			if got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
}

func TestCurrent(t *testing.T) {
	// A test binary carries no stamp and no module version.
	got := Current()
	if got.Version != Dev {
		t.Errorf("Current().Version = %q, want %q", got.Version, Dev)
	}
	if got.Go != runtime.Version() || got.OS != runtime.GOOS || got.Arch != runtime.GOARCH {
		t.Errorf("Current() = %+v, want Go %s on %s/%s", got, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	}
}
