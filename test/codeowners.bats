#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests of scripts/check-codeowners, the gate on the code owners errors
# that GitHub reports (ADR 13). As CONTRIBUTING.md asks of gate tests,
# each case guards a way the gate could pass when it should fail, where
# review would likely miss it, and says which. A fake curl stands in for
# GitHub; the spec in changes/done/2026-10-30-codeowners-gate/ records
# the checks by hand and the real API's answers.
#
# Requires: bats and jq (versions in mise.toml; run it with
# `mise run test`), git >= 2.28 (init -b).

load helpers

setup_file() {
  setup_suite_env
  export TEMPLATE="${BATS_FILE_TMPDIR}/template"
  REPO=${TEMPLATE}
  mkdir -p "${REPO}/scripts" "${REPO}/.github"
  cp -- "${SRC}/scripts/check-codeowners" "${REPO}/scripts/"
  cp -R -- "${SRC}/scripts/lib" "${REPO}/scripts/"
  printf '* @acme/maintainers\n' > "${REPO}/.github/CODEOWNERS"
  commit_template
}

setup() {
  use_template
  # As CI sets them; a case changes what it tests.
  export GITHUB_ACTIONS=true GITHUB_TOKEN=planted_token \
    GITHUB_REPOSITORY=acme/skills GITHUB_API_URL=https://api.example.com
  GITHUB_SHA=$(g rev-parse HEAD)
  export GITHUB_SHA
  # GitHub answers 200 with no errors unless a case says otherwise.
  export FAKE_STATUS=200 FAKE_BODY='{"errors":[]}'
  # The fake keeps its arguments, one to a line, in curl.args.
  export FAKE_LOG=${BATS_TEST_TMPDIR}
  # shellcheck disable=SC2016 # expanded by the fake, not here
  fake_tool curl '
out=""
printf "%s\n" "$@" > "${FAKE_LOG}/curl.args"
while [ "$#" -gt 0 ]; do
  if [ "$1" = --output ]; then out=$2; fi
  shift
done
cat > /dev/null
printf "%s" "${FAKE_BODY}" > "${out}"
printf "%s" "${FAKE_STATUS}"
exit 0'
}

# move_codeowners <from> <to>: commit the code owners file at a new
# path, and point GITHUB_SHA at that commit.
move_codeowners() {
  mkdir -p -- "$(dirname -- "${REPO}/$2")"
  g mv -- "$1" "$2"
  g commit -q -m "chore: move code owners"
  GITHUB_SHA=$(g rev-parse HEAD)
}

# Guards a CI run without the token, as after a change to the workflow,
# being skipped as if it were a local run.
@test "a missing or empty token in GitHub Actions is an error, not a skip" {
  GITHUB_TOKEN='' run "${REPO}/scripts/check-codeowners"
  assert_status 2
  assert_output_has "must be set in GitHub Actions"
  unset GITHUB_TOKEN
  run "${REPO}/scripts/check-codeowners"
  assert_status 2
  assert_output_has "must be set in GitHub Actions"
}

# Guards a request that GitHub reads for another commit, as when ref is
# dropped from the URL and GitHub checks the default branch.
@test "the request names the commit under test" {
  run "${REPO}/scripts/check-codeowners"
  assert_status 0
  run grep -cxF "https://api.example.com/repos/acme/skills/codeowners/errors?ref=${GITHUB_SHA}" \
    "${BATS_TEST_TMPDIR}/curl.args"
  assert_output_is 1
}

# Guards an API failure read as no errors. GitHub answers 404 both for a
# commit with no code owners file and for a repository the token cannot
# read, so a 404 must not pass when the commit has a file.
@test "a 404 for a commit with a code owners file is an error, not a pass" {
  FAKE_STATUS=404 FAKE_BODY='{"message":"Not Found"}' run "${REPO}/scripts/check-codeowners"
  assert_status 2
  assert_output_has "GitHub answered 404"
}

# Guards a 200 answer with no list of errors counted as an empty list:
# in jq, both `null | length` and `{} | length` are 0.
@test "a 200 answer without a list of errors is an error" {
  local body
  for body in '{"message":"Moved"}' '{"errors":{}}'; do
    FAKE_BODY=${body} run "${REPO}/scripts/check-codeowners"
    assert_status 2
    assert_output_has "no list of errors"
  done
}

# Guards a lost finding, as after a jq bump that changes how the errors
# are read. GitHub's first message is in its real form, a sentence and
# then the line it quotes.
@test "each reported error fails the gate and names its line" {
  FAKE_BODY='{"errors":[
    {"line":3,"column":5,"kind":"Unknown owner","path":".github/CODEOWNERS",
     "message":"Unknown owner on line 3: make sure @nobody exists and has write access\n\n  /docs/ @nobody\n         ^"},
    {"line":5,"column":1,"kind":"Invalid pattern","path":".github/CODEOWNERS",
     "message":"Invalid pattern on line 5"}]}' run "${REPO}/scripts/check-codeowners"
  assert_status 1
  assert_output_has "codeowners: .github/CODEOWNERS:3: Unknown owner on line 3: make sure @nobody exists"
  refute_output_has "/docs/ @nobody"
  assert_output_has "codeowners: .github/CODEOWNERS:5: Invalid pattern on line 5"
  assert_output_has "findings in .github/CODEOWNERS: 2"
}

# Guards a code owners file that GitHub reads but the gate does not look
# for, so that it passes without asking.
@test "a code owners file at the root or in docs/ is checked too" {
  local path from=.github/CODEOWNERS
  for path in CODEOWNERS docs/CODEOWNERS; do
    move_codeowners "${from}" "${path}"
    from=${path}
    FAKE_BODY='{"errors":[{"line":1,"path":"'"${path}"'","message":"Unknown owner on line 1"}]}' \
      run "${REPO}/scripts/check-codeowners"
    assert_status 1
    assert_output_has "findings in ${path}: 1"
  done
}

# Guards a commit missing from the clone, as when GITHUB_SHA names a
# commit the checkout did not fetch, read as a commit with no code owners
# file.
@test "a commit missing from the clone is an error, not a commit without the file" {
  GITHUB_SHA=0123456789abcdef0123456789abcdef01234567 run "${REPO}/scripts/check-codeowners"
  assert_status 2
  assert_output_has "is not in this clone"
}
