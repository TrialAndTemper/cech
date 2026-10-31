package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestBinary builds cech the way a release stamps it and runs the
// binary, so it covers main, its exit codes and the stamp's path.
func TestBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary")
	}
	// mise run test puts the pinned go on PATH; a missing go must not
	// pass for a skipped test.
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("go not on PATH: %v", err)
	}
	bin := filepath.Join(t.TempDir(), "cech")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	// -buildvcs=false: a build stops when it cannot read the git state,
	// as in an exported tree, and the stamp gives the version anyway.
	build := exec.CommandContext(t.Context(), goTool, "build", "-buildvcs=false", "-o", bin,
		"-ldflags", "-X github.com/TrialAndTemper/cech/cli/internal/version.stamped=9.8.7", ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	tests := []struct {
		name   string
		args   []string
		code   int
		stdout string
		stderr string // text stderr must hold; "" for none
	}{
		{"version", []string{"version"}, 0, "cech 9.8.7\n", ""},
		{"unknown command", []string{"frobnicate"}, 2, "", "Run cech help for usage."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := exec.CommandContext(t.Context(), bin, tt.args...)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			code := exitCode(t, cmd.Run())
			if code != tt.code {
				t.Errorf("exit code %d, want %d", code, tt.code)
			}
			if stdout.String() != tt.stdout {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.stdout)
			}
			if tt.stderr == "" {
				if stderr.Len() > 0 {
					t.Errorf("stderr = %q, want nothing", stderr.String())
				}
			} else if !strings.Contains(stderr.String(), tt.stderr) {
				t.Errorf("stderr = %q, want it to hold %q", stderr.String(), tt.stderr)
			}
		})
	}

	// A reader that has gone, as in cech version | true, must not kill
	// cech with SIGPIPE: the failed write exits 1.
	t.Run("stdout closed by its reader", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = w.Close() }()
		var stderr bytes.Buffer
		cmd := exec.CommandContext(t.Context(), bin, "version")
		cmd.Stdout = w
		cmd.Stderr = &stderr
		if code := exitCode(t, cmd.Run()); code != 1 {
			t.Errorf("exit code %d, want 1; stderr: %s", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "cech: cannot write the output") {
			t.Errorf("stderr = %q, want it to report the failed write", stderr.String())
		}
	})
}

// exitCode returns the exit code of a finished command, failing the test
// if the command did not run or was killed.
func exitCode(t *testing.T, err error) int {
	t.Helper()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit) && exit.ExitCode() >= 0:
		return exit.ExitCode()
	default:
		t.Fatalf("cech did not exit normally: %v", err)
		return -1
	}
}
