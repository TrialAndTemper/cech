#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests for the shell lint and format gates: each case plants files in
# its own copy of a throwaway repository and checks the verdict of
# scripts/check-shell.
#
# Requires: bats (version in mise.toml), git >= 2.28 (init -b), and the
# tools that check-shell requires.

load helpers

setup_file() {
  setup_suite_env
  export TEMPLATE="${BATS_FILE_TMPDIR}/template"
  REPO=${TEMPLATE}
  mkdir -p "${REPO}/scripts"
  cp -- "${SRC}/scripts/check-shell" "${REPO}/scripts/"
  cp -- "${SRC}/.shellcheckrc" "${SRC}/.editorconfig" "${REPO}/"
  commit_template
}

setup() {
  use_template
}

# run_shell <mode> [args...]: run the shell check.
run_shell() {
  run "${REPO}/scripts/check-shell" "$@"
}

# plant <755 | 644> <path> <text>: stage a file with the given text and
# mode. The mode is set in the index too, since Git for Windows does not
# take it from the file system.
plant() {
  mkdir -p -- "$(dirname -- "${REPO}/$2")"
  printf '%s' "$3" > "${REPO}/$2"
  chmod "$1" "${REPO}/$2"
  case "$1" in
    755) g add --chmod=+x -- "$2" ;;
    *) g add --chmod=-x -- "$2" ;;
  esac
}

# plant_suite <755 | 644> <line>...: stage test/x.bats with the given
# lines. bats rewrites lines of this file that start with @test, so the
# planted suite is passed as quoted lines instead.
plant_suite() {
  local mode=$1
  shift
  plant "${mode}" test/x.bats "$(printf '%s\n' "$@")
"
}

# fake_tool <tool> <script body>: put a fake tool first on PATH for the
# rest of the case. In the body, REAL names the real tool.
fake_tool() {
  local real
  real=$(command -v "$1")
  if [[ ${real} == *"'"* ]]; then
    printf 'cannot wrap a path that holds a quote: %s\n' "${real}"
    return 1
  fi
  mkdir -p -- "${BATS_TEST_TMPDIR}/bin"
  printf "#!/bin/sh\nREAL='%s'\n%s\n" "${real}" "$2" > "${BATS_TEST_TMPDIR}/bin/$1"
  chmod 755 "${BATS_TEST_TMPDIR}/bin/$1"
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"
}

@test "clean code" {
  run_shell lint
  assert_status 0
  assert_output_has "shell: lint ok (files: 1)"
  run_shell format
  assert_status 0
  assert_output_has "shell: format ok (files: 1)"
}

@test "help" {
  run_shell --help
  assert_status 0
  assert_output_has "Usage: mise run lint"
}

@test "no mode" {
  run_shell
  assert_status 2
  assert_output_has "no mode given"
}

@test "unknown mode" {
  run_shell shell
  assert_status 2
  assert_output_has "unknown mode: shell"
}

@test "extra argument" {
  run_shell lint extra
  assert_status 2
  assert_output_has "unexpected argument: extra"
}

# Scripts

@test "lint finding" {
  # shellcheck disable=SC2016 # the planted script must hold a literal $1
  plant 755 scripts/lint '#!/bin/sh
echo $1
'
  run_shell lint
  assert_status 1
  assert_output_has "SC2086"
}

@test "format finding, then fix" {
  plant 755 scripts/format '#!/bin/sh
if true; then
echo x
fi
'
  run_shell lint
  assert_status 0
  run_shell format
  assert_status 1
  assert_output_has "scripts/format"
  run_shell fix
  assert_status 0
  assert_output_has "shell: formatted (files: 2)"
  run_shell format
  assert_status 0
}

@test "format modes skip rule findings" {
  plant 755 scripts/env-sh '#!/usr/bin/env sh
:
'
  run_shell format
  assert_status 0
  run_shell fix
  assert_status 0
}

@test "other first line" {
  plant 755 scripts/env-sh '#!/usr/bin/env sh
:
'
  run_shell lint
  assert_status 1
  assert_output_has "scripts/env-sh: first line is not #!/bin/sh"
}

@test "script that is not executable" {
  plant 644 scripts/no-x '#!/bin/sh
:
'
  run_shell lint
  assert_status 1
  assert_output_has "scripts/no-x: has a shebang but is not executable"
}

@test "non-executable file is skipped" {
  plant 644 scripts/notes.md 'Not a script.
'
  run_shell lint
  assert_status 0
}

@test "deleted but unstaged file is skipped" {
  plant 755 scripts/gone '#!/bin/sh
:
'
  rm -- "${REPO}/scripts/gone"
  run_shell lint
  assert_status 0
}

# Libraries

@test "library lint finding" {
  # shellcheck disable=SC2016 # the planted library must hold a literal $1
  plant 644 scripts/lib/bad.sh '# shellcheck shell=sh
echo $1
'
  run_shell lint
  assert_status 1
  assert_output_has "SC2086"
}

@test "library without its first line" {
  plant 644 scripts/lib/plain.sh '# Helpers.
:
'
  run_shell lint
  assert_status 1
  assert_output_has "scripts/lib/plain.sh: a library must not be executable"
}

@test "executable library" {
  plant 755 scripts/lib/run.sh '# shellcheck shell=sh
:
'
  run_shell lint
  assert_status 1
  assert_output_has "scripts/lib/run.sh: a library must not be executable"
}

# Tests

@test "bats suite is linted" {
  # shellcheck disable=SC2016 # the planted suite must hold a literal $1
  plant_suite 644 '#!/usr/bin/env bats' '# shellcheck shell=bats' '' \
    '@test "x" {' '  echo $1' '}'
  run_shell lint
  assert_status 1
  assert_output_has "SC2086"
}

@test "clean bats suite and helper" {
  # shellcheck disable=SC2016 # the planted files hold literal expansions
  plant_suite 644 '#!/usr/bin/env bats' '# shellcheck shell=bats' '' \
    '@test "x" {' '  local v=x' '  test "${v}" = x' '}'
  plant 644 test/h.bash '# shellcheck shell=bash
h() {
  test "$#" -eq 0
}
'
  run_shell lint
  assert_status 0
  assert_output_has "shell: lint ok (files: 3)"
}

@test "bats suite without its dialect line" {
  plant_suite 644 '#!/usr/bin/env bats' '' '@test "x" {' '  :' '}'
  run_shell lint
  assert_status 1
  assert_output_has "test/x.bats: a bats suite must not be executable"
}

@test "executable bats suite" {
  plant_suite 755 '#!/usr/bin/env bats' '# shellcheck shell=bats' '' \
    '@test "x" {' '  :' '}'
  run_shell lint
  assert_status 1
  assert_output_has "test/x.bats: a bats suite must not be executable"
}

@test "bare condition in a bats suite" {
  # shellcheck disable=SC2016 # the planted suite holds literal expansions
  plant_suite 644 '#!/usr/bin/env bats' '# shellcheck shell=bats' '' \
    '@test "x" {' '  [[ -n ${HOME} ]]' '  true' '}'
  run_shell lint
  assert_status 1
  assert_output_has "test/x.bats:5:"
  assert_output_has "does not fail a case on bash 3.2"
}

@test "executable bash helper" {
  plant 755 test/h.bash '# shellcheck shell=bash
h() {
  :
}
'
  run_shell lint
  assert_status 1
  assert_output_has "test/h.bash: a bash helper must not be executable"
}

@test "bash helper without its first line" {
  plant 644 test/h.bash 'h() {
  :
}
'
  run_shell lint
  assert_status 1
  assert_output_has "test/h.bash: a bash helper must not be executable"
}

@test "executable test fixture with a shebang is skipped" {
  # shellcheck disable=SC2016 # the fixture holds a literal $1 that must not be linted
  plant 755 test/fixture.sh '#!/usr/bin/env bash
echo "$1"
'
  run_shell lint
  assert_status 0
}

@test "test fixture is skipped" {
  # shellcheck disable=SC2016 # the fixture holds a literal $1 that must not be linted
  plant 644 test/fixture.txt 'echo $1
'
  run_shell lint
  assert_status 0
}

# Tools

@test "missing tool" {
  local git
  git=$(command -v git)
  PATH=$(dirname -- "${git}"):/usr/bin:/bin
  if command -v shellcheck > /dev/null; then skip "shellcheck is installed system-wide"; fi
  run_shell lint
  assert_status 2
  assert_output_has "shellcheck not found"
}

@test "missing formatter" {
  local git
  git=$(command -v git)
  PATH=$(dirname -- "${git}"):/usr/bin:/bin
  if command -v shfmt > /dev/null; then skip "shfmt is installed system-wide"; fi
  run_shell format
  assert_status 2
  assert_output_has "shfmt not found"
}

@test "tool that fails while linting" {
  fake_tool shellcheck 'exit 3'
  run_shell lint
  assert_status 2
  assert_output_has "shellcheck failed (exit 3)"
}

@test "shfmt that fails while fixing" {
  fake_tool shfmt 'exit 2'
  run_shell fix
  assert_status 2
  assert_output_has "shfmt failed"
}
