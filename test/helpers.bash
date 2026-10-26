# shellcheck shell=bash
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; suites set TEMPLATE
# Helpers for the bats suites under test/, loaded by each with
# `load helpers`. Each suite builds a template repository once in
# setup_file, and each case works on its own copy of it, so cases do not
# depend on one another. bash 3.2 compatible, since on a stock macOS bats
# runs under the system bash, which is 3.2. In bash before 4.1 a failing
# [[ ]] or (( )) does not fail a case unless it is the case's last
# command, so cases assert through the helpers below, which return 1,
# and no suite line starts with [[ or ((. Run the suites as
# `mise run test`, which puts the pinned tools on PATH.

# shellcheck disable=SC2034 # the suites read SRC
SRC=$(cd -- "${BATS_TEST_DIRNAME}/.." && pwd)

# setup_suite_env: settings every suite needs, run from setup_file. Keep
# the user's git configuration (signing, hooks) out of the tests. git
# exports its repository variables (GIT_DIR, GIT_WORK_TREE,
# GIT_INDEX_FILE, settings given with -c, and the rest that
# `git rev-parse --local-env-vars` lists) to hooks and to
# `git rebase --exec`; left set, they would point every git command of
# a case at the calling repository. The author and committer variables,
# which git also exports to hooks, would override the cases' identity.
# CECH_RANGE, which CI sets for the gates, would change the gates' default
# mode.
setup_suite_env() {
  local v
  for v in $(git rev-parse --local-env-vars) GIT_NAMESPACE \
    GIT_AUTHOR_NAME GIT_AUTHOR_EMAIL GIT_AUTHOR_DATE \
    GIT_COMMITTER_NAME GIT_COMMITTER_EMAIL GIT_COMMITTER_DATE CECH_RANGE; do
    unset "${v}"
  done
  export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
}

# g <git args...>: git in the case's repository, without the user's
# identity, signing or hooks.
g() {
  git -C "${REPO}" -c user.name=Test -c user.email=test@example.com \
    -c commit.gpgsign=false -c tag.gpgsign=false -c core.hooksPath=/dev/null "$@"
}

# commit_template: from setup_file, commit everything in REPO as the
# template's first commit. Executable files are marked so in the index
# too, since Git for Windows does not take the exec bit from the file
# system.
commit_template() {
  local f
  g init -q -b main
  g add -A
  g ls-files > "${BATS_FILE_TMPDIR}/files"
  while IFS= read -r f; do
    if [[ -x ${REPO}/${f} ]]; then g add --chmod=+x -- "${f}"; fi
  done < "${BATS_FILE_TMPDIR}/files"
  g commit -q -m "chore: start"
}

# use_template: give the case its own copy of the template repository.
use_template() {
  REPO="${BATS_TEST_TMPDIR}/repo"
  cp -R -- "${TEMPLATE}" "${REPO}"
}

# commit_and_drop <file> <text>: commit a file, then a commit removing it.
commit_and_drop() {
  printf '%s\n' "$2" > "${REPO}/$1"
  g add -- "$1"
  g commit -q -m "docs: add note"
  g rm -q -- "$1"
  g commit -q -m "docs: drop note"
}

# fake_tool <tool> <script body>: put a fake tool first on PATH for the
# rest of the case. In the body, REAL names the real tool.
fake_tool() {
  local real
  real=$(command -v "$1")
  if [[ ${real} == *"'"* ]]; then
    printf 'cannot wrap a path that holds a quote: %s\n' "${real}"
    return 1
  fi
  mkdir -p -- "${BATS_TEST_TMPDIR}/bin"
  printf "#!/bin/sh\nREAL='%s'\n%s\n" "${real}" "$2" > "${BATS_TEST_TMPDIR}/bin/$1"
  chmod 755 "${BATS_TEST_TMPDIR}/bin/$1"
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"
}

# assert_status <code>: fail the case unless the last `run` exited so.
assert_status() {
  if [[ ${status} -ne $1 ]]; then
    printf 'expected exit %s, got %s; output:\n%s\n' "$1" "${status}" "${output}"
    return 1
  fi
}

# assert_failed: fail the case unless the last `run` exited non-zero.
assert_failed() {
  if [[ ${status} -eq 0 ]]; then
    printf 'expected a failure, got exit 0; output:\n%s\n' "${output}"
    return 1
  fi
}

# assert_output_has <text>: fail the case unless the output holds text.
assert_output_has() {
  if [[ ${output} != *"$1"* ]]; then
    printf 'output lacks %s; output:\n%s\n' "$1" "${output}"
    return 1
  fi
}

# assert_output_is <text>: fail the case unless the output is exactly
# text; an empty text means no output.
assert_output_is() {
  if [[ ${output} != "$1" ]]; then
    printf 'output is not %s; output:\n%s\n' "$1" "${output}"
    return 1
  fi
}

# refute_output_has <text>: fail the case if the output holds text,
# ignoring case.
refute_output_has() {
  if printf '%s' "${output}" | grep -qiF -- "$1"; then
    printf 'output must not hold %s; output:\n%s\n' "$1" "${output}"
    return 1
  fi
}
