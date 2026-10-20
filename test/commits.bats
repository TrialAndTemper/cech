#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests for the commit message gate (ADR 11): each case writes a message
# or commits in its own copy of a throwaway repository and checks what
# scripts/check-commits finds.
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

# Arguments

@test "help" {
  local opt
  for opt in -h --help; do
    run_commits "${opt}"
    assert_status 0
    assert_output_has "Usage: mise run commits"
  done
}

@test "unknown option" {
  run_commits --mesage x
  assert_status 2
  assert_output_has "unknown option: --mesage"
}

@test "option without its value" {
  run_commits --range
  assert_status 2
  assert_output_has "--range needs a value"
}

@test "extra argument" {
  run_commits --range HEAD extra
  assert_status 2
  assert_output_has "unexpected argument: extra"
}

@test "missing message file" {
  run_commits --message "${BATS_TEST_TMPDIR}/none"
  assert_status 2
  assert_output_has "cannot read message file"
}

@test "a value that looks like an option" {
  run_commits --range --all
  assert_status 2
  assert_output_has "--range value must not start with -: --all"
}

@test "bad range" {
  run_commits --range nope..HEAD
  assert_status 2
  assert_output_has "bad revision range: nope..HEAD"
}

# Header

@test "good headers" {
  local header
  for header in "feat: add a thing" "fix(cli): mend the parser" \
    "docs(adr): record decision 0014" "refactor(issue-tracker)!: rename a role" \
    "feat!: drop a flag" "revert: feat(cli): add a flag" \
    "chore: $(printf 'x%.0s' {1..65})"; do
    check_lines "${header}" "" "${SIGNOFF}"
    assert_status 0 || {
      printf 'header: %s\n' "${header}"
      return 1
    }
  done
}

@test "unknown type" {
  check_lines "feature: add a thing" "" "${SIGNOFF}"
  assert_status 1
  assert_output_has "message: header type is not one of"
}

@test "git's revert header" {
  check_lines 'Revert "feat: add a thing"' "" "${SIGNOFF}"
  assert_status 1
  assert_output_has "use revert: and the reverted header"
}

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

@test "summary with extra spaces" {
  check_lines "fix:  thing" "" "${SIGNOFF}"
  assert_status 1
  assert_output_has "starts or ends with a space"
  # git strips a trailing space from a message, so only a range sees one.
  commit "fix: thing " "" "${SIGNOFF}"
  run_commits --range HEAD~1..HEAD
  assert_status 1
  assert_output_has "starts or ends with a space"
}

@test "final full stop" {
  check_lines "fix: mend the thing." "" "${SIGNOFF}"
  assert_status 1
  assert_output_has "header ends with a full stop"
}

@test "header over 72 characters" {
  check_lines "chore: $(printf 'x%.0s' {1..67})" "" "${SIGNOFF}"
  assert_status 1
  assert_output_has "header is 74 characters, more than 72"
}

@test "header counts characters, not bytes, under the C locale" {
  local wide
  wide="chore: $(printf '\305\202%.0s' {1..65})"
  LC_ALL=C check_lines "${wide}" "" "${SIGNOFF}"
  assert_status 0
  LC_ALL=C check_lines "${wide}x" "" "${SIGNOFF}"
  assert_status 1
  assert_output_has "header is 73 characters"
}

@test "line 2 not blank" {
  check_lines "fix: mend the thing" "more text" "" "${SIGNOFF}"
  assert_status 1
  assert_output_has "line 2 is not blank"
}

@test "findings never quote the message" {
  check_lines "Zq-planted mend." "zq-planted body" "" "${SIGNOFF}"
  assert_status 1
  refute_output_has "planted"
}

# Sign-off

@test "missing sign-off" {
  check_lines "fix: mend the thing" "" "Body."
  assert_status 1
  assert_output_has "no Signed-off-by: trailer"
}

@test "sign-off in the body is not a trailer" {
  check_lines "fix: mend the thing" "" "${SIGNOFF}" "" "More body."
  assert_status 1
  assert_output_has "no Signed-off-by: trailer"
}

@test "any sign-off passes in message mode with no author configured" {
  check_lines "fix: mend the thing" "" "Signed-off-by: Other <other@example.com>"
  assert_status 0
}

@test "message mode checks the author git passes to the hook" {
  GIT_AUTHOR_NAME=Other GIT_AUTHOR_EMAIL=other@example.com check_lines "fix: mend the thing" "" "${SIGNOFF}"
  assert_status 1
  assert_output_has "no Signed-off-by: trailer carries the author's email"
  GIT_AUTHOR_NAME=Test GIT_AUTHOR_EMAIL=test@example.com check_lines "fix: mend the thing" "" "${SIGNOFF}"
  assert_status 0
}

@test "message mode checks the configured author" {
  g config user.name Other
  g config user.email other@example.com
  check_lines "fix: mend the thing" "" "${SIGNOFF}"
  assert_status 1
  assert_output_has "no Signed-off-by: trailer carries the author's email"
}

@test "sign-off key in another case" {
  check_lines "fix: mend the thing" "" "signed-off-by: Test <test@example.com>"
  assert_status 0
}

@test "sign-off among other trailers" {
  check_lines "fix: mend the thing" "" "Body." "" \
    "Co-authored-by: Other <other@example.com>" "${SIGNOFF}"
  assert_status 0
}

# Message mode

@test "comments and the scissors line are dropped" {
  # Text after the scissors line forms its own paragraph, so if it were
  # kept, the sign-off would no longer be a trailer.
  check_lines "" "# a comment" "fix: mend the thing" "" "${SIGNOFF}" \
    "# ------------------------ >8 ------------------------" "" "diff text"
  assert_status 0
}

@test "scissors line with another comment string and CRLF" {
  g config core.commentString ";"
  printf '%s\r\n' "fix: mend the thing" "" "${SIGNOFF}" "; a comment" \
    "; ------------------------ >8 ------------------------" "" "diff text" \
    > "${MESSAGE}"
  run_commits --message "${MESSAGE}"
  assert_status 0
}

@test "fixup passes in message mode" {
  check_lines "fixup! fix: mend the thing"
  assert_status 0
  check_lines "squash! fix: mend the thing"
  assert_status 0
  check_lines "amend! fix: mend the thing"
  assert_status 0
}

@test "empty message" {
  check_lines "# only a comment"
  assert_status 1
  assert_output_has "message is empty"
}

# Range mode

@test "range of good commits" {
  commit "feat: add a thing" "" "${SIGNOFF}"
  commit "fix: mend the thing" "" "${SIGNOFF}"
  run_commits --range HEAD~2..HEAD
  assert_status 0
  assert_output_is "commits: ok"
}

@test "range names the commit" {
  commit "feat: add a thing" "" "${SIGNOFF}"
  commit "Mend the thing" "" "${SIGNOFF}"
  local good bad
  good=$(g rev-parse --short=12 HEAD~1)
  bad=$(g rev-parse --short=12 HEAD)
  run_commits --range HEAD~2..HEAD
  assert_status 1
  assert_output_has "commit ${bad}: header type"
  refute_output_has "${good}"
}

@test "range needs the author's sign-off" {
  commit "feat: add a thing" "" "Signed-off-by: Other <other@example.com>"
  run_commits --range HEAD~1..HEAD
  assert_status 1
  assert_output_has "no Signed-off-by: trailer carries the author's email"
}

@test "author's sign-off among others" {
  commit "feat: add a thing" "" "Signed-off-by: Other <other@example.com>" \
    "Signed-off-by: Test <TEST@example.com>"
  run_commits --range HEAD~1..HEAD
  assert_status 0
}

@test "fixups fail in a range" {
  local kind
  for kind in fixup squash amend; do
    commit "${kind}! feat: add a thing"
    run_commits --range HEAD~1..HEAD
    assert_status 1 || {
      printf 'kind: %s\n' "${kind}"
      return 1
    }
    assert_output_has "git rebase --autosquash"
  done
}

@test "empty message in a range" {
  g commit -q --allow-empty --allow-empty-message -m ""
  run_commits --range HEAD~1..HEAD
  assert_status 1
  assert_output_has "message is empty"
}

@test "blank first line in a range" {
  commit "" "feat: add a thing" "" "${SIGNOFF}"
  run_commits --range HEAD~1..HEAD
  assert_status 1
  assert_output_has "line 1 is blank"
}

@test "merge commit fails in a range" {
  g switch -q -c side
  commit "feat: add a thing" "" "${SIGNOFF}"
  g switch -q main
  commit "fix: mend the thing" "" "${SIGNOFF}"
  # A merge with a good message still fails.
  g merge -q --no-ff -m "feat: merge it" -m "${SIGNOFF}" side
  local merge
  merge=$(g rev-parse --short=12 HEAD)
  run_commits --range HEAD~1..HEAD
  assert_status 1
  assert_output_has "commit ${merge}: merge commit"
}

@test "empty range" {
  run_commits --range HEAD..HEAD
  assert_status 0
  assert_output_has "no commits to check"
}

@test "default range from origin/main" {
  commit "Old style" "" "${SIGNOFF}"
  local old
  old=$(g rev-parse --short=12 HEAD)
  g update-ref refs/remotes/origin/main HEAD
  commit "feat: add a thing" "" "${SIGNOFF}"
  run_commits
  assert_status 0
  commit "Mend the thing" "" "${SIGNOFF}"
  local bad
  bad=$(g rev-parse --short=12 HEAD)
  run_commits
  assert_status 1
  assert_output_has "commit ${bad}: header type"
  refute_output_has "${old}"
}

@test "default range without origin/main checks every commit" {
  # The template's only commit has no sign-off.
  run_commits
  assert_status 1
  assert_output_has "no Signed-off-by: trailer"
}

@test "default range in a repository with no commits" {
  REPO="${BATS_TEST_TMPDIR}/empty"
  mkdir -p "${REPO}/scripts"
  cp -- "${SRC}/scripts/check-commits" "${REPO}/scripts/"
  g init -q -b main
  run_commits
  assert_status 0
  assert_output_has "no commits to check"
}
