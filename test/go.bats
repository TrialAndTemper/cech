#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests of scripts/check-go, the script behind the CLI's Go gates; the
# CLI itself is tested by go test in cli/. As ADR 4 asks of gate tests,
# each case guards a way the gates could pass when they should fail,
# where review would likely miss it, and says which. The rest of the script was
# checked by hand when it was added.
#
# Requires: bats (version in mise.toml), and the tools that check-go
# requires.

load helpers

setup_file() {
  setup_suite_env
  export TEMPLATE="${BATS_FILE_TMPDIR}/template"
  local f
  mkdir -p "${TEMPLATE}/scripts"
  cp -- "${SRC}/scripts/check-go" "${TEMPLATE}/scripts/"
  cp -R -- "${SRC}/scripts/lib" "${TEMPLATE}/scripts/"
  git -C "${SRC}" ls-files -- cli > "${BATS_FILE_TMPDIR}/tracked"
  while IFS= read -r f; do
    if [[ -f ${SRC}/${f} ]]; then
      mkdir -p -- "$(dirname -- "${TEMPLATE}/${f}")"
      cp -p -- "${SRC}/${f}" "${TEMPLATE}/${f}"
    fi
  done < "${BATS_FILE_TMPDIR}/tracked"
}

setup() {
  use_template
  # Keep go off the network: a case that needs a module fails at once.
  export GOPROXY=off
}

# run_go <mode> [args...]: run the Go check.
run_go() {
  run "${REPO}/scripts/check-go" "$@"
}

# plant <path> <text>: write a file under the module.
plant() {
  mkdir -p -- "$(dirname -- "${REPO}/cli/$1")"
  printf '%s' "$2" > "${REPO}/cli/$1"
}

# Lint

# Guards a golangci-lint bump that stops errcheck reporting, and a check
# the script skips after an earlier one's finding.
@test "lint runs every check after a finding" {
  plant internal/bad/bad.go 'package bad

import (
	"fmt"
	"os"
)

// Bad prints a string as a number.
func Bad() { fmt.Printf("%d\n", "x") }

// Remove drops its error.
func Remove() { os.Remove("x") }
'
  printf '\nrequire example.com/unused v1.0.0\n' >> "${REPO}/cli/go.mod"
  run_go lint
  assert_status 1
  assert_output_has "fmt.Printf format %d has arg"
  assert_output_has "(errcheck)"
  assert_output_has "-require example.com/unused v1.0.0"
}

# Format

# Guards a golangci-lint bump after which fmt --diff exits 0 on a diff.
@test "format finding" {
  plant internal/bad/bad.go 'package bad
// Bad is badly formatted.
var  Bad = 1
'
  run_go format
  assert_status 1
  assert_output_has "internal/bad/bad.go"
}

# Test

# Guards the exit status of go test lost under dash or macOS sh, which
# would let every Go test fail unseen. bats does not run on Windows, so
# Git Bash is not covered here.
@test "failing test" {
  plant internal/bad/bad_test.go 'package bad

import "testing"

func TestFails(t *testing.T) { t.Fatal("planted failure") }
'
  run_go test
  assert_status 1
  assert_output_has "planted failure"
  assert_output_has "go: test findings"
}

# Guards GOTOOLCHAIN=local lost from check-go, which shows only when a
# later go.mod bump makes go download a newer toolchain and run it
# instead of the pinned one.
@test "the pinned Go runs, not one go would download" {
  # go stops before it builds anything, which this script counts as a
  # finding, as it does code that does not build.
  sed 's/^go .*/go 1.999.0/' "${REPO}/cli/go.mod" > "${BATS_TEST_TMPDIR}/go.mod"
  cp -- "${BATS_TEST_TMPDIR}/go.mod" "${REPO}/cli/go.mod"
  GOTOOLCHAIN=auto run_go test
  assert_status 1
  assert_output_has "GOTOOLCHAIN=local"
}

# fake_go: a fake go on linux/amd64 with cgo on, whose race runtime
# builds; it prints the arguments of any other command.
fake_go() {
  # shellcheck disable=SC2016 # the fake expands them when it runs
  fake_tool go 'case "$1:${2:-}" in
  env:CGO_ENABLED) echo 1 ;;
  env:GOOS) echo linux ;;
  env:GOARCH) echo amd64 ;;
  build:-race) ;;
  *) printf "%s\\n" "$*" ;;
esac'
}

# Guards a change to check-go that drops -race on linux/amd64 with cgo
# on: a reviewer without cgo or a race detector sees no difference, and
# an accidental drop prints nothing.
@test "tests run with -race where cgo is on" {
  fake_go
  run_go test
  assert_status 0
  assert_output_has "test -race ./..."
}
