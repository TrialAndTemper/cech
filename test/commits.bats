#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests of scripts/check-commits, the commit message gate (ADR 6). As
# ADR 4 asks of gate tests, each case guards a way the gate could pass
# when it should fail, where review would likely miss it, and says
# which. The other ways were checked by hand when their cases were
# dropped.
#
# Requires: bats (version in mise.toml), git >= 2.28 (init -b), and what
# check-commits requires.

load helpers

readonly SIGNOFF='Signed-off-by: Test <test@example.com>'

setup_file() {
  setup_suite_env
  export TEMPLATE="${BATS_FILE_TMPDIR}/template"
  REPO=${TEMPLATE}
  mkdir -p "${REPO}/scripts"
  cp -- "${SRC}/scripts/check-commits" "${REPO}/scripts/"
  cp -R -- "${SRC}/scripts/lib" "${REPO}/scripts/"
  commit_template
}

setup() {
  use_template
  MESSAGE="${BATS_TEST_TMPDIR}/message"
}

# run_commits [commits args...]: run the gate.
run_commits() {
  run "${REPO}/scripts/check-commits" "$@"
}

# check_lines <line>...: write the lines as a message file, then check
# it with --message.
check_lines() {
  printf '%s\n' "$@" > "${MESSAGE}"
  run_commits --message "${MESSAGE}"
}

# commit <line>...: commit an empty change with the lines as message.
commit() {
  printf '%s\n' "$@" > "${MESSAGE}"
  g commit -q --allow-empty --cleanup=verbatim -F "${MESSAGE}"
}

# Guards a shell, such as macOS sh, or a grep, such as BSD grep, that
# splits the header or matches the scope differently and lets a bad
# scope through.
@test "bad scopes" {
  local header
  for header in "fix(): x" "fix(CLI): x" "fix(a b): x" "fix(a-): x" "fix(a: x" \
    "fix(1st): x"; do
    check_lines "${header}" "" "${SIGNOFF}"
    assert_status 1 || {
      printf 'header: %s\n' "${header}"
      return 1
    }
    assert_output_has "header scope is not lower-case"
  done
}

# Guards a shell, such as macOS sh, whose parameter expansion splits the
# header differently and finds a summary where there is none.
@test "missing summary" {
  local header
  for header in "fix" "fix:" "fix: " "fix:thing" "fix(cli)x: thing" "fix!!: thing"; do
    check_lines "${header}" "" "${SIGNOFF}"
    assert_status 1 || {
      printf 'header: %s\n' "${header}"
      return 1
    }
    assert_output_has "header lacks"
  done
}

# Guards a locale or a tr, such as on macOS, under which the header
# length counts bytes or characters differently and a long header
# passes.
@test "header of 73 characters, some of them wide, under the C locale" {
  local wide
  wide="chore: $(printf '\305\202%.0s' {1..65})x"
  LC_ALL=C check_lines "${wide}" "" "${SIGNOFF}"
  assert_status 1
  assert_output_has "header is 73 characters"
}

# Guards a git version, such as on a CI runner image, whose
# interpret-trailers takes a sign-off in the body for a trailer.
@test "sign-off in the body is not a trailer" {
  check_lines "fix: mend the thing" "" "${SIGNOFF}" "" "More body."
  assert_status 1
  assert_output_has "no Signed-off-by: trailer"
}

# Guards a sed or grep, such as on macOS, that finds the author's email
# in another person's sign-off in range mode, which CI uses.
@test "range needs the author's sign-off" {
  commit "feat: add a thing" "" "Signed-off-by: Other <other@example.com>"
  run_commits --range HEAD~1..HEAD
  assert_status 1
  assert_output_has "no Signed-off-by: trailer carries the author's email"
}

# Guards CECH_RANGE ignored: in CI the gate would then check its default
# range instead of the one the workflow sets, and could pass.
@test "CECH_RANGE replaces the default range" {
  commit "Old style" "" "${SIGNOFF}"
  g update-ref refs/remotes/origin/main HEAD
  commit "feat: add a thing" "" "${SIGNOFF}"
  # The default range holds only the good commit.
  CECH_RANGE=HEAD~2..HEAD run_commits
  assert_status 1
  assert_output_has "header type"
}
