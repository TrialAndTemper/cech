package app

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TrialAndTemper/cech/cli/internal/version"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata/")

// info is a fixed version, so the golden files hold the same output on
// every machine.
var info = version.Info{Version: "1.2.3", Go: "go1.27.2", OS: "linux", Arch: "amd64"}

func TestRun(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		code   int
		golden string // file in testdata/ that stdout must match; "" for none
		stderr string // text stderr must hold; "" for none
	}{
		{"version", []string{"version"}, ExitOK, "version.golden", ""},
		{"version as JSON", []string{"version", "--json"}, ExitOK, "version-json.golden", ""},
		{"version as JSON, one dash", []string{"version", "-json"}, ExitOK, "version-json.golden", ""},
		{"version help", []string{"version", "-h"}, ExitOK, "version-help.golden", ""},
		{"version long help", []string{"version", "--help"}, ExitOK, "version-help.golden", ""},
		{"help", []string{"help"}, ExitOK, "help.golden", ""},
		{"help flag", []string{"-h"}, ExitOK, "help.golden", ""},
		{"long help flag", []string{"--help"}, ExitOK, "help.golden", ""},
		{"one-dash long help flag", []string{"-help"}, ExitOK, "help.golden", ""},
		{"help with a help flag", []string{"help", "-h"}, ExitOK, "help.golden", ""},
		{"help for a command", []string{"help", "version"}, ExitOK, "version-help.golden", ""},
		{"build help", []string{"build", "-h"}, ExitOK, "build-help.golden", ""},
		{"help for build", []string{"help", "build"}, ExitOK, "build-help.golden", ""},
		{"help for help", []string{"help", "help"}, ExitOK, "help.golden", ""},
		{"version as text, by flag", []string{"version", "--json=false"}, ExitOK, "version.golden", ""},
		{"no command", nil, ExitUsage, "", "cech: no command given\n"},
		{"unknown command", []string{"frobnicate"}, ExitUsage, "", `unknown command "frobnicate"`},
		{"unknown flag before a command", []string{"--json"}, ExitUsage, "", `unknown command "--json"`},
		{"help for an unknown command", []string{"help", "frobnicate"}, ExitUsage, "", `no help for "frobnicate": unknown command`},
		{"help for two commands", []string{"help", "version", "version"}, ExitUsage, "", "help takes at most one command"},
		{"version after --", []string{"version", "--", "--json"}, ExitUsage, "", `version takes no arguments, got "--json"`},
		{"version with an argument", []string{"version", "extra"}, ExitUsage, "", `version takes no arguments, got "extra"`},
		// The flag package names a flag with one dash, however it was typed.
		{"version with an unknown flag", []string{"version", "--short"}, ExitUsage, "", "flag provided but not defined: -short"},
		{"version with a bad flag value", []string{"version", "--json=maybe"}, ExitUsage, "", `invalid boolean value "maybe" for -json`},
		{"build with --check and --dry-run", []string{"build", "--check", "--dry-run"}, ExitUsage, "", "build takes --check or --dry-run, not both"},
		{"build with an argument", []string{"build", "extra"}, ExitUsage, "", `build takes no arguments, got "extra"`},
		{"build --release with a v", []string{"build", "--release", "v0.1.0"}, ExitUsage, "", `build --release takes X.Y.Z or X.Y.Z-rc.N, got "v0.1.0"`},
		{"build --release with two numbers", []string{"build", "--release", "0.1"}, ExitUsage, "", `build --release takes X.Y.Z or X.Y.Z-rc.N, got "0.1"`},
		{"build --release with a leading zero", []string{"build", "--release", "0.01.0"}, ExitUsage, "", `build --release takes X.Y.Z or X.Y.Z-rc.N, got "0.01.0"`},
		{"build --release with rc.0", []string{"build", "--release", "0.1.0-rc.0"}, ExitUsage, "", `build --release takes X.Y.Z or X.Y.Z-rc.N, got "0.1.0-rc.0"`},
		{"build --release with another pre-release", []string{"build", "--release", "0.1.0-beta.1"}, ExitUsage, "", `build --release takes X.Y.Z or X.Y.Z-rc.N, got "0.1.0-beta.1"`},
		{"build --release empty", []string{"build", "--release", ""}, ExitUsage, "", `build --release takes X.Y.Z or X.Y.Z-rc.N, got ""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr, info)
			if code != tt.code {
				t.Errorf("exit code %d, want %d; stderr:\n%s", code, tt.code, stderr.String())
			}
			if tt.golden == "" {
				if stdout.Len() > 0 {
					t.Errorf("stdout = %q, want nothing", stdout.String())
				}
			} else {
				checkGolden(t, tt.golden, stdout.Bytes())
			}
			if tt.stderr == "" {
				if stderr.Len() > 0 {
					t.Errorf("stderr = %q, want nothing", stderr.String())
				}
			} else {
				if !strings.Contains(stderr.String(), tt.stderr) {
					t.Errorf("stderr = %q, want it to hold %q", stderr.String(), tt.stderr)
				}
				if !strings.HasSuffix(stderr.String(), "Run cech help for usage.\n") {
					t.Errorf("stderr = %q, want it to end with a pointer to help", stderr.String())
				}
			}
		})
	}
}

// failingWriter fails every write, as a closed pipe does.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

// panickingWriter panics on every write, as a bug in cech would.
type panickingWriter struct{}

func (panickingWriter) Write([]byte) (int, error) { panic("boom") }

func TestRunReportsAPanicAsAFailure(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"version"}, panickingWriter{}, &stderr, info)
	if code != ExitFailure {
		t.Errorf("exit code %d, want %d", code, ExitFailure)
	}
	if want := "cech: internal error: boom\n"; !strings.HasPrefix(stderr.String(), want) {
		t.Errorf("stderr = %q, want it to start with %q", stderr.String(), want)
	}
	if !strings.Contains(stderr.String(), "goroutine ") {
		t.Errorf("stderr = %q, want a stack trace", stderr.String())
	}
}

func TestRunReportsAFailedWrite(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"version"}, failingWriter{}, &stderr, info)
	if code != ExitFailure {
		t.Errorf("exit code %d, want %d", code, ExitFailure)
	}
	if want := "cech: cannot write the output: broken pipe\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

// checkGolden compares got with testdata/<name>, or with -update
// rewrites the file.
func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v; run go test ./internal/app -update to write it", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("stdout differs from %s\ngot:\n%s\nwant:\n%s", path, got, want)
	}
}

// TestVersionJSONEscapes checks that version --json escapes strings as
// encoding/json did: <, >, &, U+2028 and U+2029 escaped, and invalid
// UTF-8 as U+FFFD.
func TestVersionJSONEscapes(t *testing.T) {
	odd := version.Info{Version: "1<2>&3\u2028\u2029\xff", Go: "go", OS: "os", Arch: "arch"}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"version", "--json"}, &stdout, &stderr, odd); code != ExitOK {
		t.Fatalf("exit code %d; stderr:\n%s", code, stderr.String())
	}
	want := `{"version":"1\u003c2\u003e\u00263\u2028\u2029` + "\uFFFD" + `","go":"go","os":"os","arch":"arch"}` + "\n"
	if got := stdout.String(); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
