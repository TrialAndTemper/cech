#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests of the harness probes (ADR 9 item 4): scripts/run-probes,
# scripts/report-probes and the scripts in scripts/probes/. They never
# block a merge, so, as ADR 4 asks of gate tests, each case guards a
# way the scheduled job could pass without a probe run, or a differing
# result unreported, where review would likely miss it, and says
# which. Each probe's own steps need the real harnesses, so they were
# checked by hand, on a harness made to give another result, when the
# probes were added.
#
# Requires: bats (version in mise.toml), git >= 2.28 (init -b), awk and
# diff.

load helpers

setup_file() {
  setup_suite_env
  export TEMPLATE="${BATS_FILE_TMPDIR}/template"
  REPO=${TEMPLATE}
  mkdir -p "${REPO}/scripts/probes"
  cp -- "${SRC}/scripts/run-probes" "${SRC}/scripts/report-probes" "${REPO}/scripts/"
  cp -R -- "${SRC}/scripts/lib" "${REPO}/scripts/"
  commit_template
}

setup() {
  use_template
  fake_tool claude 'echo "2.1.312 (Claude Code)"'
  fake_tool codex 'echo "codex-cli 0.166.0"'
}

# add_probe <entry> <exit status>: add a probe for entry that exits so.
add_probe() {
  printf '#!/bin/sh\nexit %s\n' "$2" > "${REPO}/scripts/probes/$1"
  chmod 755 "${REPO}/scripts/probes/$1"
}

# Guards an entry marked unattended whose script is missing or misnamed,
# which the job would skip without a word, and a script left after its
# entry is unmarked or renamed.
@test "every unattended entry has a probe, and every probe an entry" {
  local dir=${BATS_TEST_TMPDIR}
  awk '
    /^\[[^.[]+\]$/ { entry = substr($0, 2, length($0) - 2) }
    /^unattended = true$/ { print entry }' \
    "${SRC}/cli/internal/harnessdata/harnesses.toml" > "${dir}/marked"
  sort -o "${dir}/marked" "${dir}/marked"
  git -C "${SRC}" ls-files -- scripts/probes > "${dir}/files"
  sed 's|^scripts/probes/||' "${dir}/files" > "${dir}/scripts"
  sort -o "${dir}/scripts" "${dir}/scripts"
  run cat "${dir}/marked"
  # An empty list would make the comparison pass with nothing marked.
  assert_output_has "codex-plugin-metadata"
  run diff "${dir}/marked" "${dir}/scripts"
  assert_status 0
}

# Guards a runner that lets a probe's exit status go, so the job passes
# and opens no issue, or loses a probe's line in the results file, from
# which the job opens the issues.
@test "a differing result or a probe that cannot run fails the runner and is in the results" {
  add_probe codex-a 0
  add_probe codex-b 1
  add_probe claude-code-c 2
  run "${REPO}/scripts/run-probes" --results "${BATS_TEST_TMPDIR}/results"
  assert_status 2
  run cat "${BATS_TEST_TMPDIR}/results"
  assert_output_is "claude-code-c failed claude-code 2.1.312
codex-a ok codex 0.166.0
codex-b differs codex 0.166.0"

  run "${REPO}/scripts/run-probes" codex-a codex-b
  assert_status 1
  run "${REPO}/scripts/run-probes" codex-a
  assert_status 0
}

# Guards a report job that opens no issue for an entry that differs, or
# for a probe that could not run, so a failed run goes unseen; and one
# that reads its results through a gh that eats stdin, and so reports
# only the first line.
@test "the report opens or comments on an issue for each differing entry, and one for the job" {
  # The fake gh logs each call on one line, and finds an open issue,
  # number 7, for codex-b only.
  # shellcheck disable=SC2016 # the fake gh expands the variables
  fake_tool gh '
cat > /dev/null
printf "%s\n" "$*" | tr "\n" " " >> "${BATS_TEST_TMPDIR}/gh.log"
echo >> "${BATS_TEST_TMPDIR}/gh.log"
case "$1 $2 ${TITLE:-}" in
  "issue list harness probe: codex-b gives another result") echo 7 ;;
  *) ;;
esac'
  add_probe codex-a 1
  add_probe codex-b 1
  add_probe codex-c 0
  add_probe claude-code-d 2
  printf '%s\n' "codex-a differs codex 0.166.0" "codex-b differs codex 0.166.0" \
    "codex-c ok codex 0.166.0" "claude-code-d failed claude-code 2.1.312" > "${BATS_TEST_TMPDIR}/results"
  run "${REPO}/scripts/report-probes" "${BATS_TEST_TMPDIR}/results" https://example.com/run
  assert_status 0
  run grep -c -e '^issue create --title harness probe: codex-a gives another result --body .*codex 0.166.0: https://example.com/run.*--label triage --label bug --label portability ' \
    -e '^issue comment 7 --body The probe of codex-b' \
    -e '^issue create --title harness probes: the scheduled job fails on main --body .*could not run: claude-code-d\. .*--label triage --label bug --label ci ' \
    "${BATS_TEST_TMPDIR}/gh.log"
  assert_output_is 3
}

# Guards a run that fails before it writes a result, such as a harness
# that will not install, which would otherwise open no issue.
@test "the report opens the job's issue for a run with no result" {
  # shellcheck disable=SC2016 # the fake gh expands the variables
  fake_tool gh 'printf "%s\n" "$*" >> "${BATS_TEST_TMPDIR}/gh.log"'
  : > "${BATS_TEST_TMPDIR}/results"
  run "${REPO}/scripts/report-probes" "${BATS_TEST_TMPDIR}/results" https://example.com/run
  assert_status 0
  run grep -c '^issue create --title harness probes: the scheduled job fails on main' "${BATS_TEST_TMPDIR}/gh.log"
  assert_output_is 1
}

# Guards a run stopped by the step's timeout after a probe that
# differs: the probes after it have no line, and without the job's
# issue a hung harness would go unseen.
@test "the report opens the job's issue for a probe with no result" {
  add_probe codex-a 1
  add_probe codex-b 0
  # shellcheck disable=SC2016 # the fake gh expands the variables
  fake_tool gh 'printf "%s\n" "$*" | tr "\n" " " >> "${BATS_TEST_TMPDIR}/gh.log"; echo >> "${BATS_TEST_TMPDIR}/gh.log"'
  printf '%s\n' "codex-a differs codex 0.166.0" > "${BATS_TEST_TMPDIR}/results"
  run "${REPO}/scripts/report-probes" "${BATS_TEST_TMPDIR}/results" https://example.com/run
  assert_status 0
  run grep -c '^issue create --title harness probes: the scheduled job fails on main --body .*codex-b (no result)' \
    "${BATS_TEST_TMPDIR}/gh.log"
  assert_output_is 1
}
