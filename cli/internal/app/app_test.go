package app

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/TrialAndTemper/cech/cli/internal/version"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata/")

// info is a fixed version, so the golden files hold the same output on
// every machine.
var info = version.Info{Version: "1.2.3", Go: "go1.27.2", OS: "linux", Arch: "amd64"}

// TestRun checks the exit code, stdout and stderr of each command line
// (ADR 15 item 5). Help is in TestCommandLine.
func TestRun(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		code   int
		golden string // file in testdata/ that stdout must match; "" for none
		// Text stderr must hold; "" for none. Where Cobra or pflag words
		// the message, only the word at fault, so their wording may change.
		stderr string
	}{
		{"version", []string{"version"}, ExitOK, "version.golden", ""},
		{"version as JSON", []string{"version", "--json"}, ExitOK, "version-json.golden", ""},
		{"no command", nil, ExitUsage, "", "cech: no command given\n"},
		{"unknown command", []string{"frobnicate"}, ExitUsage, "", "frobnicate"},
		{"help for an unknown command", []string{"help", "frobnicate"}, ExitUsage, "", `no help for "frobnicate": unknown command`},
		{"help for an unknown subcommand", []string{"help", "version", "frobnicate"}, ExitUsage, "", `no help for "version frobnicate": unknown command`},
		{"only --", []string{"--"}, ExitUsage, "", "cech: no command given\n"},
		{"unknown command after --", []string{"--", "frobnicate"}, ExitUsage, "", `unknown command "frobnicate"`},
		{"version with an argument", []string{"version", "extra"}, ExitUsage, "", `version takes no arguments, got "extra"`},
		{"completion without a shell", []string{"completion"}, ExitUsage, "", "completion takes a shell: bash, zsh, fish or powershell"},
		// ADR 15 item 5 exempts completion from --json.
		{"completion as JSON", []string{"completion", "bash", "--json"}, ExitUsage, "", "--json"},
		{"build with a long flag after one dash", []string{"build", "-check"}, ExitUsage, "", "-check"},
		{"build --json with --check and --dry-run", []string{"build", "--json", "--check", "--dry-run"}, ExitUsage, "", "build takes --check or --dry-run, not both"},
		{"build --json with an argument", []string{"build", "--json", "extra"}, ExitUsage, "", `build takes no arguments, got "extra"`},
		{"build --release with a v", []string{"build", "--release", "v0.1.0"}, ExitUsage, "", `build --release takes X.Y.Z or X.Y.Z-rc.N, got "v0.1.0"`},
		{"build --release with two numbers", []string{"build", "--release", "0.1"}, ExitUsage, "", `build --release takes X.Y.Z or X.Y.Z-rc.N, got "0.1"`},
		{"build --release with a leading zero", []string{"build", "--release", "0.01.0"}, ExitUsage, "", `build --release takes X.Y.Z or X.Y.Z-rc.N, got "0.01.0"`},
		{"build --release with rc.0", []string{"build", "--release", "0.1.0-rc.0"}, ExitUsage, "", `build --release takes X.Y.Z or X.Y.Z-rc.N, got "0.1.0-rc.0"`},
		{"build --release with another pre-release", []string{"build", "--release", "0.1.0-beta.1"}, ExitUsage, "", `build --release takes X.Y.Z or X.Y.Z-rc.N, got "0.1.0-beta.1"`},
		{"build --json -C empty", []string{"build", "--json", "-C", ""}, ExitUsage, "", `build -C takes a directory, got ""`},
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
				if !strings.HasPrefix(stderr.String(), "cech: ") || !strings.Contains(stderr.String(), tt.stderr) {
					t.Errorf("stderr = %q, want it to start with %q and hold %q", stderr.String(), "cech: ", tt.stderr)
				}
				if !strings.HasSuffix(stderr.String(), "Run cech help for usage.\n") {
					t.Errorf("stderr = %q, want it to end with a pointer to help", stderr.String())
				}
			}
		})
	}
}

// TestCommandLine writes the help of every command of cech into one
// golden file, so a change to any command, flag or help text shows in
// it, and a new command adds itself.
func TestCommandLine(t *testing.T) {
	var all bytes.Buffer
	var visit func(c *cobra.Command)
	visit = func(c *cobra.Command) {
		args := strings.Fields(c.CommandPath())[1:]
		args = append(args, "--help")
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr, info); code != ExitOK || stderr.Len() > 0 {
			t.Errorf("%s: exit code %d, stderr %q; want %d and nothing", strings.Join(args, " "), code, stderr.String(), ExitOK)
		}
		fmt.Fprintf(&all, "=== %s\n%s", c.CommandPath(), stdout.String())
		// The help of completion's shells is Cobra's; their names and
		// cech's one-line summaries show in the help of completion.
		if c.Name() == "completion" {
			return
		}
		for _, sub := range c.Commands() {
			if !sub.Hidden {
				visit(sub)
			}
		}
	}
	var code int
	visit(newRoot(io.Discard, io.Discard, io.Discard, info, &code))
	checkGolden(t, "commandline.golden", all.Bytes())
}

// TestHelp checks the help that cech's own choices give: cech help, a
// help flag before a command, and cech help <command>, whose help must
// list the help flag that cech adds to it.
func TestHelp(t *testing.T) {
	tests := []struct {
		args []string
		want string // the command whose help stdout must be
	}{
		{[]string{"help"}, "cech"},
		{[]string{"--help", "build"}, "cech build"},
		{[]string{"help", "version"}, "cech version"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, &stdout, &stderr, info); code != ExitOK || stderr.Len() > 0 {
				t.Fatalf("exit code %d, stderr %q; want %d and nothing", code, stderr.String(), ExitOK)
			}
			out := stdout.String()
			found := false
			for _, form := range []string{"", " [flags]", " [command]"} {
				found = found || strings.Contains(out, "Usage:\n  "+tt.want+form+"\n")
			}
			if !found {
				t.Errorf("stdout is not the help of %s:\n%s", tt.want, out)
			}
			if !strings.Contains(out, "-h, --help") {
				t.Errorf("the help of %s does not list -h, --help:\n%s", tt.want, out)
			}
		})
	}
}

// TestCompletion checks that cech completion prints a script on stdout
// and exits 0. The scripts of each shell, and what they hold, are Cobra's.
func TestCompletion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"completion", "bash"}, &stdout, &stderr, info)
	if code != ExitOK || stderr.Len() > 0 {
		t.Errorf("exit code %d, stderr %q; want %d and nothing", code, stderr.String(), ExitOK)
	}
	if !strings.Contains(stdout.String(), "__complete") {
		t.Errorf("stdout holds no script that calls __complete:\n%s", stdout.String())
	}
}

// TestCompleteHelp checks the completion that cech's choices give: a
// command path after help, from Cobra's help command, which cech keeps,
// no file names after version or build, which take no argument, and
// only directories after -C.
func TestCompleteHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string // stdout: one name and its description a line, then the directive
	}{
		{"a path", []string{"__complete", "help", "completion", "b"}, "bash\tprint the completion script of bash\n:4\n"},
		{"after version", []string{"__complete", "version", ""}, ":4\n"},
		{"after build", []string{"__complete", "build", ""}, ":4\n"},
		{"after -C", []string{"__complete", "build", "-C", ""}, ":16\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, &stdout, &stderr, info); code != ExitOK {
				t.Errorf("exit code %d, want %d; stderr:\n%s", code, ExitOK, stderr.String())
			}
			if stdout.String() != tt.want {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.want)
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

// TestRunReportsAFailedWrite checks that a failed write exits 1, whether
// a command or Cobra writes. The build cases list the changes a dry run
// of a fixture finds, which succeeds, as text and as JSON.
func TestRunReportsAFailedWrite(t *testing.T) {
	fixture := filepath.Join("testdata", "build", "acme", "in")
	for _, args := range [][]string{
		{"version"},
		{"help"},
		{"completion", "bash"},
		{"build", "--dry-run", "-C", fixture},
		{"build", "--dry-run", "--json", "-C", fixture},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stderr bytes.Buffer
			code := run(args, failingWriter{}, &stderr, info)
			if code != ExitFailure {
				t.Errorf("exit code %d, want %d", code, ExitFailure)
			}
			if want := "cech: cannot write the output: broken pipe\n"; stderr.String() != want {
				t.Errorf("stderr = %q, want %q", stderr.String(), want)
			}
		})
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
