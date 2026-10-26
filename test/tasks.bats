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
  # Keep go off the network: a case that needs a module fails at once.
  export GOPROXY=off
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

# commit_with_hooks <git commit args...>: commit through scripts/hooks,
# signed off.
commit_with_hooks() {
  run g -c core.hooksPath=scripts/hooks commit -q -s "$@"
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
  assert_task check depends commits terms lint format:check docs:check test
}

@test "the task list is exactly the gates and hooks" {
  run mise tasks ls --name-only --hidden
  assert_status 0
  assert_output_is "$(printf '%s\n' check commit-msg commits docs:check format:check format:check:go format:check:markdown format:check:shell format:fix format:fix:go format:fix:markdown format:fix:shell lint lint:actions lint:go lint:markdown lint:shell pre-commit terms test test:bats test:go vuln)"
}

@test "each task runs its gate" {
  assert_task terms run "scripts/check-terms"
  assert_task commits run "scripts/check-commits"
  assert_task lint:actions run "actionlint .github/workflows/*.y*ml" \
    "zizmor --offline --pedantic --strict-collection ."
  assert_task lint:shell run "scripts/check-shell lint"
  assert_task lint:markdown run "rumdl check ."
  assert_task lint:go run "scripts/check-go lint"
  assert_task lint depends "lint:*"
  assert_task format:check:shell run "scripts/check-shell format"
  assert_task format:check:markdown run "rumdl fmt --check ."
  assert_task format:check:go run "scripts/check-go format"
  assert_task format:check depends "format:check:*"
  assert_task format:fix:shell run "scripts/check-shell fix"
  assert_task format:fix:markdown run "rumdl fmt ."
  assert_task format:fix:go run "scripts/check-go fix"
  assert_task format:fix depends "format:fix:*"
  assert_task docs:check run "lychee --no-progress ."
  assert_task test depends "test:*"
  assert_task test:bats run "bats test"
  assert_task test:go run "scripts/check-go test"
  assert_task vuln run "scripts/check-go vuln"
  assert_task check run
  assert_task pre-commit run "scripts/check-terms --staged" \
    "scripts/staged-tree scripts/check-shell lint" \
    "scripts/staged-tree scripts/check-shell format" \
    "scripts/staged-tree scripts/check-go lint" \
    "scripts/staged-tree scripts/check-go format" \
    "scripts/staged-tree sh -c 'actionlint .github/workflows/*.y*ml'" \
    "scripts/staged-tree zizmor --offline --pedantic --strict-collection ." \
    "scripts/staged-tree rumdl check ." \
    "scripts/staged-tree rumdl fmt --check ."
  # shellcheck disable=SC2016 # mise expands usage_file, not this shell
  assert_task commit-msg run 'scripts/check-terms --message \"${usage_file?}\"' \
    'scripts/check-commits --message \"${usage_file?}\"'
}

@test "no task sets its own shell, environment or directory" {
  local task dir
  # mise reports the path with symlinks resolved, as /private/var for
  # the /var of a macOS temporary directory.
  dir=$(pwd -P)
  for task in $(mise tasks ls --name-only --hidden); do
    assert_task "${task}" env
    case "${task}" in
      check | lint | format:check | format:fix | test) ;;
      *) assert_task "${task}" depends ;;
    esac
    run mise tasks info "${task}" --json
    assert_output_has "\"dir\": \"${dir}\","
    assert_output_has '"shell": "sh -o errexit -c",'
  done
}

@test "only the test:bats task has a Windows command" {
  run grep -c '^run_windows = ' mise.toml
  assert_output_is 1
  run grep -A3 '^\[tasks."test:bats"\]$' mise.toml
  assert_output_has "run_windows"
}

@test "test:bats task runs every suite" {
  local want
  want=$(bats --count test)
  run mise run -q test:bats -- --count
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

@test "lint task lints the workflows" {
  mkdir -p .github/workflows
  printf 'on: push\njobs:\n  a:\n    steps: []\n' > .github/workflows/bad.yml
  g add .github/workflows/bad.yml
  run mise run lint
  assert_status 1
  assert_output_has '"runs-on" section is missing'
}

@test "lint task finds an unpinned action" {
  mkdir -p .github/workflows
  printf '%s\n' 'name: x' 'on: push' 'permissions: {}' 'jobs:' '  a:' \
    '    name: a' '    runs-on: ubuntu-24.04' '    steps:' \
    '      - uses: actions/checkout@v4' '        with:' \
    '          persist-credentials: false' > .github/workflows/tag.yml
  g add .github/workflows/tag.yml
  run mise run lint:actions
  assert_failed
  assert_output_has "unpinned-uses"
}

@test "lint task lints the Markdown" {
  # A first line that is not a heading, which rumdl cannot fix.
  printf 'Text.\n' > note.md
  g add note.md
  run mise run lint
  assert_status 1
  assert_output_has "MD041"
}

@test "format:check task checks the Markdown format" {
  printf '# Note\n\nOne sentence. Another sentence.\n' > note.md
  g add note.md
  run mise run format:check
  assert_status 1
  assert_output_has "note.md"
}

@test "docs:check task finds a dead local link" {
  printf '# Note\n\nSee [the guide](missing-guide.md).\n' > note.md
  g add note.md
  # Offline, so the case checks local links only.
  run mise run docs:check -- --offline
  assert_status 2
  assert_output_has "missing-guide.md"
}

@test "docs:check task checks links in hidden directories" {
  mkdir -p .github
  printf '# Note\n\nSee [the guide](hidden-missing-guide.md).\n' > .github/note.md
  g add .github/note.md
  run mise run docs:check -- --offline
  assert_status 2
  assert_output_has "hidden-missing-guide.md"
}

@test "docs:check task checks links in YAML files" {
  mkdir -p .github/ISSUE_TEMPLATE
  # lychee reads YAML as plain text, so only a full URL is found.
  printf 'url: file:///yaml-missing-guide.md\n' > .github/ISSUE_TEMPLATE/form.yml
  g add .github/ISSUE_TEMPLATE/form.yml
  run mise run docs:check -- --offline
  assert_status 2
  assert_output_has "yaml-missing-guide.md"
}

@test "docs:check task skips the .git directory" {
  printf '# Note\n\nSee [the guide](git-missing-guide.md).\n' > .git/note.md
  run mise run docs:check -- --offline
  assert_status 0
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

@test "a suite run from git keeps out of the calling repository" {
  # A suite run as a hook or from rebase --exec gets git's repository
  # and identity variables. This one writes config in a repository of
  # its own and checks that git sees the cases' author there.
  local caller="${BATS_TEST_TMPDIR}/caller" suite="${BATS_TEST_TMPDIR}/probe.bats"
  git init -q "${caller}"
  printf 'load %s\n' "${SRC}/test/helpers" > "${suite}"
  cat >> "${suite}" << 'EOF'
setup_file() { setup_suite_env; }
@test "probe" {
  REPO="${BATS_TEST_TMPDIR}/own"
  git init -q "${REPO}"
  g config probe.key set
  case "$(g var GIT_AUTHOR_IDENT)" in
    "Test <test@example.com> "*) ;;
    *) return 1 ;;
  esac
}
EOF
  GIT_DIR="${caller}/.git" GIT_AUTHOR_NAME=Caller GIT_AUTHOR_EMAIL=caller@example.com \
    run bats "${suite}"
  assert_status 0
  assert_output_has "ok 1 probe"
  run git -C "${caller}" config --local probe.key
  assert_status 1
}

@test "mise sets no CECH variables" {
  run mise env -s bash
  assert_status 0
  refute_output_has "CECH_"
}

@test "commit-msg task checks its file" {
  message "$(printf 'fix: thing ABC-123\n\nSigned-off-by: Test <test@example.com>')"
  run mise run commit-msg "${MESSAGE}"
  assert_status 1
  assert_output_has "tracker-key"
  message "$(printf 'Fix thing\n\nSigned-off-by: Test <test@example.com>')"
  run mise run commit-msg "${MESSAGE}"
  assert_status 1
  assert_output_has "header type"
  message "$(printf 'fix: thing\n\nSigned-off-by: Test <test@example.com>')"
  run mise run commit-msg "${MESSAGE}"
  assert_status 0
}

@test "commits task checks the commits missing from origin/main" {
  # The template's commit has no sign-off, so a pass shows it was left out.
  g update-ref refs/remotes/origin/main HEAD
  plant "Clean text."
  g commit -q -s -m "docs: add note"
  run mise run commits
  assert_status 0
  printf 'More text.\n' >> planted.txt
  g add planted.txt
  g commit -q -m "docs: add more"
  local bad
  bad=$(g rev-parse --short=12 HEAD)
  run mise run commits
  assert_status 1
  assert_output_has "commit ${bad}: no Signed-off-by: trailer"
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

@test "commit-msg hook blocks a commit without sign-off" {
  plant "Clean text."
  run g -c core.hooksPath=scripts/hooks commit -q -m "docs: add note"
  assert_status 1
  assert_output_has "no Signed-off-by: trailer"
}

@test "commit-msg hook checks the sign-off against the author" {
  plant "Clean text."
  commit_with_hooks --author "Other <other@example.com>" -m "docs: add note"
  assert_status 1
  assert_output_has "no Signed-off-by: trailer carries the author's email"
}

@test "commit-msg hook blocks a bad header" {
  plant "Clean text."
  commit_with_hooks -m "Add note."
  assert_status 1
  assert_output_has "header type"
}

@test "commit-msg hook lets a fixup through" {
  plant "Clean text."
  commit_with_hooks -m "docs: add note"
  assert_status 0
  printf 'More text.\n' >> planted.txt
  g add planted.txt
  run g -c core.hooksPath=scripts/hooks commit -q --fixup HEAD
  assert_status 0
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

@test "pre-commit hook lints the staged workflows" {
  mkdir -p .github/workflows
  printf 'on: push\njobs:\n  a:\n    steps: []\n' > .github/workflows/bad.yml
  g add .github/workflows/bad.yml
  rm .github/workflows/bad.yml
  commit_with_hooks -m "ci: add workflow"
  assert_status 1
  assert_output_has '"runs-on" section is missing'
}

# Guards a change to check-go or staged-tree after which check-go finds
# cli/ somewhere other than in the staged tree, so the hook checks the
# working copy and passes. assert_task pins the task's text, not this.
@test "pre-commit hook lints the staged Go code" {
  printf 'package bad\n\nimport "os"\n\n// Remove drops its error.\nfunc Remove() { os.Remove("x") }\n' > cli/bad.go
  g add cli/bad.go
  printf 'package bad\n' > cli/bad.go
  commit_with_hooks -m "feat(cli): add bad code"
  assert_status 1
  assert_output_has "(errcheck)"
}

@test "pre-commit hook lints the staged Markdown" {
  printf 'Text.\n' > note.md
  g add note.md
  printf '# Note\n\nText.\n' > note.md
  commit_with_hooks -m "docs: add note"
  assert_status 1
  assert_output_has "MD041"
}

@test "pre-commit hook checks the staged Markdown format" {
  printf '# Note\n\nOne sentence. Another sentence.\n' > note.md
  g add note.md
  printf '# Note\n\nOne sentence.\nAnother sentence.\n' > note.md
  commit_with_hooks -m "docs: add note"
  assert_status 1
  assert_output_has "note.md"
}

@test "hooks let a clean commit through" {
  plant "Clean text."
  commit_with_hooks -m "docs: add note"
  assert_status 0
  run g log --format=%s -1
  assert_output_is "docs: add note"
}
