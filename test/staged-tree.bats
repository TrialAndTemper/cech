#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests of scripts/staged-tree, through which the pre-commit hook runs
# its lint and format checks. As ADR 4 asks of gate tests, each case
# guards a way a check run through it could pass when it should fail,
# where review would likely miss it, and says which: a check run this
# way shows its effect only during a commit. The other ways were checked
# by hand when their cases were dropped.
#
# Requires: bats (version in mise.toml) and git >= 2.28 (init -b).

load helpers

setup_file() {
  setup_suite_env
  export TEMPLATE="${BATS_FILE_TMPDIR}/template"
  REPO=${TEMPLATE}
  mkdir -p "${REPO}/scripts"
  cp -- "${SRC}/scripts/staged-tree" "${REPO}/scripts/"
  cp -R -- "${SRC}/scripts/lib" "${REPO}/scripts/"
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

# Guards a check that reads the files on disk instead of the staged
# text, which only a commit would show.
@test "the command sees the staged text" {
  printf 'staged\n' > note.txt
  g add note.txt
  printf 'on disk\n' > note.txt
  staged_tree cat note.txt
  assert_status 0
  assert_output_is "staged"
}

# Guards the relative index path that git can pass during a commit:
# git reads an index it cannot find as empty, so a check would see no
# files.
@test "a relative index path still works" {
  printf 'new\n' > new.txt
  g add new.txt
  run env GIT_INDEX_FILE=.git/index "${REPO}/scripts/staged-tree" git ls-files
  assert_status 0
  assert_output_has "new.txt"
}

# Guards a shell, such as dash or macOS sh, whose EXIT trap replaces
# the check's exit status with the cleanup's.
@test "the command's exit status is kept" {
  staged_tree sh -c 'exit 3'
  assert_status 3
}
