#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests of scripts/approval-diff, which writes the approval diff comment
# that shows what changed since each dismissed approval (ADR 6 item 5),
# and of the option checks of scripts/approval-comment, which posts it.
# Each approval-diff case builds a pull request in the case's
# repository, which serves as the remote: an approved head, then a
# current head made from it, and checks the diff comment the script
# writes. The names match the criteria in
# changes/done/2026-11-16-approval-diff/spec.md.
#
# Requires: bats (version in mise.toml), git >= 2.40, and what
# approval-diff requires.

load helpers

setup_file() {
  setup_suite_env
  export TEMPLATE="${BATS_FILE_TMPDIR}/template"
  REPO=${TEMPLATE}
  mkdir -p "${REPO}"
  printf 'one\ntwo\nthree\n' > "${REPO}/a.txt"
  commit_template
  # The approved head is unreachable once the branch is rewritten, as
  # on GitHub, which still serves it by SHA.
  g config uploadpack.allowAnySHA1InWant true
}

setup() {
  use_template
  OUT="${BATS_TEST_TMPDIR}/out"
  mkdir -p "${OUT}"
  : > "${BATS_TEST_TMPDIR}/reviews"
  : > "${BATS_TEST_TMPDIR}/dismissed"
}

# put <file> <text> <message>: write a file and commit it.
put() {
  printf '%s\n' "$2" > "${REPO}/$1"
  g add -- "$1"
  g commit -q -m "$3"
}

# review <login> <state> <rev>: add a review of a revision, newest
# last, with the next review id; a revision that does not resolve is
# taken as a SHA. The state DISMISSED_APPROVAL adds a review in the
# state DISMISSED and lists its id as a dismissed approval, as the
# workflow does.
review() {
  local sha id state
  sha=$(g rev-parse -q --verify "$3^{commit}") || sha=$3
  id=$(($(wc -l < "${BATS_TEST_TMPDIR}/reviews") + 1))
  state=$2
  if [[ ${state} == DISMISSED_APPROVAL ]]; then
    state=DISMISSED
    printf '%s\n' "${id}" >> "${BATS_TEST_TMPDIR}/dismissed"
  fi
  printf '%s\t%s\t%s\t%s\n' "$1" "${state}" "${sha}" "${id}" >> "${BATS_TEST_TMPDIR}/reviews"
}

# branch_with_fixup: on branch pr from main, commit c1, a fixup of it
# and c2; leave pr checked out.
branch_with_fixup() {
  g checkout -q -b pr main
  put a.txt "$(printf 'one\ntwo\nthree\nfour')" "feat: add four"
  put a.txt "$(printf 'one\ntwo\nthree\nFOUR')" "fixup! feat: add four"
  put b.txt "bee" "feat: add b"
}

# main_moves: add a commit to main that touches no file of the branch.
main_moves() {
  g checkout -q main
  put c.txt "sea" "feat: add c"
  g checkout -q pr
}

# squash_and_rebase: fold the fixups into pr and rebase it onto main.
squash_and_rebase() {
  GIT_SEQUENCE_EDITOR=true g rebase -q -i --autosquash main
}

# run_diff: run the script on pr against main, writing the comment to
# OUT.
run_diff() {
  local base head
  base=$(g rev-parse main)
  head=$(g rev-parse pr)
  run "${SRC}/scripts/approval-diff" --remote "${REPO}" \
    --base "${base}" --head "${head}" \
    --reviews "${BATS_TEST_TMPDIR}/reviews" \
    --dismissed-approvals "${BATS_TEST_TMPDIR}/dismissed" --comment "${OUT}"
}

# assert_comment_has <text>: fail the case unless the comment holds text.
assert_comment_has() {
  if ! grep -qF -- "$1" "${OUT}/body.md"; then
    printf 'comment lacks %s; comment:\n' "$1"
    cat "${OUT}/body.md"
    return 1
  fi
}

# refute_comment_has <text>: fail the case if the comment holds text.
refute_comment_has() {
  if grep -qF -- "$1" "${OUT}/body.md"; then
    printf 'comment holds %s; comment:\n' "$1"
    cat "${OUT}/body.md"
    return 1
  fi
}

# assert_comment_line <line>: fail the case unless the comment has the
# whole line.
assert_comment_line() {
  if ! grep -qxF -- "$1" "${OUT}/body.md"; then
    printf 'comment lacks the line %s; comment:\n' "$1"
    cat "${OUT}/body.md"
    return 1
  fi
}

# refute_file <path>: fail the case if the file exists.
refute_file() {
  if [[ -e $1 ]]; then
    printf 'unexpected file: %s\n' "$1"
    return 1
  fi
}

# assert_smaller <file> <bytes>: fail the case unless the file is
# smaller than the given number of bytes.
assert_smaller() {
  local size
  size=$(wc -c < "$1")
  if [[ ${size} -ge $2 ]]; then
    printf '%s is %s bytes, not under %s\n' "$1" "${size}" "$2"
    return 1
  fi
}

# Guards a diff that shows the squash or the new base as changes, or
# that GitHub does not colour: the range-diff's own - and + must start
# each changed line, with each commit's header as a hunk header.
@test "changed line" {
  local approved head base
  branch_with_fixup
  approved=$(g rev-parse pr)
  main_moves
  squash_and_rebase
  put b.txt "bees" "fixup! feat: add b"
  squash_and_rebase
  head=$(g rev-parse pr)
  base=$(g rev-parse main)
  review maint DISMISSED_APPROVAL "${approved}"
  run_diff
  assert_status 0
  assert_output_has "git version"
  assert_output_has "base ${base}, head ${head}, merge base ${base}"
  assert_output_has "dismissed approval of ${approved}, by maint"
  assert_comment_has "<!-- approval-diff head=${head} -->"
  assert_comment_has "@maint approved ${approved:0:12}; the head is now ${head:0:12}"
  assert_comment_has "replayed onto the current base"
  assert_comment_line "<details open>"
  assert_comment_line '```diff'
  assert_comment_line "-+bee"
  assert_comment_line "++bees"
  run grep -c '^@@ [0-9]*: .* @@$' "${OUT}/body.md"
  assert_output_is 2
}

# Guards a comment that shows a fixup pushed after the dismissal as a
# new commit with no patch, hiding its change.
@test "a fixup after the dismissal shows its change" {
  local approved
  g checkout -q -b pr main
  put b.txt "bee" "feat: add b"
  approved=$(g rev-parse pr)
  put b.txt "bees" "fixup! feat: add b"
  review maint DISMISSED_APPROVAL "${approved}"
  run_diff
  assert_status 0
  assert_comment_has "its fixups folded in"
  assert_comment_line "-+bee"
  assert_comment_line "++bees"
  refute_comment_has 'fixup!'
}

# Guards a comment that misses a reworded message.
@test "a reworded message shows in the comment" {
  local approved
  g checkout -q -b pr main
  put b.txt "bee" "feat: add b"
  approved=$(g rev-parse pr)
  g commit -q --amend -m "feat: add the letter b"
  review maint DISMISSED_APPROVAL "${approved}"
  run_diff
  assert_status 0
  assert_comment_line "-    feat: add b"
  assert_comment_line "+    feat: add the letter b"
}

# Guards a replay that resolves a conflict itself, for example with a
# strategy option, and so hides a resolution nobody reviewed, and a
# fallback that shows the approved fixups as commits of their own.
@test "resolved conflict" {
  g checkout -q -b pr main
  put a.txt "$(printf 'one\nTWO\nthree')" "feat: shout two"
  put b.txt "bee" "feat: add b"
  put b.txt "bees" "fixup! feat: add b"
  review maint DISMISSED_APPROVAL pr
  g checkout -q main
  put a.txt "$(printf 'one\n2\nthree')" "feat: number two"
  g checkout -q pr
  g rebase -q main 2> /dev/null || :
  printf 'one\nTWO 2\nthree\n' > "${REPO}/a.txt"
  g add a.txt
  GIT_EDITOR=true g rebase --continue > /dev/null
  run_diff
  assert_status 0
  assert_comment_has "as approved with their fixups folded in, since they conflict with the current base"
  refute_comment_has 'fixup!'
}

# Guards a fallback that claims to fold approved fixups that do not
# fold, here a fixup of the first commit that a later commit conflicts
# with.
@test "approved fixups that do not fold" {
  g checkout -q -b pr main
  put a.txt "$(printf 'one\ntwo\nthree\nfour')" "feat: add four"
  put a.txt "$(printf 'one\ntwo\nthree\nFOUR')" "feat: shout four"
  put a.txt "$(printf 'one\ntwo\nthree\nfour!')" "fixup! feat: add four"
  review maint DISMISSED_APPROVAL pr
  g reset -q --hard main
  put a.txt "$(printf 'one\ntwo\nthree\nfour!')" "feat: add four"
  run_diff
  assert_status 0
  assert_comment_has "as approved, since they do not replay cleanly"
  assert_comment_has "fixup! feat: add four"
}

# Guards a replay that misses the autosquash or the new base, after an
# approval dismissed by hand with nothing changed.
@test "a squash and rebase that changes nothing says so" {
  local approved
  branch_with_fixup
  approved=$(g rev-parse pr)
  main_moves
  squash_and_rebase
  review maint DISMISSED_APPROVAL "${approved}"
  run_diff
  assert_status 0
  assert_comment_has "nothing changed"
}

# Guards a section for an approval that a later approval or request
# for changes replaced, and one lost to a comment.
@test "approving again posts nothing" {
  g checkout -q -b pr main
  put b.txt "bee" "feat: add b"
  review maint DISMISSED_APPROVAL pr
  put b.txt "bees" "feat: change b"
  review maint COMMENTED pr
  run_diff
  assert_status 0
  assert_comment_has "@maint approved"
  rm -f -- "${OUT}/body.md"
  review maint APPROVED pr
  run_diff
  assert_status 0
  assert_output_has "no dismissed approval"
  refute_file "${OUT}/body.md"
  review maint DISMISSED_APPROVAL pr~1
  review maint CHANGES_REQUESTED pr
  run_diff
  assert_status 0
  refute_file "${OUT}/body.md"
  # A request for changes that is later dismissed still ends the older
  # approval's need for a section.
  review maint DISMISSED_APPROVAL pr~1
  review maint DISMISSED pr
  run_diff
  assert_status 0
  refute_file "${OUT}/body.md"
}

# Guards a comment for approvals GitHub kept, and sections out of order
# or with logins compared by case.
@test "no dismissed approval" {
  local approved
  branch_with_fixup
  approved=$(g rev-parse pr)
  squash_and_rebase
  review maint APPROVED pr
  run_diff
  assert_status 0
  refute_file "${OUT}/body.md"
  # Every dismissed approval gets a section, newest first.
  : > "${BATS_TEST_TMPDIR}/reviews"
  : > "${BATS_TEST_TMPDIR}/dismissed"
  review second DISMISSED_APPROVAL "${approved}"
  review maint DISMISSED_APPROVAL "${approved}"
  review MAINT COMMENTED pr
  review third DISMISSED_APPROVAL "${approved}"
  review Second APPROVED pr
  run_diff
  assert_status 0
  sed -n 's/^<summary>@\([a-z]*\) .*/\1/p' "${OUT}/body.md" > "${BATS_TEST_TMPDIR}/order"
  run cat "${BATS_TEST_TMPDIR}/order"
  assert_output_is "$(printf 'third\nmaint')"
  # Only the newest section is open.
  run grep -c '^<details open>$' "${OUT}/body.md"
  assert_output_is 1
  assert_comment_line "<details>"
}

# Guards a section that stops the comment, or names an older approval,
# when the approved head is gone, and a script that runs on a head it
# cannot fetch.
@test "approved head unavailable" {
  local base
  g checkout -q -b pr main
  put b.txt "bee" "feat: add b"
  review maint DISMISSED_APPROVAL 0123456789abcdef0123456789abcdef01234567
  # The API gives no commit for some reviews.
  review other DISMISSED_APPROVAL ''
  run_diff
  assert_status 0
  assert_comment_has "@maint approved a commit GitHub no longer serves"
  assert_comment_has "The approved head was 0123456789abcdef0123456789abcdef01234567"
  assert_comment_has "GitHub gave no commit for the approval"
  base=$(g rev-parse main)
  run "${SRC}/scripts/approval-diff" --remote "${REPO}" \
    --base "${base}" --head 0123456789abcdef0123456789abcdef01234567 \
    --reviews "${BATS_TEST_TMPDIR}/reviews" \
    --dismissed-approvals "${BATS_TEST_TMPDIR}/dismissed" --comment "${OUT}"
  assert_status 2
  assert_output_has "cannot fetch the head"
}

# Guards a script that runs a hook, filter or merge driver from the
# runner's configuration, or lets the commits' attributes choose a
# built-in merge driver that resolves a conflict nobody reviewed.
@test "hooks in the repository do not run" {
  local marker
  marker="${BATS_TEST_TMPDIR}/ran"
  mkdir -p "${BATS_TEST_TMPDIR}/hooks"
  printf '#!/bin/sh\ntouch "%s"\n' "${marker}" > "${BATS_TEST_TMPDIR}/hooks/post-checkout"
  chmod 755 "${BATS_TEST_TMPDIR}/hooks/post-checkout"
  cp -- "${BATS_TEST_TMPDIR}/hooks/post-checkout" "${BATS_TEST_TMPDIR}/hooks/post-rewrite"
  # A template directory whose hooks git init would copy without --template=.
  mkdir -p "${BATS_TEST_TMPDIR}/template"
  cp -R -- "${BATS_TEST_TMPDIR}/hooks" "${BATS_TEST_TMPDIR}/template/hooks"
  g checkout -q -b pr main
  put .gitattributes "* filter=x merge=x" "build: add attributes"
  put b.txt "bee" "feat: add b"
  review maint DISMISSED_APPROVAL pr
  main_moves
  g rebase -q main
  printf '[core]\n\thooksPath = %s\n[filter "x"]\n\tsmudge = touch %s; cat\n\tclean = cat\n[merge "x"]\n\tdriver = touch %s; false\n' \
    "${BATS_TEST_TMPDIR}/hooks" "${marker}" "${marker}" > "${BATS_TEST_TMPDIR}/gitconfig"
  GIT_CONFIG_GLOBAL="${BATS_TEST_TMPDIR}/gitconfig" GIT_TEMPLATE_DIR="${BATS_TEST_TMPDIR}/template" run_diff
  assert_status 0
  refute_file "${marker}"
  assert_comment_has "nothing changed"
  # union needs no configuration, so only ignoring the commits'
  # attributes keeps it from merging both sides of a conflict.
  : > "${BATS_TEST_TMPDIR}/reviews"
  : > "${BATS_TEST_TMPDIR}/dismissed"
  g checkout -q -b pr2 main
  put .gitattributes "* merge=union" "build: merge by union"
  put a.txt "$(printf 'one\nTWO\nthree')" "feat: shout two"
  review maint DISMISSED_APPROVAL pr2
  g checkout -q main
  put a.txt "$(printf 'one\n2\nthree')" "feat: number two"
  g checkout -q pr2
  g rebase -q main
  g branch -q -f pr pr2
  run_diff
  assert_status 0
  assert_comment_has "since they conflict with the current base"
}

# Guards a fence that a run of backticks in the diff closes early, and
# a comment longer than GitHub takes.
@test "backticks in the diff do not close the block, and a long comment is cut" {
  g checkout -q -b pr main
  put b.txt '``````' "feat: add b"
  review maint DISMISSED_APPROVAL pr
  g reset -q --hard main
  put b.txt '``````x' "feat: add b"
  run_diff
  assert_status 0
  assert_comment_line '```````diff'
  # A diff too long for one comment is cut, and the comment says so.
  awk 'BEGIN { for (i = 0; i < 4000; i++) printf "line %d of a long file\n", i }' > "${REPO}/b.txt"
  g add b.txt
  g commit -q --amend --no-edit
  run_diff
  assert_status 0
  assert_comment_has "The comment is cut here"
  # The cut closes the code block and the section before its note.
  assert_comment_line '```````'
  assert_comment_line '</details>'
  assert_smaller "${OUT}/body.md" 65536
}

# run_comment <args...>: run scripts/approval-comment with valid options,
# then the given ones, which override the earlier ones.
run_comment() {
  run "${SRC}/scripts/approval-comment" --repo acme/example --run 1 \
    --head 0123456789abcdef0123456789abcdef01234567 --head-repo acme/example \
    --image "docker.io/library/alpine@sha256:$(printf '%064d' 0)" \
    --server https://example.com "$@"
}

# Guards a posting script that takes a relay run id the API would read
# as a path, or an image without a full sha256 digest, before it calls
# gh or docker.
@test "approval-comment refuses a bad run id or an unpinned image" {
  run_comment --run '1/../2'
  assert_status 2
  assert_output_has "--run is not a run id"
  run_comment --image docker.io/library/alpine:3.24
  assert_status 2
  assert_output_has "--image is not pinned by digest"
  run_comment --image docker.io/library/alpine@sha256:
  assert_status 2
  assert_output_has "--image is not pinned by digest"
}

# Guards a posting script that runs with an option missing, or takes an
# option as another's value.
@test "approval-comment needs every option" {
  run "${SRC}/scripts/approval-comment" --repo acme/example
  assert_status 2
  assert_output_has "are required"
  run_comment --head --server
  assert_status 2
  assert_output_has "--head value must not start with -"
}
