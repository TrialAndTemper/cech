#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests of scripts/check-adr, the gate on ADR length (ADR 1 item 6). As
# ADR 4 asks of gate tests, each case guards a way the gate could pass
# when it should fail, where review would likely miss it, and says
# which. The other ways were checked by hand when the gate was added.
# The pre-commit hook's run on the staged tree has its case in
# tasks.bats.
#
# Requires: bats (version in mise.toml), git >= 2.28 (init -b), and awk.

load helpers

setup_file() {
  setup_suite_env
  export TEMPLATE="${BATS_FILE_TMPDIR}/template"
  REPO=${TEMPLATE}
  mkdir -p "${REPO}/scripts" "${REPO}/docs/adr"
  cp -- "${SRC}/scripts/check-adr" "${REPO}/scripts/"
  cp -R -- "${SRC}/scripts/lib" "${REPO}/scripts/"
  words 10 > "${REPO}/docs/adr/0001-first.md"
  commit_template
}

setup() {
  use_template
}

# words <n>: print n words of prose, ten to a line.
words() {
  local i
  for ((i = 1; i <= $1; i++)); do
    printf 'w '
    if ((i % 10 == 0)); then printf '\n'; fi
  done
  printf '\n'
}

# plant_adr <lines...>: stage docs/adr/0002-planted.md with the lines.
plant_adr() {
  printf '%s\n' "$@" > "${REPO}/docs/adr/0002-planted.md"
  g add docs/adr/0002-planted.md
}

# run_adr: run the gate in the case's repository.
run_adr() {
  run "${REPO}/scripts/check-adr"
}

# Guards an off-by-one in the limit, which reads as "under 800": 800
# words must fail. One line separates its words by tabs, which guards a
# word count that splits on spaces alone.
@test "800 words fail" {
  local prose
  prose=$(words 790)
  plant_adr "${prose}" "$(printf 'w\tw\tw\tw\tw\tw\tw\tw\tw\tw')"
  run_adr
  assert_status 1
  assert_output_has "0002-planted.md: 800 words of prose"
}

# Guards a fence that never closes hiding the rest of the file, as
# CommonMark renders it as code to the end.
@test "an unclosed fence is a finding" {
  local prose
  prose=$(words 900)
  plant_adr '# 2. Planted' '' '   ```toml' "${prose}"
  run_adr
  assert_status 1
  assert_output_has "0002-planted.md:3: this fence never closes"
}

# Guards a change to the awk program that closes a fence on a line that
# CommonMark keeps inside it: another fence character, as the one-liner
# `awk '/^```/{c=!c}'` would, a shorter run, a run with text after it,
# or one indented four or more columns into the fence's container, by
# spaces or by a tab, even when the opener is indented too. The real
# closer would then open a fence that hides the prose after it up to
# the next fence line.
@test "only a matching fence line closes a fence" {
  local inner prose tab
  prose=$(words 800)
  tab=$(printf '\t')
  for inner in '`````' '~~~' '~~~~ x' '    ~~~~' "${tab}~~~~"; do
    plant_adr '~~~~' "${inner}" '~~~~' "${prose}" '~~~~'
    run_adr
    # Name the line that closed the fence.
    assert_status 1 || {
      echo "inner line: ${inner}"
      return 1
    }
    assert_output_has "0002-planted.md: 800 words of prose" || {
      echo "inner line: ${inner}"
      return 1
    }
  done
  # A closer counts its indent from the container, not from the opener.
  plant_adr '  ```' '     ```' '  ```' "${prose}" '```'
  run_adr
  assert_output_has "0002-planted.md: 800 words of prose"
}

# Guards a change that opens a fence on a backtick line whose info
# string holds a backtick, which CommonMark reads as inline code, or on
# a fence line after `>`, which the gate reads as prose: either way the
# prose after it would be hidden up to the next fence line.
@test "a backtick run with a backtick after it, or one after >, opens no fence" {
  local prose
  prose=$(words 800)
  # shellcheck disable=SC2016 # the backticks are the fixture
  plant_adr '```x`' "${prose}" '```' 'code' '```'
  run_adr
  assert_status 1
  assert_output_has "0002-planted.md: 801 words of prose"
  # shellcheck disable=SC2016 # the backticks are the fixture
  plant_adr '> ```' "${prose}" '```' 'code' '```'
  run_adr
  assert_output_has "0002-planted.md: 802 words of prose"
}

# Guards a change that opens a fence on a line indented four or more
# columns outside a list item, by spaces or by a tab: CommonMark reads
# it as part of a paragraph or as indented code, so the prose after it
# would be hidden up to the next fence line. Each way of misreading
# where a list item starts or ends has a fixture: an item that has
# ended, a gap of five columns or of tabs after the marker, a marker in
# indented code, a thematic break, and a number other than 1 after a
# paragraph line. The trailing
# fence line also leaves a fence unclosed, which the case need not
# check.
@test "an indented fence line outside a list item opens no fence" {
  local prose indented
  prose=$(words 800)
  # shellcheck disable=SC2016 # the backticks are the fixture
  for indented in '    ```' "$(printf '\t```')"; do
    plant_adr 'Para' "${indented}" "${prose}" '```'
    run_adr
    # Name the indent that opened the fence.
    assert_output_has "0002-planted.md: 802 words of prose" || {
      echo "after a paragraph: [${indented}]"
      return 1
    }
    plant_adr 'Intro' '' "${indented}" "${prose}" '```'
    run_adr
    assert_output_has "0002-planted.md: 802 words of prose" || {
      echo "after a blank line: [${indented}]"
      return 1
    }
  done
  # A list item ends at the first line indented less than its text.
  plant_adr '- item' '' 'Para' '' '    ```' "${prose}" '```'
  run_adr
  assert_output_has "0002-planted.md: 804 words of prose"
  # Text five or more columns after a marker is code, so the item's
  # text starts one column after the marker, not at that text.
  plant_adr '-     code' '' '      ```' "${prose}" '```'
  run_adr
  assert_output_has "0002-planted.md: 803 words of prose"
  # Two tabs after the marker are a gap of more than four columns.
  plant_adr "$(printf -- '-\t\tcode')" '' '      ```' "${prose}" '```'
  run_adr
  assert_output_has "0002-planted.md: 803 words of prose"
  # The gap after the marker counts from the marker's column.
  plant_adr "$(printf -- '-\tcode')" '' '        ```' "${prose}" '```'
  run_adr
  assert_output_has "0002-planted.md: 803 words of prose"
  # A marker in indented code starts no list item.
  plant_adr 'Intro' '' '    - x' '' '       ```' "${prose}" '```'
  run_adr
  assert_output_has "0002-planted.md: 804 words of prose"
  # A thematic break is no list item.
  plant_adr '* * *' '' '    ```' "${prose}" '```'
  run_adr
  assert_output_has "0002-planted.md: 804 words of prose"
  # Only 1 starts an ordered list after a paragraph line.
  plant_adr 'Some text' '2026. year' '      ```' "${prose}" '```'
  run_adr
  assert_output_has "0002-planted.md: 805 words of prose"
}

# Guards a file list mangled between git ls-files and the read loop,
# such as git quoting a non-ASCII name or the shell splitting one at a
# space: the skip for a file deleted but not staged would then pass
# over the ADR unseen.
@test "an ADR with a space and a non-ASCII letter in its name is checked" {
  local prose
  prose=$(words 800)
  printf '%s\n' "${prose}" > "${REPO}/docs/adr/0002-é b.md"
  g add -- "docs/adr/0002-é b.md"
  run_adr
  assert_status 1
  assert_output_has "0002-é b.md: 800 words of prose"
}

# Guards a moved docs/adr/ or a changed pattern, after which the gate
# would find nothing and pass.
@test "no ADR stops the check" {
  # The template's one short ADR passes.
  run_adr
  assert_status 0
  g rm -q docs/adr/0001-first.md
  run_adr
  assert_status 2
  assert_output_has "no ADR found"
}
