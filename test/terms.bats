#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests for the term check (ADR 4): each case plants values in its own copy
# of a throwaway repository and checks what scripts/check-terms finds.
# Values the check would flag in this file are built at run time.
#
# Requires: bats (version in mise.toml), git >= 2.28 (init -b), and what
# check-terms requires.

load helpers

readonly CORP=bigcorp

setup_file() {
  setup_suite_env
  export SECRET="zq-$$-planted"
  WIDE="zq$(printf '\305\202')$$"
  export WIDE

  export TEMPLATE="${BATS_FILE_TMPDIR}/template"
  REPO=${TEMPLATE}
  mkdir -p "${REPO}/scripts"
  cp -- "${SRC}/scripts/check-terms" "${REPO}/scripts/"
  cp -R -- "${SRC}/checks" "${REPO}/"
  printf 'Quuxly Cloud\nZorp++\n' >> "${REPO}/checks/terms/names.txt"
  commit_template
}

setup() {
  use_template
  printf '# test list\n%s\n' "${SECRET}" > "${BATS_TEST_TMPDIR}/private.txt"
  export CECH_PRIVATE_TERMS="${BATS_TEST_TMPDIR}/private.txt"
  unset CECH_REQUIRE_PRIVATE_TERMS GITHUB_HEAD_REF
}

# run_terms [terms args...]: run the term check; the output must never
# show the private term.
run_terms() {
  run "${REPO}/scripts/check-terms" "$@"
  refute_output_has "${SECRET}"
}

# plant <text>: stage a file holding text.
plant() {
  printf '%s\n' "$1" > "${REPO}/planted.md"
  g add planted.md
}

# Arguments

@test "clean repository" {
  run_terms
  assert_status 0
}

@test "help" {
  # A hit is staged, so help passes only if it exits before scanning.
  plant "Write to jane@${CORP}.com."
  run_terms --help
  assert_status 0
  assert_output_has "Usage: mise run terms"
}

@test "unknown option" {
  run_terms --stagd
  assert_status 2
  assert_output_has "unknown option: --stagd"
}

@test "option without its value" {
  run_terms --range
  assert_status 2
  assert_output_has "--range needs a value"
}

@test "extra argument" {
  run_terms --staged --all
  assert_status 2
  assert_output_has "unexpected argument: --all"
}

@test "missing gitleaks" {
  local git
  git=$(command -v git)
  PATH=$(dirname -- "${git}"):/usr/bin:/bin
  if command -v gitleaks > /dev/null; then skip "gitleaks is installed system-wide"; fi
  run_terms --staged
  assert_status 2
  assert_output_has "gitleaks not found"
}

# Tier 1

@test "email" {
  plant "Write to jane@${CORP}.com."
  run_terms --staged
  assert_status 1
  assert_output_has "planted.md:1: email:"
}

@test "example emails" {
  plant "Mail jane@sub.example.com or a@b.test."
  run_terms --staged
  assert_status 0
}

@test "public forge remote" {
  plant "git clone git@github.com:acme/repo.git"
  run_terms --staged
  assert_status 0
}

@test "private host remote" {
  plant "git clone git@gitlab.${CORP}.com:team/repo.git"
  run_terms --staged
  assert_status 1
  assert_output_has "planted.md:1: email:"
}

@test "aws arn" {
  local acct=987654321098
  plant "arn:aws:iam::${acct}:role/deploy"
  run_terms --staged
  assert_status 1
  assert_output_has "planted.md:1: aws-arn:"
}

@test "aws arn from the docs" {
  plant "arn:aws:iam::123456789012:role/deploy"
  run_terms --staged
  assert_status 0
}

@test "azure subscription" {
  local guid=3f2b1c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d
  plant "/subscriptions/${guid}/resourceGroups/rg"
  run_terms --staged
  assert_status 1
  assert_output_has "planted.md:1: azure-subscription:"
}

@test "azure zero subscription" {
  plant "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg"
  run_terms --staged
  assert_status 0
}

@test "gcp project" {
  plant "https://console.cloud.google.com/home?project=${CORP}-prod-42"
  run_terms --staged
  assert_status 1
  assert_output_has "planted.md:1: gcp-project:"
}

@test "gcp example project" {
  plant "https://console.cloud.google.com/home?project=my-project"
  run_terms --staged
  assert_status 0
}

@test "signed url" {
  plant "https://bucket.example/f?X-Amz-""Signature=0123456789abcdef0123"
  run_terms --staged
  assert_status 1
  assert_output_has "planted.md:1: signed-url:"
}

@test "sso user" {
  plant "jane_${CORP}.com#EXT""#@tenant"
  run_terms --staged
  assert_status 1
  assert_output_has "planted.md:1: sso-user:"
}

# Tier 2

@test "name with spaces, hyphens, underscores or none" {
  local v
  for v in "Quuxly Cloud" "Quuxly-Cloud" "quuxly_cloud" "QuuxlyCloud"; do
    plant "Runs on ${v} today."
    run_terms --staged
    # Name the spelling that broke.
    assert_status 1 || {
      echo "spelling: ${v}"
      false
    }
    assert_output_has "planted.md:1: name-" || {
      echo "spelling: ${v}"
      false
    }
  done
}

@test "name needs a word boundary" {
  plant "Runs on Quuxly Cloudy today."
  run_terms --staged
  assert_status 0
}

@test "name ending in punctuation" {
  plant "Built with Zorp++ here."
  run_terms --staged
  assert_status 1
}

@test "inline marker with a reason" {
  plant "Runs on Quuxly Cloud. cech-terms: allow comparison table"
  run_terms --staged
  assert_status 0
}

@test "inline marker without a reason" {
  plant "Runs on Quuxly Cloud. cech-terms: allow"
  run_terms --staged
  assert_status 1
}

@test "gitleaks marker on a public tier" {
  plant "Runs on Quuxly Cloud. gitleaks"":allow"
  run_terms --staged
  assert_status 1
}

# Tier 3

@test "private term" {
  plant "see ${SECRET} here"
  run_terms --staged
  assert_status 1
  assert_output_has "planted.md:1: private-term-2"
}

@test "private term in capitals" {
  local upper
  upper=$(printf '%s' "${SECRET}" | tr '[:lower:]' '[:upper:]')
  plant "see ${upper} here"
  run_terms --staged
  assert_status 1
  assert_output_has "planted.md:1: private-term-2"
}

@test "private term ignores the inline marker" {
  plant "see ${SECRET} here cech-terms: allow no"
  run_terms --staged
  assert_status 1
  assert_output_has "private-term-2"
}

@test "private term ignores the gitleaks marker" {
  plant "see ${SECRET} here gitleaks"":allow"
  run_terms --staged
  assert_status 1
  assert_output_has "private-term-2"
}

@test "private term inside a public hit" {
  plant "Write to jane@${SECRET}.com."
  run_terms --staged
  assert_status 1
  assert_output_has "email (location hidden)"
}

@test "private list with CRLF and spaces" {
  printf '# crlf list\r\n%s \r\n' "${SECRET}" > "${BATS_TEST_TMPDIR}/crlf.txt"
  plant "see ${SECRET} here"
  CECH_PRIVATE_TERMS="${BATS_TEST_TMPDIR}/crlf.txt" run_terms --staged
  assert_status 1
  assert_output_has "planted.md:1: private-term-2"
}

@test "a broken private list stops the check" {
  printf '%s\na++\n' "${SECRET}" > "${BATS_TEST_TMPDIR}/broken.txt"
  plant "Write to jane@${SECRET}.com."
  CECH_PRIVATE_TERMS="${BATS_TEST_TMPDIR}/broken.txt" run_terms --staged
  assert_status 2
  assert_output_has "private tier could not run"
}

# Paths

@test "private term in a path" {
  printf 'clean\n' > "${REPO}/notes-${SECRET}.md"
  g add .
  run_terms --staged
  assert_status 1
  assert_output_has "paths:"
}

@test "private term in a path and its content" {
  printf 'see %s\n' "${SECRET}" > "${REPO}/notes-${SECRET}.md"
  g add .
  run_terms --staged
  assert_status 1
  assert_output_has "(location hidden: a path holds a private term)"
}

@test "private term in a path and its content, in history" {
  commit_and_drop "notes-${SECRET}.md" "see ${SECRET}"
  run_terms
  assert_status 1
}

@test "private term in a removed path" {
  commit_and_drop "notes-${SECRET}.md" clean
  run_terms
  assert_status 1
  assert_output_has "paths:"
}

@test "non-ASCII private term in a path" {
  printf '%s\n' "${WIDE}" > "${BATS_TEST_TMPDIR}/wide.txt"
  printf 'Write to jane@%s.com.\n' "${CORP}" > "${REPO}/notes-${WIDE}.md"
  g add .
  CECH_PRIVATE_TERMS="${BATS_TEST_TMPDIR}/wide.txt" run_terms --staged
  assert_status 1
  assert_output_has "paths:"
  refute_output_has "${WIDE}"
}

@test "non-ASCII private term in a removed path" {
  printf '%s\n' "${WIDE}" > "${BATS_TEST_TMPDIR}/wide.txt"
  commit_and_drop "notes-${WIDE}.md" clean
  CECH_PRIVATE_TERMS="${BATS_TEST_TMPDIR}/wide.txt" run_terms
  assert_status 1
  assert_output_has "paths:"
}

# Refusals

@test "a broken public rule makes the check error" {
  printf '\n[[rules]]\nid = "broken"\nregex = %sa++%s\n' "'''" "'''" \
    >> "${REPO}/checks/terms/rules.toml"
  run_terms --staged
  assert_status 2
  assert_output_has "gitleaks failed (public"
}

@test "a .gitleaksignore file is refused" {
  printf 'planted.md:private-term-2:1\n' > "${REPO}/.gitleaksignore"
  g add .gitleaksignore
  run_terms --staged
  assert_status 2
  assert_output_has "remove .gitleaksignore"
}

@test "an untracked .gitleaksignore at the root is refused" {
  printf 'planted.md:private-term-2:1\n' > "${REPO}/.gitleaksignore"
  run_terms --staged
  assert_status 2
  assert_output_has "remove .gitleaksignore"
}

@test "missing list when required" {
  CECH_PRIVATE_TERMS="${BATS_TEST_TMPDIR}/nonexistent" CECH_REQUIRE_PRIVATE_TERMS=1 \
    run_terms
  assert_status 2
  assert_output_has "private term list not found"
}

# Commit messages

@test "tracker key in a message" {
  printf 'fix: thing\n\nFixes %s-123.\n' "ABC" > "${BATS_TEST_TMPDIR}/msg"
  run_terms --message "${BATS_TEST_TMPDIR}/msg"
  assert_status 1
  assert_output_has "commit-message:3: tracker-key"
}

@test "standards are not tracker keys" {
  printf 'fix: thing\n\nUTF-8 and SHA-256 only.\n# ABC-123 in a comment\n' \
    > "${BATS_TEST_TMPDIR}/msg"
  run_terms --message "${BATS_TEST_TMPDIR}/msg"
  assert_status 0
}

@test "private term in a message" {
  printf 'fix: thing\n\nAbout %s.\n' "${SECRET}" > "${BATS_TEST_TMPDIR}/msg"
  run_terms --message "${BATS_TEST_TMPDIR}/msg"
  assert_status 1
  assert_output_has "commit-message:3: private-term-2"
}

@test "diff below the scissors line is ignored" {
  printf 'fix: thing\n# %s >8 %s\n+Fixes %s-123\n' \
    "------------------------" "------------------------" "ABC" > "${BATS_TEST_TMPDIR}/msg"
  run_terms --message "${BATS_TEST_TMPDIR}/msg"
  assert_status 0
}

@test "missing message file" {
  run_terms --message "${BATS_TEST_TMPDIR}/nonexistent"
  assert_status 2
  assert_output_has "cannot read message file"
}

# History

@test "private term added then removed" {
  commit_and_drop gone.md "note ${SECRET}"
  run_terms
  assert_status 1
  assert_output_has "private-term-2 in commit"
  run_terms --staged
  assert_status 0
}

@test "range with and without the adding commit" {
  commit_and_drop gone.md "note ${SECRET}"
  run_terms --range HEAD~1..HEAD
  assert_status 0
  run_terms --range HEAD~2..HEAD
  assert_status 1
}

@test "bad range" {
  run_terms --range nope..HEAD
  assert_status 2
  assert_output_has "bad revision range"
}

@test "default range from origin/main" {
  commit_and_drop gone.md "note ${SECRET}"
  g update-ref refs/remotes/origin/main HEAD~2
  run_terms
  assert_status 1
  assert_output_has "private-term-2 in commit"
  g update-ref refs/remotes/origin/main HEAD
  run_terms
  assert_status 0
}

@test "full scan reaches other branches" {
  g switch -q -c side
  printf 'note %s\n' "${SECRET}" > "${REPO}/side.md"
  g add side.md
  g commit -q -m "docs: add note"
  g switch -q main
  run_terms
  assert_status 0
  run_terms --all
  assert_status 1
  assert_output_has "private-term-2 in commit"
}

@test "public hit added then removed" {
  commit_and_drop gone.md "mail jane@${CORP}.com"
  run_terms
  assert_status 1
  assert_output_has "email in commit"
}

# Ref names and tags

@test "private term in the CI branch name" {
  GITHUB_HEAD_REF="topic-${SECRET}" run_terms
  assert_status 1
  assert_output_has "ref-names:"
}

@test "private term in a branch name" {
  g branch "topic-${SECRET}"
  run_terms
  assert_status 1
  assert_output_has "ref-names:"
}

@test "private term in a tag name" {
  g tag "v-${SECRET}"
  run_terms
  assert_status 1
  assert_output_has "ref-names:"
}

@test "private term in a tag message" {
  g tag -a v0.0.1 -m "Release about ${SECRET}"
  run_terms
  assert_status 1
  assert_output_has "tag-messages:"
}
