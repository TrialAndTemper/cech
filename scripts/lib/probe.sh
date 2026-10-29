# shellcheck shell=sh
# shellcheck disable=SC2154 # common.sh, sourced first, sets PROG and SCRATCH
# Helpers for the harness probes in scripts/probes/ (ADR 9 item 4). A
# probe sources common.sh, then this file, sets PROG to the name of its
# entry in cli/internal/harnessdata/harnesses.toml, and calls
# probe_start. It exits 0 on the result in the entry's expect, 1 on any
# other, through differs, and 2 when it cannot run, through die.
# POSIX sh (ADR 4), run under dash and macOS sh. The probes run on
# Linux and macOS only: Node and Codex on Windows need SYSTEMROOT and
# USERPROFILE, which harness leaves out with the rest of the
# environment.

# The output of the last harness command, its stdout and stderr in one,
# its stderr alone, and its exit status.
OUT=''
ERR=''
STATUS=0
# The harness's home folder, which probe_start makes empty; a probe that
# runs the harness from scratch more than once points it at a new one.
PROBE_HOME=''

# probe_start <command>: check that the harness command is on PATH, and
# make SCRATCH, with PROBE_HOME in it.
probe_start() {
  command -v "$1" > /dev/null 2>&1 || die "$1 is not on PATH"
  make_scratch
  PROBE_HOME="${SCRATCH}/home"
  : > "${SCRATCH}/all" || die "cannot write in ${SCRATCH}"
}

# harness <command> <args...>: run claude or codex with PROBE_HOME as
# its home and config folder, and nothing from the environment but PATH
# and TMPDIR, so no login, key or setting of the person or runner
# reaches it, and with no stdin, so it cannot wait for input. It sets
# STATUS to the command's exit status, puts its stderr in ERR and its
# output, stdout then stderr, in OUT, and adds that to SCRATCH/all.
harness() {
  case "$1" in
    claude) set -- CLAUDE_CONFIG_DIR="${PROBE_HOME}/.claude" "$@" ;;
    codex) set -- CODEX_HOME="${PROBE_HOME}/.codex" "$@" ;;
    *) die "not a harness: $1" ;;
  esac
  mkdir -p -- "${PROBE_HOME}/.claude" "${PROBE_HOME}/.codex" ||
    die "cannot create the config folders"
  OUT="${SCRATCH}/out"
  ERR="${SCRATCH}/err"
  STATUS=0
  env -i PATH="${PATH}" HOME="${PROBE_HOME}" TMPDIR="${TMPDIR:-/tmp}" "$@" \
    < /dev/null > "${SCRATCH}/stdout" 2> "${ERR}" || STATUS=$?
  cat -- "${SCRATCH}/stdout" "${ERR}" > "${OUT}" || die "cannot keep the output"
  cat -- "${OUT}" >> "${SCRATCH}/all" || die "cannot keep the output"
}

# must <message> <command> <args...>: run the harness command, and fail
# with differs and message if it exits non-zero.
must() {
  local message
  message=$1
  shift
  harness "$@"
  [ "${STATUS}" = 0 ] || differs "${message}"
}

# expect_out <pattern> <message>: fail with differs and message unless a
# line of OUT matches the basic regular expression pattern.
# Exit 2 through die if OUT cannot be searched.
expect_out() {
  local st
  st=0
  grep -q -e "$1" -- "${OUT}" || st=$?
  case "${st}" in
    0) ;;
    1) differs "$2" ;;
    *) die "cannot search the harness output" ;;
  esac
}

# refuse_out <pattern> <message>: fail with differs and message if a
# line of OUT matches the basic regular expression pattern.
# Exit 2 through die if OUT cannot be searched.
refuse_out() {
  local st
  st=0
  grep -q -e "$1" -- "${OUT}" || st=$?
  case "${st}" in
    0) differs "$2" ;;
    1) ;;
    *) die "cannot search the harness output" ;;
  esac
}

# differs <message...>: report a result other than the expected one,
# with OUT, and exit 1.
differs() {
  printf '%s: %s\n' "${PROG}" "$*" >&2
  if [ -s "${OUT}" ]; then
    printf '%s: output of the last command:\n' "${PROG}" >&2
    sed 's/^/  /' -- "${OUT}" >&2
  fi
  exit 1
}

# write_plugin <folder> <name> <word> [<json>]: write a plugin with one
# skill, whose description holds word, and a .claude-plugin/plugin.json
# that adds the members in json, such as "license": "MIT".
write_plugin() {
  mkdir -p -- "$1/.claude-plugin" "$1/skills/$2" || die "cannot create $1"
  printf '{"name": "%s", "version": "0.0.0-dev", "description": "A probe plugin.", "author": {"name": "Acme"}%s}\n' \
    "$2" "${4:+, $4}" > "$1/.claude-plugin/plugin.json" || die "cannot write $1"
  printf -- '---\nname: %s\ndescription: Use when the user asks about %s.\n---\n\nAnswer briefly.\n' \
    "$2" "$3" > "$1/skills/$2/SKILL.md" || die "cannot write $1"
}

# write_marketplace <folder> <name> <plugin...>: write a marketplace
# that lists each plugin in folder/plugins/<plugin>.
write_marketplace() {
  local mkt_dir mkt_name mkt_list p
  mkt_dir=$1
  mkt_name=$2
  shift 2
  mkt_list=''
  for p in "$@"; do
    mkt_list="${mkt_list:+${mkt_list}, }{\"name\": \"${p}\", \"source\": \"./plugins/${p}\"}"
  done
  mkdir -p -- "${mkt_dir}/.claude-plugin" || die "cannot create ${mkt_dir}"
  printf '{"name": "%s", "owner": {"name": "Acme"}, "plugins": [%s]}\n' \
    "${mkt_name}" "${mkt_list}" > "${mkt_dir}/.claude-plugin/marketplace.json" ||
    die "cannot write ${mkt_dir}"
}

# new_project <folder>: create folder as a new git repository and go to
# it.
new_project() {
  mkdir -p -- "$1" || die "cannot create $1"
  git init -q -- "$1" || die "cannot create a git repository in $1"
  cd -- "$1" || die "cannot go to $1"
}

# claude_plugin <plugin@marketplace>: set OUT to the plugin's lines in
# the output of claude plugin list, the last harness command: from its
# name to the next plugin's.
claude_plugin() {
  awk -v id="$1" '
    $NF == id && /❯/ { on = 1; print; next }
    /❯/ { on = 0 }
    on { print }' "${OUT}" > "${SCRATCH}/plugin" || die "cannot read the plugin list"
  OUT="${SCRATCH}/plugin"
}

# no_deprecation [<file>]: fail with differs if file, by default the
# output of every harness command so far, holds a word starting with
# deprecat, in any case.
no_deprecation() {
  OUT=${1:-${SCRATCH}/all}
  refuse_out '[Dd][Ee][Pp][Rr][Ee][Cc][Aa][Tt]' "a harness command printed a deprecation warning"
}

# probe_ok: report the expected result and exit 0.
probe_ok() {
  printf '%s: ok\n' "${PROG}"
  exit 0
}
