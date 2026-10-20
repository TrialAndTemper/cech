#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests for scripts/lib/common.sh: each case sources it into a POSIX sh
# script and checks what the helpers print, how the script ends, and
# that the temporary directory is removed.
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

# assert_gone <path>: fail the case if the path still exists.
assert_gone() {
  if [[ -e $1 ]]; then
    printf 'left behind: %s\n' "$1"
    return 1
  fi
}

@test "die names the script and exits 2" {
  lib_run 'die "no luck"; echo after'
  assert_status 2
  assert_output_is "demo: no luck"
}

@test "make_tmp creates a directory and removes it on exit" {
  # shellcheck disable=SC2016 # the inner shell expands ${TMP}
  lib_run 'make_tmp; [ -d "${TMP}" ] && printf "%s\n" "${TMP}"'
  assert_status 0
  assert_gone "${output}"
}

@test "make_tmp removes the directory on TERM and ends by the signal" {
  # shellcheck disable=SC2016 # the inner shell expands ${TMP} and $$
  lib_run 'make_tmp; printf "%s\n" "${TMP}"; kill -s TERM $$; echo after'
  assert_status 143
  refute_output_has "after"
  assert_gone "${output}"
}

@test "die after make_tmp removes the directory" {
  # shellcheck disable=SC2016 # the inner shell expands ${TMP}
  lib_run 'make_tmp; printf "%s\n" "${TMP}"; die x'
  assert_status 2
  assert_output_has "demo: x"
  assert_gone "${output%%$'\n'*}"
}

@test "make_tmp stops with exit 2 if mktemp fails" {
  mkdir -p -- "${BATS_TEST_TMPDIR}/bin"
  printf '#!/bin/sh\nexit 1\n' > "${BATS_TEST_TMPDIR}/bin/mktemp"
  chmod 755 "${BATS_TEST_TMPDIR}/bin/mktemp"
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"
  lib_run 'make_tmp; echo after'
  assert_status 2
  assert_output_is "demo: cannot create a temporary directory"
}
