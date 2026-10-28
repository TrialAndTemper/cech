#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests of scripts/lib/common.sh, which every gate script sources. As
# ADR 10 asks of gate tests, each case guards a way a gate could pass
# when it should fail, where review would likely miss it, and says
# which. The other ways were checked by hand when their cases were
# dropped.
#
# Requires: bats (version in mise.toml).

load helpers

# lib_run <script>: run the script in sh, after sourcing the library and
# setting PROG=demo.
lib_run() {
  # shellcheck disable=SC2016 # the inner shell expands $1
  run sh -c '. "$1"; PROG=demo; shift; eval "$1"' sh \
    "${SRC}/scripts/lib/common.sh" "$1"
}

# Guards a shell, such as dash or macOS sh, whose EXIT trap replaces a
# script's findings status with the cleanup's.
@test "a script that exits 1 after make_scratch ends with 1" {
  lib_run 'make_scratch; exit 1'
  assert_status 1
}

# Guards a shell, such as dash or macOS sh, in which a script stopped
# by TERM ends as if it had passed.
@test "a script stopped by TERM after make_scratch ends by the signal" {
  # shellcheck disable=SC2016 # the inner shell expands $$
  lib_run 'make_scratch; kill -s TERM $$; echo after'
  assert_status 143
  refute_output_has "after"
}

# Guards a library that sets TMP, TEMP or TMPDIR, which every tool a
# gate runs inherits. Under Git Bash an empty TMP left MinGW gcc
# without a usable temporary directory, so it could not build the race
# runtime, and the Go tests ran without -race with only a warning.
@test "the tools a script runs get the TMP, TEMP and TMPDIR it was given" {
  # TMPDIR must exist: make_scratch creates its directory there.
  # shellcheck disable=SC2016 # the inner shells expand the variables
  TMP=/kept-tmp TEMP=/kept-temp TMPDIR=${BATS_TEST_TMPDIR} \
    lib_run 'show() { sh -c '\''echo "${TMP} ${TEMP} ${TMPDIR}"'\''; }
      show; make_scratch; show'
  assert_status 0
  assert_output_is "/kept-tmp /kept-temp ${BATS_TEST_TMPDIR}
/kept-tmp /kept-temp ${BATS_TEST_TMPDIR}"
}
