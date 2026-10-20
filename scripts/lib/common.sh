# shellcheck shell=sh
# Helpers shared by the scripts under scripts/. A script sources this
# file first, since sourcing resets PROG and TMP, then sets PROG to the
# name that die's messages start with. make_tmp owns the EXIT, INT and
# TERM traps.
# POSIX sh (ADR 10): it must run under dash, macOS sh and Git Bash.

PROG=''
TMP=''

# die <message...>: report that the script cannot run, and exit 2.
die() {
  printf '%s: %s\n' "${PROG}" "$*" >&2
  exit 2
}

# cleanup: remove TMP, if make_tmp created it.
cleanup() {
  if [ -n "${TMP}" ]; then rm -rf -- "${TMP}"; fi
}

# on_signal <signal>: clean up, then stop with the signal, so the caller
# sees how the script ended.
on_signal() {
  cleanup
  trap - "$1"
  kill -s "$1" "$$"
}

# make_tmp: set TMP to a new temporary directory, removed when the
# script exits or is stopped by INT or TERM.
make_tmp() {
  trap cleanup EXIT
  trap 'on_signal INT' INT
  trap 'on_signal TERM' TERM
  TMP=$(mktemp -d) || die "cannot create a temporary directory"
}
