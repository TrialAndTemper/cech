#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests of the mise tasks and the git hooks (ADR 4). As ADR 4 asks of
# gate tests, each case guards a way `mise run check` or a hook could
# pass when it should fail, where review would likely miss it, and says
# which: most are a tool bump that changes what a gate sees, or a hook,
# which checks the staged tree. The other ways, such as a task whose
# command changes in mise.toml, show in the diff and were checked by
# hand when their cases were dropped. The vuln task never blocks a merge
# (ADR 16 item 3), so it has no case.
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
  # Cases match mise's output, which must be plain text.
  unset CLICOLOR_FORCE
  export NO_COLOR=1

  download_go_modules

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

# commit_with_hooks <git commit args...>: commit through scripts/hooks,
# signed off.
commit_with_hooks() {
  run g -c core.hooksPath=scripts/hooks commit -q -s "$@"
  refute_output_has "${SECRET}"
}

# Guards a mise bump after which a pattern in depends, such as lint:*,
# matches fewer tasks, so that check skips a gate.
@test "check runs every gate task" {
  local tasks task
  tasks=$(mise tasks ls --name-only --hidden) || return
  # An empty list would let the loop check nothing.
  run printf '%s\n' "${tasks}"
  assert_output_has "lint:shell"
  run mise run -n check
  assert_status 0
  for task in ${tasks}; do
    case "${task}" in
      commits | terms | docs:check | lint:* | format:check:* | test:*)
        assert_output_has "[${task}] \$"
        ;;
      *) ;;
    esac
  done
}

# Guards a pin bump whose `mise lock` writes a download without a
# checksum, which review of a long lock file diff would likely miss.
@test "every locked download has a checksum" {
  # A lock file laid out in some other way has no platform section, and
  # fails.
  run awk '
    /^\[/ { if (section != "" && !sum) print section; section = "" }
    /^\[tools\..*platforms/ { section = $0; sum = 0; n++ }
    /^checksum = "sha256:[0-9a-f]+"$/ { sum = 1 }
    END { if (section != "" && !sum) print section; if (!n) print "no platform sections" }' mise.lock
  assert_status 0
  assert_output_is ""
}

# Guards a zizmor bump after which --strict-collection no longer makes
# it fail on a file it cannot read, which it would then skip.
@test "lint:actions task fails on a file zizmor cannot read" {
  printf 'version: 2\nupdates: [\n' > .github/dependabot.yml
  g add .github/dependabot.yml
  run mise run lint:actions
  assert_failed
  assert_output_has "failed to load"
}

# Guards a lychee bump after which `hidden` in lychee.toml no longer
# takes in hidden directories such as .github.
@test "docs:check task checks links in hidden directories" {
  mkdir -p .github
  printf '# Note\n\nSee [the guide](hidden-missing-guide.md).\n' > .github/note.md
  g add .github/note.md
  run mise run docs:check -- --offline
  assert_status 2
  assert_output_has "hidden-missing-guide.md"
}

# Guards a lychee bump after which `extensions` in lychee.toml no
# longer makes it read YAML files as text.
@test "docs:check task checks links in YAML files" {
  mkdir -p .github/ISSUE_TEMPLATE
  # lychee reads YAML as plain text, so only a full URL is found.
  printf 'url: file:///yaml-missing-guide.md\n' > .github/ISSUE_TEMPLATE/form.yml
  g add .github/ISSUE_TEMPLATE/form.yml
  run mise run docs:check -- --offline
  assert_status 2
  assert_output_has "yaml-missing-guide.md"
}

# Guards a rumdl bump after which fmt --check exits 0 on a file it would
# rewrite.
@test "format:check:markdown task fails on a file it would rewrite" {
  printf '# Note\n\nOne sentence. Another sentence.\n' > note.md
  g add note.md
  run mise run format:check:markdown
  assert_status 1
  assert_output_has "note.md"
}

# Guards a mise bump after which --continue-on-error, as CI runs check,
# exits 0 when a gate fails.
@test "check fails in CI when a gate fails" {
  local dir="${BATS_TEST_TMPDIR}/ci"
  mkdir -p "${dir}"
  printf '%s\n' '[task_config]' 'shell = "sh -o errexit -c"' '[tasks.bad]' \
    'run = "exit 1"' '[tasks.good]' 'run = "true"' '[tasks.check]' \
    'depends = ["bad", "good"]' > "${dir}/mise.toml"
  cd -- "${dir}" || return
  run mise run --continue-on-error --output prefix check
  assert_status 1
  assert_output_has "[bad] exited with status 1"
}

# Guards a suite run from a hook or rebase --exec, whose git variables
# would point the cases at the calling repository.
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

# Guards the hook dropping the private list on its way through git and
# mise, or scanning the files on disk instead of the staged ones.
@test "pre-commit hook blocks a staged private term" {
  plant "Write to ${SECRET}."
  printf 'Clean text.\n' > planted.txt
  commit_with_hooks -m "docs: add note"
  assert_status 1
  assert_output_has "planted.txt:1"
}

# Guards the hook missing the author that git passes only in its
# environment, which no run outside a commit shows.
@test "commit-msg hook checks the sign-off against the author" {
  plant "Clean text."
  commit_with_hooks --author "Other <other@example.com>" -m "docs: add note"
  assert_status 1
  assert_output_has "no Signed-off-by: trailer carries the author's email"
}

# Guards the message file that git passes to the hook not reaching the
# term check.
@test "commit-msg hook blocks a tracker key" {
  plant "Clean text."
  commit_with_hooks -m "docs: add note for ABC-123"
  assert_status 1
  assert_output_has "tracker-key"
}

# Guards a change to check-shell or staged-tree after which check-shell
# lists the files on disk instead of the staged tree.
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

# Guards actionlint linting the workflows on disk instead of those in
# the hook's checkout, which has no .git.
@test "pre-commit hook lints the staged workflows" {
  mkdir -p .github/workflows
  printf 'on: push\njobs:\n  a:\n    steps: []\n' > .github/workflows/bad.yml
  g add .github/workflows/bad.yml
  rm .github/workflows/bad.yml
  commit_with_hooks -m "ci: add workflow"
  assert_status 1
  assert_output_has '"runs-on" section is missing'
}

# Guards a zizmor bump that changes what it reads in a checkout without
# .git, or which audits --pedantic adds.
@test "pre-commit hook finds a staged unpinned action" {
  mkdir -p .github/workflows
  printf '%s\n' 'name: x' 'on: push' 'permissions: {}' 'jobs:' '  a:' \
    '    name: a' '    runs-on: ubuntu-24.04' '    steps:' \
    '      - uses: actions/checkout@v4' '        with:' \
    '          persist-credentials: false' > .github/workflows/tag.yml
  g add .github/workflows/tag.yml
  rm .github/workflows/tag.yml
  commit_with_hooks -m "ci: add workflow"
  assert_status 1
  assert_output_has "unpinned-uses"
  # Reported only with --pedantic.
  assert_output_has "concurrency-limits"
}

# Guards a change to check-go or staged-tree after which check-go finds
# cli/ somewhere other than in the staged tree, so the hook checks the
# working copy and passes.
@test "pre-commit hook lints the staged Go code" {
  printf 'package bad\n\nimport "os"\n\n// Remove drops its error.\nfunc Remove() { os.Remove("x") }\n' > cli/bad.go
  g add cli/bad.go
  printf 'package bad\n' > cli/bad.go
  commit_with_hooks -m "feat(cli): add bad code"
  assert_status 1
  assert_output_has "(errcheck)"
}

# Guards a rumdl bump that changes which files it finds in the hook's
# checkout or which settings it reads there, such as the sentence per
# line that only .rumdl.toml asks for.
@test "pre-commit hook lints the staged Markdown" {
  printf '# Note\n\nOne sentence. Another sentence.\n' > note.md
  g add note.md
  printf '# Note\n\nOne sentence.\nAnother sentence.\n' > note.md
  commit_with_hooks -m "docs: add note"
  assert_status 1
  assert_output_has "note.md:3:1: [MD013]"
}

# Guards check-adr or staged-tree reading the ADRs on disk instead of
# the staged ones, so that the hook passes an ADR over the limit.
@test "pre-commit hook checks the staged ADRs" {
  local i
  for ((i = 0; i < 80; i++)); do
    printf 'w w w w w w w w w w\n'
  done > docs/adr/0099-long.md
  g add docs/adr/0099-long.md
  printf 'Short.\n' > docs/adr/0099-long.md
  commit_with_hooks -m "docs(adr): add a long one"
  assert_status 1
  assert_output_has "0099-long.md: 800 words of prose"
}
