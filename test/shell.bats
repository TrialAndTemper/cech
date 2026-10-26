#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests of scripts/check-shell, the script behind the shell lint and
# format gates. As ADR 10 asks of gate tests, each case guards a way the
# gates could pass when they should fail, where review would likely miss
# it, and says which. The other ways were checked by hand when their
# cases were dropped.
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
  cp -R -- "${SRC}/scripts/lib" "${REPO}/scripts/"
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

# Guards a shellcheck bump that stops reading .shellcheckrc, which alone
# enables SC2250, or stops linting a script as POSIX sh.
@test "lint finding" {
  # shellcheck disable=SC2016 # the planted script holds literal expansions
  plant 755 scripts/lint '#!/bin/sh
x=1
echo "$x"
if [[ -n "${x}" ]]; then :; fi
'
  run_shell lint
  assert_status 1
  assert_output_has "SC2250"
  assert_output_has "SC3010"
}

# Guards a shfmt bump that ignores .editorconfig, which alone asks for a
# space after >, or exits 0 on a diff.
@test "format finding" {
  plant 755 scripts/format '#!/bin/sh
echo x >/dev/null
'
  run_shell format
  assert_status 1
  assert_output_has "scripts/format"
}

# Guards a shellcheck bump that skips or misreads the bats dialect its
# directive names.
@test "bats suite is linted" {
  # shellcheck disable=SC2016 # the planted suite must hold a literal $1
  plant_suite 644 '#!/usr/bin/env bats' '# shellcheck shell=bats' '' \
    '@test "x" {' '  echo $1' '}'
  run_shell lint
  assert_status 1
  assert_output_has "SC2086"
}

# Guards BSD grep on macOS missing a bare [[ line, which bash 3.2 there
# would not fail.
@test "bare condition in a bats suite" {
  # shellcheck disable=SC2016 # the planted suite holds literal expansions
  plant_suite 644 '#!/usr/bin/env bats' '# shellcheck shell=bats' '' \
    '@test "x" {' '  [[ -n ${HOME} ]]' '  true' '}'
  run_shell lint
  assert_status 1
  assert_output_has "test/x.bats:5:"
  assert_output_has "does not fail a case on bash 3.2"
}
