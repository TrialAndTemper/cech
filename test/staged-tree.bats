#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests for scripts/staged-tree: each case stages files in its own copy
# of a throwaway repository and checks what a command run through it
# sees.
#
# Requires: bats (version in mise.toml) and git >= 2.28 (init -b).

load helpers

setup_file() {
  setup_suite_env
  export TEMPLATE="${BATS_FILE_TMPDIR}/template"
  REPO=${TEMPLATE}
  mkdir -p "${REPO}/scripts"
  cp -- "${SRC}/scripts/staged-tree" "${REPO}/scripts/"
  printf 'committed\n' > "${REPO}/note.txt"
  commit_template
}

setup() {
  use_template
  cd -- "${REPO}" || return
}

# staged_tree <command> [args...]: run a command through staged-tree.
staged_tree() {
  run "${REPO}/scripts/staged-tree" "$@"
}

@test "the command sees the staged text" {
  printf 'staged\n' > note.txt
  g add note.txt
  printf 'on disk\n' > note.txt
  staged_tree cat note.txt
  assert_status 0
  assert_output_is "staged"
}

@test "the command sees staged files, not untracked ones" {
  printf 'new\n' > new.txt
  g add new.txt
  printf 'stray\n' > stray.txt
  staged_tree ls
  assert_status 0
  assert_output_has "new.txt"
  refute_output_has "stray.txt"
}

@test "git in the command sees the index" {
  printf 'new\n' > new.txt
  g add new.txt
  staged_tree git ls-files
  assert_status 0
  assert_output_has "new.txt"
  assert_output_has "scripts/staged-tree"
}

@test "a relative index path still works" {
  printf 'new\n' > new.txt
  g add new.txt
  run env GIT_INDEX_FILE=.git/index "${REPO}/scripts/staged-tree" git ls-files
  assert_status 0
  assert_output_has "new.txt"
}

@test "the command's exit status is kept" {
  staged_tree sh -c 'exit 3'
  assert_status 3
}

@test "the checkout is removed" {
  # shellcheck disable=SC2016 # the inner shell expands $PWD
  staged_tree sh -c 'printf "%s\n" "$PWD"'
  assert_status 0
  if [[ -e ${output} ]]; then
    printf 'checkout left behind: %s\n' "${output}"
    return 1
  fi
}

@test "no command" {
  staged_tree
  assert_status 2
  assert_output_has "no command given"
}

@test "outside a repository" {
  mkdir "${BATS_TEST_TMPDIR}/plain"
  cd -- "${BATS_TEST_TMPDIR}/plain" || return
  run env GIT_CEILING_DIRECTORIES="${BATS_TEST_TMPDIR}" "${REPO}/scripts/staged-tree" true
  assert_status 2
  assert_output_has "not in a git repository"
}
