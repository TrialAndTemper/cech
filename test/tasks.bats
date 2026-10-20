#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests for the mise tasks and the git hooks (ADR 10): each case runs
# mise or a hook in its own copy of a throwaway repository that holds the
# tracked files, and checks that the gates behind them still run and
# still fail. A few cases also pin what each task runs and depends on,
# which covers gates a case cannot run, such as the online link check.
#
# Requires: mise and bats (run it with `mise run test`), git >= 2.28
# (init -b), and what the gates require.

load helpers

setup_file() {
  setup_suite_env
  export SECRET="zq-$$-planted"

  # The suite runs inside `mise run test`; the cases start their own mise
  # runs, which must not take the outer task's settings.
  local v
  for v in $(compgen -v MISE_TASK_) MISE_CONFIG_ROOT MISE_ORIGINAL_CWD \
    MISE_PROJECT_ROOT __MISE_DIFF __MISE_SESSION __MISE_ORIG_PATH; do
    unset "${v}"
  done
  # Trust the copies' mise.toml without writing to the user's trust list.
  export MISE_TRUSTED_CONFIG_PATHS="${BATS_RUN_TMPDIR}"

  export TEMPLATE="${BATS_FILE_TMPDIR}/template"
  REPO=${TEMPLATE}
  mkdir -p "${REPO}"
  git -C "${SRC}" ls-files > "${BATS_FILE_TMPDIR}/tracked"
  local f
  while IFS= read -r f; do
    if [[ -f ${SRC}/${f} ]]; then
      mkdir -p -- "$(dirname -- "${REPO}/${f}")"
      cp -p -- "${SRC}/${f}" "${REPO}/${f}"
    fi
  done < "${BATS_FILE_TMPDIR}/tracked"
  commit_template
}

setup() {
  use_template
  printf '%s\n' "${SECRET}" > "${BATS_TEST_TMPDIR}/private.txt"
  export CECH_PRIVATE_TERMS="${BATS_TEST_TMPDIR}/private.txt"
  cd -- "${REPO}" || return
}

# plant <text>: stage a file holding text.
plant() {
  printf '%s\n' "$1" > "${REPO}/planted.txt"
  g add planted.txt
}

# message <text>: write the commit message file MESSAGE.
message() {
  MESSAGE="${BATS_TEST_TMPDIR}/message"
  printf '%s\n' "$1" > "${MESSAGE}"
}

# commit_with_hooks <git commit args...>: commit through scripts/hooks.
commit_with_hooks() {
  run g -c core.hooksPath=scripts/hooks commit -q "$@"
  refute_output_has "${SECRET}"
}

# task_field <task> <field>: print the items of a list field of a task,
# one per line, as `mise tasks info --json` gives them. run, depends and
# env are lists there; env holds NAME=value items.
task_field() {
  local json
  json=$(mise tasks info "$1" --json) || return
  printf '%s\n' "${json}" | awk -v key="\"$2\": " '
    index($0, "  " key) == 1 && /\[\],?$/ { exit }
    index($0, "  " key "[") == 1 { inside = 1; next }
    inside && /^  \]/ { exit }
    inside { sub(/^ *"/, ""); sub(/",?$/, ""); print }'
}

# assert_task <task> <field> <item>...: fail unless the task's field
# holds exactly the items, in order.
assert_task() {
  local task=$1 field=$2
  shift 2
  run task_field "${task}" "${field}"
  assert_status 0
  assert_output_is "$(printf '%s\n' "$@")"
}

@test "check depends on every gate" {
  assert_task check depends terms lint format:check test
}

@test "the task list is exactly the gates and hooks" {
  run mise tasks ls --name-only --hidden
  assert_status 0
  assert_output_is "$(printf '%s\n' check commit-msg format:check format:check:shell format:fix format:fix:shell lint lint:shell pre-commit terms test)"
}

@test "each task runs its gate" {
  assert_task terms run "scripts/check-terms"
  assert_task lint:shell run "scripts/check-shell lint"
  assert_task lint depends "lint:*"
  assert_task format:check:shell run "scripts/check-shell format"
  assert_task format:check depends "format:check:*"
  assert_task format:fix:shell run "scripts/check-shell fix"
  assert_task format:fix depends "format:fix:*"
  assert_task test run "bats test"
  assert_task check run "echo check: every gate passed"
  assert_task pre-commit run "scripts/check-terms --staged" \
    "scripts/staged-tree scripts/check-shell lint" \
    "scripts/staged-tree scripts/check-shell format"
  # shellcheck disable=SC2016 # mise expands usage_file, not this shell
  assert_task commit-msg run 'scripts/check-terms --message \"${usage_file?}\"'
}

@test "no task sets its own shell, environment or directory" {
  local task
  for task in $(mise tasks ls --name-only --hidden); do
    assert_task "${task}" env
    case "${task}" in
      check | lint | format:check | format:fix) ;;
      *) assert_task "${task}" depends ;;
    esac
    run mise tasks info "${task}" --json
    assert_output_has "\"dir\": \"${REPO}\","
    assert_output_has '"shell": "sh -o errexit -c",'
  done
}

@test "only the test task has a Windows command" {
  run grep -c '^run_windows = ' mise.toml
  assert_output_is 1
  run grep -A3 '^\[tasks.test\]$' mise.toml
  assert_output_has "run_windows"
}

@test "test task runs every suite" {
  local want
  want=$(bats --count test)
  run mise run -q test -- --count
  assert_status 0
  assert_output_is "${want}"
}

@test "every locked download has a checksum" {
  run awk '
    /^\[tools\..*platforms/ { if (section != "" && !sum) print section; section = $0; sum = 0 }
    /^checksum = "sha256:/ { sum = 1 }
    END { if (section != "" && !sum) print section }' mise.lock
  assert_status 0
  assert_output_is ""
}

@test "terms task finds a public term" {
  plant "Fixes ABC-123 in the message only."
  printf 'See ABC-123.\n' > "${REPO}/branch-note.txt"
  g add branch-note.txt
  g commit -q -m "docs: add note ABC-123"
  run mise run terms -- --range HEAD~1..HEAD
  assert_status 1
  assert_output_has "tracker-key"
}

@test "lint task lints the shell code" {
  # shellcheck disable=SC2016 # the planted script must hold a literal $1
  printf '#!/bin/sh\necho $1\n' > scripts/bad
  g add --chmod=+x scripts/bad
  run mise run lint
  assert_status 1
  assert_output_has "SC2086"
}

@test "format:check task checks the shell format" {
  printf '#!/bin/sh\nif true; then\necho x\nfi\n' > scripts/flat
  g add --chmod=+x scripts/flat
  run mise run format:check
  assert_status 1
  assert_output_has "scripts/flat"
}

@test "format:fix task rewrites the shell code" {
  printf '#!/bin/sh\nif true; then\necho x\nfi\n' > scripts/flat
  g add --chmod=+x scripts/flat
  run mise run format:fix
  assert_status 0
  run cat scripts/flat
  assert_output_is "$(printf '#!/bin/sh\nif true; then\n  echo x\nfi')"
}

@test "terms task uses the caller's private list" {
  plant "Write to ${SECRET}."
  run mise run terms -- --staged
  refute_output_has "${SECRET}"
  assert_status 1
  assert_output_has "planted.txt:1"
}

@test "mise sets no CECH variables" {
  run mise env -s bash
  assert_status 0
  refute_output_has "CECH_"
}

@test "commit-msg task checks its file" {
  message "fix: thing ABC-123"
  run mise run commit-msg "${MESSAGE}"
  assert_status 1
  assert_output_has "tracker-key"
  message "fix: thing"
  run mise run commit-msg "${MESSAGE}"
  assert_status 0
}

@test "commit-msg task needs a file" {
  run mise run commit-msg
  assert_status 1
}

@test "pre-commit hook blocks a staged private term" {
  plant "Write to ${SECRET}."
  commit_with_hooks -m "docs: add note"
  assert_status 1
  assert_output_has "planted.txt:1"
}

@test "commit-msg hook blocks a tracker key" {
  plant "Clean text."
  commit_with_hooks -m "docs: add note for ABC-123"
  assert_status 1
  assert_output_has "tracker-key"
}

@test "pre-commit hook lints the staged shell code" {
  # shellcheck disable=SC2016 # the planted script must hold a literal $1
  printf '#!/bin/sh\necho $1\n' > scripts/bad
  g add --chmod=+x scripts/bad
  # shellcheck disable=SC2016 # the fixed copy on disk is not staged
  printf '#!/bin/sh\necho "$1"\n' > scripts/bad
  commit_with_hooks -m "chore: add script"
  assert_status 1
  assert_output_has "SC2086"
}

@test "pre-commit hook checks the staged format" {
  printf '#!/bin/sh\nif true; then\necho x\nfi\n' > scripts/flat
  g add --chmod=+x scripts/flat
  printf '#!/bin/sh\nif true; then\n  echo x\nfi\n' > scripts/flat
  commit_with_hooks -m "chore: add script"
  assert_status 1
  assert_output_has "scripts/flat"
}

@test "hooks let a clean commit through" {
  plant "Clean text."
  commit_with_hooks -m "docs: add note"
  assert_status 0
  run g log --format=%s -1
  assert_output_is "docs: add note"
}
