#!/usr/bin/env bats
# shellcheck shell=bats
# shellcheck disable=SC2154 # bats sets BATS_*, status and output; helpers.bash sets SRC
# Tests of scripts/check-terms, the term check (ADR 3). As ADR 4 asks
# of gate tests, each case guards a way the check could pass when it
# should fail, where review would likely miss it, and says which. The
# other ways were checked by hand when their cases were dropped. Every
# case also checks that the output never shows the private term. Values
# the check would flag in this file are built at run time.
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
  cp -R -- "${SRC}/scripts/lib" "${REPO}/scripts/"
  cp -R -- "${SRC}/checks" "${REPO}/"
  mkdir -p "${REPO}/cli/internal/terms"
  cp -- "${SRC}/cli/go.mod" "${REPO}/cli/"
  cp -- "${SRC}/cli/internal/terms/generic.toml" "${REPO}/cli/internal/terms/"
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

# Public tiers

# Guards a betterleaks bump that changes how keywords, allowlists or the
# extend path match, so that a generic rule stops reporting its shape,
# an allowlisted address hides another on its line, or the inline
# marker hides a hit on another line.
@test "every rule of the file tier reports its shape" {
  local acct=987654321098 guid=3f2b1c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d rule
  # Line 8 ends in the allowlisted address, so that an allowlist applied
  # to the whole line would hide the other one.
  printf '%s\n' "Write to jane@${CORP}.com." \
    "git clone git@gitlab.${CORP}.com:team/repo.git" \
    "arn:aws:iam::${acct}:role/deploy" \
    "/subscriptions/${guid}/resourceGroups/rg" \
    "https://console.cloud.google.com/home?project=${CORP}-prod-42" \
    "https://bucket.example/f?X-Amz-""Signature=0123456789abcdef0123" \
    "jane_${CORP}.com#EXT""#@tenant" \
    "Write to bob@${CORP}.com or a@sub.example.com" \
    "Write to eve@${CORP}.com." "ok cech-terms: allow a reason" > "${REPO}/planted.md"
  g add planted.md
  run_terms --staged
  assert_status 1
  for rule in 1:email 2:email 3:aws-arn 4:azure-subscription 5:gcp-project \
    6:signed-url 7:sso-user 8:email 9:email; do
    assert_output_has "planted.md:${rule%%:*}: ${rule#*:}:"
  done
}

# Guards Betterleaks' default rules dropped through the chain of extends
# (checks/terms/rules.toml, then generic.toml, then the defaults), which
# is at Betterleaks' depth limit: past it, an extend is skipped with no
# error. The token is built in pieces, so this file passes the check.
@test "the default rules run through the extends" {
  plant "key ghp_""aB3dE5fG7hI9jK1mN3pQ5rS7tU9vW1xY3zA5"
  run_terms --staged
  assert_status 1
  assert_output_has "planted.md:1: github-pat:"
}

# Guards BSD sed on macOS building a name rule that misses a spelling or
# a name that ends in punctuation.
@test "names in every spelling" {
  local v
  for v in "Quuxly Cloud" "Quuxly-Cloud" "quuxly_cloud" "QuuxlyCloud" "Zorp++"; do
    plant "Runs on ${v} today."
    run_terms --staged
    # Name the spelling that broke.
    assert_status 1 || {
      echo "spelling: ${v}"
      return 1
    }
    assert_output_has "planted.md:1: name-" || {
      echo "spelling: ${v}"
      return 1
    }
  done
}

# Guards a mise pin bumped without cli/go.mod, or the reverse: the gate
# and `cech check` would then match differently (ADR 24) while both pass.
@test "a pin that differs from cli/go.mod stops the check" {
  sed -e 's|^\(.*github.com/betterleaks/betterleaks\) v.*|\1 v0.0.1|' \
    "${SRC}/cli/go.mod" > "${REPO}/cli/go.mod"
  run_terms --staged
  assert_status 2
  assert_output_has "cli/go.mod requires 0.0.1"
}

# Private tier

# Guards a betterleaks bump after which --ignore-gitleaks-allow no longer
# makes it ignore both of its allow markers.
@test "the engine's markers hide nothing" {
  local m
  for m in betterleaks gitleaks; do
    plant "see ${SECRET} here ${m}"":allow"
    run_terms --staged
    # Name the marker that broke.
    assert_status 1 || {
      echo "marker: ${m}"
      return 1
    }
    assert_output_has "planted.md:1: private-term-2" || {
      echo "marker: ${m}"
      return 1
    }
  done
}

# Guards a betterleaks bump that changes how the private tier reports a hit,
# so that the public hit around the term shows the term or its location.
@test "private term inside a public hit" {
  plant "Write to jane@${SECRET}.com."
  run_terms --staged
  assert_status 1
  assert_output_has "email (location hidden)"
}

# Guards a private list saved on Windows, whose carriage returns and
# trailing spaces would leave each term matching nothing.
@test "private list with CRLF and spaces" {
  printf '# crlf list\r\n%s \r\n' "${SECRET}" > "${BATS_TEST_TMPDIR}/crlf.txt"
  plant "see ${SECRET} here"
  CECH_PRIVATE_TERMS="${BATS_TEST_TMPDIR}/crlf.txt" run_terms --staged
  assert_status 1
  assert_output_has "planted.md:1: private-term-2"
}

# Guards a betterleaks bump that skips a rule it cannot compile instead of
# failing, which would drop the private tier unseen.
@test "a broken private list stops the check" {
  printf '%s\na++\n' "${SECRET}" > "${BATS_TEST_TMPDIR}/broken.txt"
  plant "Write to jane@${SECRET}.com."
  CECH_PRIVATE_TERMS="${BATS_TEST_TMPDIR}/broken.txt" run_terms --staged
  assert_status 2
  assert_output_has "private tier could not run"
}

# Guards the private tier skipped in CI, the one place that requires the
# list.
@test "missing list when required" {
  CECH_PRIVATE_TERMS="${BATS_TEST_TMPDIR}/nonexistent" CECH_REQUIRE_PRIVATE_TERMS=1 \
    run_terms
  assert_status 2
  assert_output_has "private term list not found"
}

# Paths

# Guards a platform, such as macOS, or git path settings that hand a
# non-ASCII path to the scan in a form the term misses.
@test "non-ASCII private term in a path" {
  printf '%s\n' "${WIDE}" > "${BATS_TEST_TMPDIR}/wide.txt"
  printf 'Write to jane@%s.com.\n' "${CORP}" > "${REPO}/notes-${WIDE}.md"
  g add .
  CECH_PRIVATE_TERMS="${BATS_TEST_TMPDIR}/wide.txt" run_terms --staged
  assert_status 1
  assert_output_has "paths:"
  refute_output_has "${WIDE}"
}

# Guards the same for a path that only history holds.
@test "non-ASCII private term in a removed path" {
  printf '%s\n' "${WIDE}" > "${BATS_TEST_TMPDIR}/wide.txt"
  commit_and_drop "notes-${WIDE}.md" clean
  CECH_PRIVATE_TERMS="${BATS_TEST_TMPDIR}/wide.txt" run_terms
  assert_status 1
  assert_output_has "paths:"
  refute_output_has "${WIDE}"
}

# History

# Guards a betterleaks bump after which its git scan reads --log-opts
# differently or no longer scans removed lines.
@test "private term added then removed" {
  commit_and_drop gone.md "note ${SECRET}"
  run_terms
  assert_status 1
  assert_output_has "private-term-2 in commit"
}

# Guards CECH_RANGE ignored: in CI the gate would then check its default
# range instead of the one the workflow sets, and could pass.
@test "CECH_RANGE replaces the default range" {
  commit_and_drop gone.md "note ${SECRET}"
  g update-ref refs/remotes/origin/main HEAD
  CECH_RANGE=HEAD~2..HEAD run_terms
  assert_status 1
  assert_output_has "private-term-2 in commit"
}

# Guards GITHUB_HEAD_REF unread: a pull request's branch exists only in
# CI, so its name would go unchecked.
@test "private term in the CI branch name" {
  GITHUB_HEAD_REF="topic-${SECRET}" run_terms
  assert_status 1
  assert_output_has "ref-names:"
}
