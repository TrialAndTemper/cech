#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests for the Go gates: each case plants files in its own copy of the
# CLI module and checks the verdict of scripts/check-go. The cases that
# need the network, such as a real govulncheck run, use a fake tool.
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

@test "clean module" {
  run_go lint
  assert_status 0
  assert_output_has "go: lint ok"
  run_go format
  assert_status 0
  assert_output_has "go: format ok"
  run_go test
  assert_status 0
  assert_output_has "go: test ok"
}

@test "help" {
  run_go --help
  assert_status 0
  assert_output_has "Usage: mise run lint"
}

@test "no mode" {
  run_go
  assert_status 2
  assert_output_has "no mode given"
}

@test "unknown mode" {
  run_go build
  assert_status 2
  assert_output_has "unknown mode: build"
}

@test "extra argument" {
  run_go lint extra
  assert_status 2
  assert_output_has "unexpected argument: extra"
}

@test "no module" {
  rm -- "${REPO}/cli/go.mod"
  run_go lint
  assert_status 2
  assert_output_has "no Go module found at cli/go.mod"
}

# Lint

@test "vet finding" {
  plant internal/bad/bad.go 'package bad

import "fmt"

// Bad prints a string as a number.
func Bad() { fmt.Printf("%d\n", "x") }
'
  run_go lint
  assert_status 1
  assert_output_has "fmt.Printf format %d has arg"
  assert_output_has "go: lint findings"
}

@test "unchecked error" {
  plant internal/bad/bad.go 'package bad

import "os"

// Remove drops its error.
func Remove() { os.Remove("x") }
'
  run_go lint
  assert_status 1
  assert_output_has "(errcheck)"
}

@test "untidy go.mod" {
  printf '\nrequire example.com/unused v1.0.0\n' >> "${REPO}/cli/go.mod"
  run_go lint
  assert_status 1
  assert_output_has "-require example.com/unused v1.0.0"
}

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

@test "the pinned Go runs, not one go would download" {
  # go stops before it builds anything, which this script counts as a
  # finding, as it does code that does not build.
  sed 's/^go .*/go 1.999.0/' "${REPO}/cli/go.mod" > "${BATS_TEST_TMPDIR}/go.mod"
  cp -- "${BATS_TEST_TMPDIR}/go.mod" "${REPO}/cli/go.mod"
  GOTOOLCHAIN=auto run_go test
  assert_status 1
  assert_output_has "GOTOOLCHAIN=local"
}

@test "a go.work file above the module is ignored" {
  printf 'go 1.27.0\n\nuse ./missing\n' > "${REPO}/go.work"
  run_go lint
  assert_status 0
}

@test "linter that cannot run" {
  fake_tool golangci-lint 'exit 3'
  run_go lint
  assert_status 2
  assert_output_has "golangci-lint failed (exit 3)"
}

# Format

@test "format finding, then fix" {
  plant internal/bad/bad.go 'package bad
// Bad is badly formatted.
var  Bad = 1
'
  run_go format
  assert_status 1
  assert_output_has "internal/bad/bad.go"
  run_go fix
  assert_status 0
  assert_output_has "go: formatted"
  run_go format
  assert_status 0
}

@test "formatter that fails while fixing" {
  fake_tool golangci-lint 'exit 3'
  run_go fix
  assert_status 2
  assert_output_has "golangci-lint fmt failed"
}

# Test

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

# fake_go <CGO_ENABLED> <GOOS> <GOARCH> [race build status]: a fake go
# that reports that setting, fails a race build with the status (0 by
# default), and prints the arguments of any other command.
fake_go() {
  fake_tool go "case \"\$1:\${2:-}\" in
  env:CGO_ENABLED) echo $1 ;;
  env:GOOS) echo $2 ;;
  env:GOARCH) echo $3 ;;
  build:-race) if [ ${4:-0} != 0 ]; then echo 'race runtime: cannot build' >&2; exit ${4:-0}; fi ;;
  *) printf '%s\\n' \"\$*\" ;;
esac"
}

@test "tests run with -race where cgo is on" {
  fake_go 1 linux amd64
  run_go test
  assert_status 0
  assert_output_has "test -race ./..."
}

@test "code that does not build leaves -race on" {
  plant internal/bad/bad.go 'package bad

// Bad does not build.
func Bad() { undefined() }
'
  run_go test
  assert_status 1
  assert_output_has "undefined: undefined"
  refute_output_has "race detector does not build"
}

@test "tests run without -race where the race detector does not build" {
  fake_go 1 windows amd64 1
  run_go test
  assert_status 0
  assert_output_has "the race detector does not build here, so the tests run without -race"
  assert_output_has "race runtime: cannot build"
  assert_output_has "test ./..."
  refute_output_has "-race ./..."
}

@test "tests run without -race where the platform has no race detector" {
  fake_go 1 linux 386
  run_go test
  assert_status 0
  assert_output_has "linux/386 has no race detector, so the tests run without -race"
  assert_output_has "test ./..."
  refute_output_has "-race ./..."
}

@test "tests run without -race where cgo is off" {
  fake_go 0 linux amd64
  run_go test
  assert_status 0
  assert_output_has "cgo is off, so the tests run without -race"
  assert_output_has "test ./..."
  refute_output_has "-race ./..."
}

# Vulnerabilities

@test "vulnerability found" {
  fake_tool govulncheck 'echo "Vulnerability #1: GO-0000-0000"; exit 3'
  run_go vuln
  assert_status 1
  assert_output_has "GO-0000-0000"
  assert_output_has "go: vuln findings"
}

@test "vulnerability check that cannot run" {
  fake_tool govulncheck 'echo "cannot reach the database" >&2; exit 1'
  run_go vuln
  assert_status 2
  assert_output_has "govulncheck failed (exit 1)"
}

# Tools

@test "missing tool" {
  PATH=/usr/bin:/bin
  if command -v golangci-lint > /dev/null; then skip "golangci-lint is installed system-wide"; fi
  if command -v go > /dev/null; then skip "go is installed system-wide"; fi
  run_go lint
  assert_status 2
  assert_output_has "go not found"
}

@test "missing linter" {
  local go
  go=$(command -v go)
  PATH=$(dirname -- "${go}"):/usr/bin:/bin
  if command -v golangci-lint > /dev/null; then skip "golangci-lint is installed system-wide"; fi
  run_go format
  assert_status 2
  assert_output_has "golangci-lint not found"
}

@test "missing vulnerability checker" {
  local go
  go=$(command -v go)
  PATH=$(dirname -- "${go}"):/usr/bin:/bin
  if command -v govulncheck > /dev/null; then skip "govulncheck is installed system-wide"; fi
  run_go vuln
  assert_status 2
  assert_output_has "govulncheck not found"
}
