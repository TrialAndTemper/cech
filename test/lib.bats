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
@test "a script that exits 1 after make_tmp ends with 1" {
  lib_run 'make_tmp; exit 1'
  assert_status 1
}

# Guards a shell, such as dash or macOS sh, in which a script stopped
# by TERM ends as if it had passed.
@test "a script stopped by TERM after make_tmp ends by the signal" {
  # shellcheck disable=SC2016 # the inner shell expands $$
  lib_run 'make_tmp; kill -s TERM $$; echo after'
  assert_status 143
  refute_output_has "after"
}
